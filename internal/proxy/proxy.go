package proxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateStopping     State = "stopping"
)

type StatusInfo struct {
	State             State
	Uptime            time.Duration
	ActiveConnections int64
	BytesSent         uint64
	BytesReceived     uint64
	LastError         error
}

type Tunnel interface {
	OpenStream(ctx context.Context) (net.Conn, error)
	AcceptStream() (net.Conn, error)
	Close() error
}

type Transport interface {
	Connect(ctx context.Context, target string) (Tunnel, error)
	ProtocolName() string
}

type Proxy struct {
	mu               sync.RWMutex
	state            State
	targetAddr       string
	localAddr        string
	activeConnsCount int64
	startTime        time.Time
	lastError        error
	cancel           context.CancelFunc
	server           *http.Server
	legacyTransport  Transport
	legacyTunnel     Tunnel
}

func NewProxy(transport Transport, localAddr, targetAddr string) *Proxy {
	initTransport(NewDefaultDialer(30 * time.Second))
	return &Proxy{
		state:           StateDisconnected,
		localAddr:       localAddr,
		targetAddr:      targetAddr,
		legacyTransport: transport,
	}
}

func (p *Proxy) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.state == StateConnected || p.state == StateConnecting {
		p.mu.Unlock()
		return errors.New("proxy is already running or connecting")
	}
	p.state = StateConnecting
	p.lastError = nil
	p.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)

	if p.legacyTransport != nil {
		tunnel, err := p.legacyTransport.Connect(ctx, p.targetAddr)
		if err != nil {
			p.mu.Lock()
			p.state = StateDisconnected
			p.lastError = err
			p.mu.Unlock()
			cancel()
			return err
		}
		p.legacyTunnel = tunnel
	}

	listener, err := net.Listen("tcp", p.localAddr)
	if err != nil {
		p.mu.Lock()
		p.state = StateDisconnected
		p.lastError = err
		p.mu.Unlock()
		if p.legacyTunnel != nil {
			_ = p.legacyTunnel.Close()
		}
		cancel()
		return err
	}

	actualAddr := listener.Addr().String()

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p.handleRequest(w, r)
		}),
	}

	p.mu.Lock()
	p.state = StateConnected
	p.startTime = time.Now()
	p.cancel = cancel
	p.server = server
	p.localAddr = actualAddr
	p.mu.Unlock()

	// Clear session metrics on start
	atomic.StoreInt64(&GlobalMetrics.Requests, 0)
	atomic.StoreInt64(&GlobalMetrics.SuccessfulRequests, 0)
	atomic.StoreInt64(&GlobalMetrics.FailedRequests, 0)
	atomic.StoreInt64(&GlobalMetrics.TotalTTFB, 0)
	atomic.StoreInt64(&GlobalMetrics.TotalDuration, 0)
	atomic.StoreUint64(&GlobalMetrics.BytesUploaded, 0)
	atomic.StoreUint64(&GlobalMetrics.BytesDownloaded, 0)

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.mu.Lock()
			p.lastError = err
			p.mu.Unlock()
		}
	}()

	go func() {
		<-ctx.Done()
		_ = server.Close()
		_ = listener.Close()
	}()

	return nil
}

func (p *Proxy) Stop() error {
	p.mu.Lock()
	if p.state == StateDisconnected {
		p.mu.Unlock()
		return nil
	}
	p.state = StateStopping
	cancel := p.cancel
	server := p.server
	tunnel := p.legacyTunnel
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	var err error
	if server != nil {
		err = server.Close()
	}

	if tunnel != nil {
		if closeErr := tunnel.Close(); closeErr != nil {
			err = closeErr
		}
	}

	if EdgeTransport != nil {
		EdgeTransport.CloseIdleConnections()
	}

	p.mu.Lock()
	p.state = StateDisconnected
	p.server = nil
	p.legacyTunnel = nil
	p.startTime = time.Time{}
	p.mu.Unlock()

	return err
}

func (p *Proxy) Status() (StatusInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var uptime time.Duration
	if !p.startTime.IsZero() {
		uptime = time.Since(p.startTime)
	}

	return StatusInfo{
		State:             p.state,
		Uptime:            uptime,
		ActiveConnections: atomic.LoadInt64(&p.activeConnsCount),
		BytesSent:         atomic.LoadUint64(&GlobalMetrics.BytesUploaded),
		BytesReceived:     atomic.LoadUint64(&GlobalMetrics.BytesDownloaded),
		LastError:         p.lastError,
	}, nil
}

func (p *Proxy) SetTargetAddr(addr string) {
	p.mu.Lock()
	p.targetAddr = addr
	p.mu.Unlock()
}

func (p *Proxy) GetLocalAddr() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.localAddr
}

type statusWriter struct {
	http.ResponseWriter
	status  int
	written int64
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("underlying ResponseWriter does not implement Hijacker")
}

