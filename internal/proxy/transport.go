package proxy

import (
	"context"
	"net"
	"net/http"
	"time"
)

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type DefaultDialer struct {
	dialer net.Dialer
}

func NewDefaultDialer(keepAlive time.Duration) *DefaultDialer {
	return &DefaultDialer{
		dialer: net.Dialer{
			KeepAlive: keepAlive,
		},
	}
}

func (d *DefaultDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, address)
}

var (
	EdgeTransport *http.Transport
	CustomDialer  Dialer
)

func initTransport(dialer Dialer) {
	CustomDialer = dialer
	EdgeTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return CustomDialer.DialContext(ctx, network, address)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DisableCompression:    true,
	}
}
