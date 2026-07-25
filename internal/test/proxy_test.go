package test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenameisvicky/tokenly-cli/internal/proxy"
)

type mockTransport struct {
	connectErr error
	tunnel     *mockTunnel
}

func (m *mockTransport) Connect(ctx context.Context, target string) (proxy.Tunnel, error) {
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
	p := proxy.NewProxy(transport, "127.0.0.1:0", "edge.tokenly.network:443")

	// Verify initial state
	status, err := p.Status()
	assert.NoError(t, err)
	assert.Equal(t, proxy.StateDisconnected, status.State)

	// Start proxy
	ctx := context.Background()
	err = p.Start(ctx)
	assert.NoError(t, err)

	status, err = p.Status()
	assert.NoError(t, err)
	assert.Equal(t, proxy.StateConnected, status.State)
	assert.True(t, status.Uptime > 0)

	// Try starting again (should fail)
	err = p.Start(ctx)
	assert.Error(t, err)

	// Stop proxy
	err = p.Stop()
	assert.NoError(t, err)

	status, err = p.Status()
	assert.NoError(t, err)
	assert.Equal(t, proxy.StateDisconnected, status.State)
	assert.True(t, tunnel.closed)
}

func TestProxy_ConnectionError(t *testing.T) {
	expectedErr := errors.New("connection failed")
	transport := &mockTransport{connectErr: expectedErr}
	p := proxy.NewProxy(transport, "127.0.0.1:0", "edge.tokenly.network:443")

	ctx := context.Background()
	err := p.Start(ctx)
	assert.ErrorIs(t, err, expectedErr)

	status, err := p.Status()
	assert.NoError(t, err)
	assert.Equal(t, proxy.StateDisconnected, status.State)
	assert.ErrorIs(t, status.LastError, expectedErr)
}

func TestDefaultProxyFunctions(t *testing.T) {
	tunnel := &mockTunnel{}
	transport := &mockTransport{tunnel: tunnel}

	proxy.InitDefaultProxy(transport, "127.0.0.1:0", "edge:443")

	status, err := proxy.Status()
	assert.NoError(t, err)
	assert.Equal(t, proxy.StateDisconnected, status.State)

	err = proxy.Start(context.Background())
	assert.NoError(t, err)

	status, err = proxy.Status()
	assert.NoError(t, err)
	assert.Equal(t, proxy.StateConnected, status.State)

	err = proxy.Stop()
	assert.NoError(t, err)

	status, err = proxy.Status()
	assert.NoError(t, err)
	assert.Equal(t, proxy.StateDisconnected, status.State)
}
