// Package proxy provides an HTTP/HTTPS proxy with policy enforcement.
package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/google/martian/v3"
	"github.com/seslattery/veil/internal/policy"
)

// Proxy is an HTTP/HTTPS proxy that enforces allowlist policy.
type Proxy struct {
	policy   *policy.Policy
	listener net.Listener
	proxy    *martian.Proxy
	logger   *slog.Logger
}

// New creates a new Proxy.
func New(pol *policy.Policy, logger *slog.Logger) (*Proxy, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	p := martian.NewProxy()

	return &Proxy{
		policy:   pol,
		listener: listener,
		proxy:    p,
		logger:   logger,
	}, nil
}

// Port returns the proxy's listening port.
func (p *Proxy) Port() int {
	return p.listener.Addr().(*net.TCPAddr).Port
}

// Addr returns the proxy's address (e.g., "127.0.0.1:12345").
func (p *Proxy) Addr() string {
	return p.listener.Addr().String()
}

// Start starts the proxy server. Blocks until context is cancelled.
func (p *Proxy) Start(ctx context.Context) error {
	// Set up CONNECT handler for policy enforcement
	p.proxy.SetDial(p.dialWithPolicy)

	// Handle shutdown
	go func() {
		<-ctx.Done()
		p.listener.Close()
	}()

	p.logger.Info("proxy started", "addr", p.Addr())
	return p.proxy.Serve(p.listener)
}

// dialWithPolicy is a custom dialer that enforces the allowlist policy.
func (p *Proxy) dialWithPolicy(network, addr string) (net.Conn, error) {
	if !p.policy.Evaluate(addr) {
		p.logger.Warn("blocked by policy", "host", addr)
		return nil, fmt.Errorf("blocked by policy: %s", addr)
	}

	p.logger.Debug("allowed", "host", addr)
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return dialer.Dial(network, addr)
}

// Close shuts down the proxy.
func (p *Proxy) Close() error {
	return p.listener.Close()
}
