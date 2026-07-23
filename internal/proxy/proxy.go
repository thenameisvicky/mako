package proxy

import (
	"context"
	"errors"
	"net"
	"sync"
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
	mu            sync.RWMutex
	state         State
	transport     Transport
	tunnel        Tunnel
	targetAddr    string
	localAddr     string
	activeConns   sync.WaitGroup
	startTime     time.Time
	lastError     error
	bytesSent     uint64
	bytesReceived uint64
	cancel        context.CancelFunc
}

func NewProxy(transport Transport, localAddr, targetAddr string) *Proxy {
	return &Proxy{
		state:      StateDisconnected,
		transport:  transport,
		localAddr:  localAddr,
		targetAddr: targetAddr,
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

	tunnel, err := p.transport.Connect(ctx, p.targetAddr)
	if err != nil {
		p.mu.Lock()
		p.state = StateDisconnected
		p.lastError = err
		p.mu.Unlock()
		cancel()
		return err
	}

	p.mu.Lock()
	p.state = StateConnected
	p.tunnel = tunnel
	p.startTime = time.Now()
	p.cancel = cancel
	p.mu.Unlock()

	go p.routingLoop(ctx)

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
	tunnel := p.tunnel
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	var err error
	if tunnel != nil {
		err = tunnel.Close()
	}

	p.mu.Lock()
	p.state = StateDisconnected
	p.tunnel = nil
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
		State:         p.state,
		Uptime:        uptime,
		LastError:     p.lastError,
		BytesSent:     p.bytesSent,
		BytesReceived: p.bytesReceived,
	}, nil
}

func (p *Proxy) routingLoop(ctx context.Context) {
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

func init() {
	InitDefaultProxy(&StubTransport{}, "127.0.0.1:8080", "edge.tokenly.network:443")
}
