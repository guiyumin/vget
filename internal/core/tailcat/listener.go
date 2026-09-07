package tailcat

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/tailscale/tailcat"
	"tailscale.com/types/logger"
	"tailscale.com/wgengine/filter"
)

// Server is a running tailcat server exposing Port as a net.Listener.
type Server struct {
	srv *tailcat.Server
	ln  *listener
	id  *Identity
}

// ListenOptions configures Listen.
type ListenOptions struct {
	Identity *Identity

	// Allow lists client public keys ("nodekey:…") permitted to connect.
	// Empty allows everyone who knows the address.
	Allow []string

	// Logf receives tailcat's own diagnostic logging. nil discards it.
	Logf func(format string, args ...any)
}

// Listen connects to the identity's relay and starts accepting clients.
// Connections to Port inside the tunnel come out of Server.Listener.
func Listen(opts ListenOptions) (*Server, error) {
	id := opts.Identity
	if id == nil || id.pk == nil {
		return nil, errors.New("tailcat: missing identity")
	}
	reg := id.Region()
	if reg == nil {
		return nil, errors.New("tailcat: identity has no relay region")
	}
	allowed, err := ParseAllowedKeys(opts.Allow)
	if err != nil {
		return nil, err
	}
	logf := logger.Discard
	if opts.Logf != nil {
		logf = logger.Logf(opts.Logf)
	}

	ln := &listener{conns: make(chan net.Conn), done: make(chan struct{})}
	srv := &tailcat.Server{
		Key:            id.pk.Private,
		PresharedKey:   id.pk.Public.PresharedKey,
		Region:         reg,
		AllowedClients: allowed,
		ServedTCPPorts: []filter.PortRange{{First: Port, Last: Port}},
		Logf:           logf,
		OnTCP: func(port uint16) func(net.Conn) {
			if port != Port {
				return nil // RST
			}
			return ln.deliver
		},
	}
	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("tailcat: %w", err)
	}
	ln.addr = tcAddr(srv.TailcatAddr())
	return &Server{srv: srv, ln: ln, id: id}, nil
}

// Listener returns the listener for connections arriving through the tunnel.
func (s *Server) Listener() net.Listener { return s.ln }

// Addr returns the server's tailcat address (a bearer secret).
func (s *Server) Addr() string { return string(s.srv.TailcatAddr()) }

// Relay describes the relay the server is registered with.
func (s *Server) Relay() string { return s.id.Relay() }

// RelayLatency measures the round-trip time to the server's relay.
func (s *Server) RelayLatency(ctx context.Context) (time.Duration, error) {
	return RegionLatency(ctx, s.id.Region())
}

// Close stops accepting, waits (bounded by ctx) for in-flight connections to
// drain so their final packets reach the peers, then tears the tunnel down.
// The whole TCP stack lives in this process: exiting right after closing a
// connection could lose its FIN.
func (s *Server) Close(ctx context.Context) error {
	s.ln.Close()
	_ = s.srv.DrainTCP(ctx)
	return s.srv.Close()
}

// listener turns tailcat's per-connection callback into net.Listener.Accept.
type listener struct {
	conns     chan net.Conn
	done      chan struct{}
	closeOnce sync.Once
	addr      net.Addr
}

func (l *listener) deliver(c net.Conn) {
	select {
	case l.conns <- c:
	case <-l.done:
		c.Close()
	}
}

func (l *listener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *listener) Close() error {
	l.closeOnce.Do(func() { close(l.done) })
	return nil
}

func (l *listener) Addr() net.Addr { return l.addr }

type tcAddr string

func (tcAddr) Network() string  { return "tailcat" }
func (a tcAddr) String() string { return string(a) }