func (p *Proxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	IncrementRequests()
	atomic.AddInt64(&p.activeConnsCount, 1)
	defer atomic.AddInt64(&p.activeConnsCount, -1)

	tracker := NewTracker()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

	defer func() {
		tracker.RecordLastByte()

		if sw.status >= 200 && sw.status < 400 {
			IncrementSuccessfulRequests()
		} else {
			IncrementFailedRequests()
		}
	}()

	if r.Method == http.MethodConnect {
		p.handleConnect(sw, r, tracker)
		return
	}

	p.handleHTTP(sw, r, tracker)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request, tracker *Tracker) {
	dest := r.Host
	ctx := r.Context()

	tracker.RecordDialStart()
	destConn, err := CustomDialer.DialContext(ctx, "tcp", dest)
	if err != nil {
		tracker.RecordDialEnd(false)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	tracker.RecordDialEnd(true)
	defer destConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	tracker.RecordFirstByte()

	errChan := make(chan error, 2)
	go func() {
		_, err := streamCopy(destConn, clientConn, true) // Upload
		errChan <- err
	}()
	go func() {
		_, err := streamCopy(clientConn, destConn, false) // Download
		errChan <- err
	}()

	select {
	case <-ctx.Done():
	case <-errChan:
	}
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request, tracker *Tracker) {
	p.mu.RLock()
	target := p.targetAddr
	p.mu.RUnlock()

	// In Forward Proxy Mode:
	if (target == "" || target == "/") && r.URL.Host == "" {
		http.Error(w, "Proxy destination host missing", http.StatusBadRequest)
		return
	}

	tracedCtx := WithMetricsTrace(r.Context(), tracker)
	reqURL := r.URL.String()

	// Perform Gateway Mode rewrite if target is set and is not "/"
	if target != "" && target != "/" {
		targetURL, err := url.Parse(target)
		if err != nil || targetURL.Host == "" {
			targetURL, err = url.Parse("http://" + target)
		}
		if err == nil && targetURL.Host != "" {
			targetReqURL := *r.URL
			targetReqURL.Scheme = targetURL.Scheme
			targetReqURL.Host = targetURL.Host
			reqURL = targetReqURL.String()
		}
	}

	outReq, err := http.NewRequestWithContext(tracedCtx, r.Method, reqURL, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	outReq.Header = make(http.Header)
	for k, vv := range r.Header {
		for _, v := range vv {
			outReq.Header.Add(k, v)
		}
	}

	if target != "" && target != "/" {
		if targetURL, err := url.Parse(target); err == nil && targetURL.Host != "" {
			outReq.Host = targetURL.Host
		} else if targetURL, err := url.Parse("http://" + target); err == nil && targetURL.Host != "" {
			outReq.Host = targetURL.Host
		}
	}

	// Make the request using EdgeTransport
	resp, err := EdgeTransport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream the response body back
	_, _ = streamCopy(w, resp.Body, false) // Download
}

var (
	defaultProxy *Proxy
	defaultMu    sync.Mutex
)

func InitDefaultProxy(transport Transport, localAddr, targetAddr string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultProxy = NewProxy(transport, localAddr, targetAddr)
}

func Start(ctx context.Context) error {
	defaultMu.Lock()
	p := defaultProxy
	defaultMu.Unlock()
	if p == nil {
		return errors.New("proxy not initialized")
	}
	return p.Start(ctx)
}

func Stop() error {
	defaultMu.Lock()
	p := defaultProxy
	defaultMu.Unlock()
	if p == nil {
		return nil
	}
	return p.Stop()
}

func Status() (StatusInfo, error) {
	defaultMu.Lock()
	p := defaultProxy
	defaultMu.Unlock()
	if p == nil {
		return StatusInfo{State: StateDisconnected}, nil
	}
	return p.Status()
}

func SetTargetAddr(addr string) {
	defaultMu.Lock()
	p := defaultProxy
	defaultMu.Unlock()
	if p != nil {
		p.SetTargetAddr(addr)
	}
}

func GetLocalAddr() string {
	defaultMu.Lock()
	p := defaultProxy
	defaultMu.Unlock()
	if p == nil {
		return "127.0.0.1:8080"
	}
	return p.GetLocalAddr()
}

type StubTransport struct{}

func (s *StubTransport) Connect(ctx context.Context, target string) (Tunnel, error) {
	return &StubTunnel{}, nil
}

func (s *StubTransport) ProtocolName() string {
	return "mock-quic"
}

type StubTunnel struct{}

func (s *StubTunnel) OpenStream(ctx context.Context) (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (s *StubTunnel) AcceptStream() (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (s *StubTunnel) Close() error {
	return nil
}

var proxyLogger *log.Logger

func init() {
	var writer io.Writer = os.Stderr
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".tokenly")
		if err := os.MkdirAll(dir, 0755); err == nil {
			if file, err := os.OpenFile(filepath.Join(dir, "proxy.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				writer = file
			}
		}
	}
	proxyLogger = log.New(writer, "", log.LstdFlags)

	InitDefaultProxy(&StubTransport{}, "127.0.0.1:8081", "http://127.0.0.1:8080")
}
