package proxy

import (
	"context"
	"errors"
	"net"
	"testing"
)

type mockTransport struct {
	connectErr error
	tunnel     *mockTunnel
}

func (m *mockTransport) Connect(ctx context.Context, target string) (Tunnel, error) {
	if m.connectErr != nil {
		return nil, m.connectErr
	}
	return m.tunnel, nil
}

func (m *mockTransport) ProtocolName() string {
	return "mock-transport"
}

type mockTunnel struct {
	closed bool
}

func (m *mockTunnel) OpenStream(ctx context.Context) (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTunnel) AcceptStream() (net.Conn, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTunnel) Close() error {
	m.closed = true
	return nil
}

func TestProxy_Lifecycle(t *testing.T) {
	tunnel := &mockTunnel{}
	transport := &mockTransport{tunnel: tunnel}
	p := NewProxy(transport, "127.0.0.1:0", "edge.tokenly.network:443")

	// Verify initial state
	status, err := p.Status()
	if err != nil {
		t.Fatalf("unexpected error getting status: %v", err)
	}
	if status.State != StateDisconnected {
		t.Errorf("expected disconnected, got %s", status.State)
	}

	// Start proxy
	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("unexpected error starting proxy: %v", err)
	}

	status, err = p.Status()
	if err != nil {
		t.Fatalf("unexpected error getting status: %v", err)
	}
	if status.State != StateConnected {
		t.Errorf("expected connected state, got %s", status.State)
	}
	if status.Uptime <= 0 {
		t.Errorf("expected uptime > 0, got %v", status.Uptime)
	}

	// Try starting again (should fail)
	if err := p.Start(ctx); err == nil {
		t.Error("expected error starting already running proxy")
	}

	// Stop proxy
	if err := p.Stop(); err != nil {
		t.Fatalf("unexpected error stopping proxy: %v", err)
	}

	status, err = p.Status()
	if err != nil {
		t.Fatalf("unexpected error getting status: %v", err)
	}
	if status.State != StateDisconnected {
		t.Errorf("expected state to return to disconnected, got %s", status.State)
	}

	if !tunnel.closed {
		t.Error("expected tunnel to be closed on proxy stop")
	}
}

func TestProxy_ConnectionError(t *testing.T) {
	expectedErr := errors.New("connection failed")
	transport := &mockTransport{connectErr: expectedErr}
	p := NewProxy(transport, "127.0.0.1:0", "edge.tokenly.network:443")

	ctx := context.Background()
	err := p.Start(ctx)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected connection error %v, got %v", expectedErr, err)
	}

	status, err := p.Status()
	if err != nil {
		t.Fatalf("unexpected status error: %v", err)
	}
	if status.State != StateDisconnected {
		t.Errorf("expected disconnected state after failure, got %s", status.State)
	}
	if !errors.Is(status.LastError, expectedErr) {
		t.Errorf("expected LastError to be set to %v, got %v", expectedErr, status.LastError)
	}
}

func TestDefaultProxyFunctions(t *testing.T) {
	tunnel := &mockTunnel{}
	transport := &mockTransport{tunnel: tunnel}

	InitDefaultProxy(transport, "127.0.0.1:0", "edge:443")

	status, err := Status()
	if err != nil {
		t.Fatalf("unexpected default status error: %v", err)
	}
	if status.State != StateDisconnected {
		t.Errorf("expected disconnected, got %v", status.State)
	}

	if err := Start(context.Background()); err != nil {
		t.Fatalf("failed to start default proxy: %v", err)
	}

	status, err = Status()
	if err != nil {
		t.Fatalf("unexpected default status error: %v", err)
	}
	if status.State != StateConnected {
		t.Errorf("expected connected, got %v", status.State)
	}

	if err := Stop(); err != nil {
		t.Fatalf("failed to stop default proxy: %v", err)
	}

	status, err = Status()
	if err != nil {
		t.Fatalf("unexpected default status error: %v", err)
	}
	if status.State != StateDisconnected {
		t.Errorf("expected disconnected, got %v", status.State)
	}
}
