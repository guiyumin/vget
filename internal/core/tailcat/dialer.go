package tailcat

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tailscale/tailcat"
	"tailscale.com/types/logger"
)

// tunnel is a lazily started client tunnel to one server address, shared by
// every connection this process opens to it.
type tunnel struct {
	c      *tailcat.Client
	dialed atomic.Bool
}

var (
	tunnelsMu  sync.Mutex
	tunnels    = map[string]*tunnel{}
	transports []*http.Transport
)

// tunnelFor returns the process-wide tunnel for addr, creating it on first
// use. The relay handshake happens on the first dial, not here.
func tunnelFor(addr string) (*tunnel, error) {
	if err := Validate(addr); err != nil {
		return nil, err
	}
	tunnelsMu.Lock()
	defer tunnelsMu.Unlock()
	if t, ok := tunnels[addr]; ok {
		return t, nil
	}
	t := &tunnel{c: &tailcat.Client{
		Server:       tailcat.Addr(addr),
		Key:          ClientKey(),
		Logf:         logger.Discard,
		DERPMapCache: diskDERPMapCache{},
	}}
	tunnels[addr] = t
	return t, nil
}

// DialContext returns a dialer opening TCP connections to the vget server
// behind addr through the tunnel. The network and address arguments are
// ignored: they only ever name the placeholder BaseURL host.
func DialContext(addr string) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		t, err := tunnelFor(addr)
		if err != nil {
			return nil, err
		}
		c, err := t.c.DialTCPPort(ctx, Port)
		if err != nil {
			return nil, err
		}
		t.dialed.Store(true)
		return c, nil
	}
}

// Transport returns an http.Transport that dials through the tunnel to addr,
// tuned like the download engine's transport (many parallel streams, no
// compression of already-compressed media).
func Transport(addr string) *http.Transport {
	tr := &http.Transport{
		DialContext:         DialContext(addr),
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     120 * time.Second,
		DisableCompression:  true,
		WriteBufferSize:     128 * 1024,
		ReadBufferSize:      128 * 1024,
	}
	tunnelsMu.Lock()
	transports = append(transports, tr)
	tunnelsMu.Unlock()
	return tr
}

// HTTPClient returns an http.Client that dials through the tunnel to addr.
func HTTPClient(addr string) *http.Client {
	return &http.Client{Transport: Transport(addr)}
}

// Shutdown closes every tunnel this process opened. Idle HTTP connections are
// closed first and the tunnels then drained (bounded by ctx) so the servers
// see clean closes instead of retransmitting FINs to a vanished peer.
func Shutdown(ctx context.Context) {
	tunnelsMu.Lock()
	ts, trs := tunnels, transports
	tunnels, transports = map[string]*tunnel{}, nil
	tunnelsMu.Unlock()
	for _, tr := range trs {
		tr.CloseIdleConnections()
	}
	for _, t := range ts {
		if t.dialed.Load() {
			_ = t.c.DrainTCP(ctx)
		}
		t.c.Close()
	}
}
