package tailcat

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/tailscale/tailcat"
	"tailscale.com/net/netcheck"
	"tailscale.com/net/netmon"
	"tailscale.com/net/netns"
	"tailscale.com/tailcfg"
	"tailscale.com/types/logger"
)

// ownRegionID is the DERP region ID given to a self-hosted relay. It only has
// to be non-zero; vget's DERP maps never contain more than one region.
const ownRegionID = 900

// DERPOption describes a self-hosted DERP relay (--derp / serve.derp).
type DERPOption struct {
	Host       string // hostname or bare IP
	Port       int    // DERP TLS port; 0 means 443
	STUNPort   int    // STUN UDP port; 0 means 3478
	CertSHA256 string // hex SHA-256 of the relay's certificate, for self-signed certs
}

// ParseHostPort splits the "host[:port]" form accepted by --derp and
// "vget derp test". A missing port is returned as 0.
func ParseHostPort(s string) (host string, port int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, errors.New("empty relay address")
	}
	if h, p, err := net.SplitHostPort(s); err == nil {
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 || n > 65535 {
			return "", 0, fmt.Errorf("invalid port in %q", s)
		}
		return h, n, nil
	}
	return strings.Trim(s, "[]"), 0, nil
}

// NormalizeFingerprint canonicalizes a certificate fingerprint to 64 lowercase
// hex characters, accepting "sha256:", "sha256-raw:" prefixes and colons.
func NormalizeFingerprint(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "sha256-raw:")
	s = strings.TrimPrefix(s, "sha256:")
	return strings.NewReplacer(":", "", " ", "").Replace(s)
}

// Region builds the single-node DERP region describing the relay, in the
// form that gets embedded in the server identity and its tailcat address.
func (o DERPOption) Region() (*tailcfg.DERPRegion, error) {
	host := strings.TrimSpace(o.Host)
	if host == "" {
		return nil, errors.New("relay host is empty")
	}
	if o.Port < 0 || o.Port > 65535 || o.STUNPort < -1 || o.STUNPort > 65535 {
		return nil, errors.New("relay port out of range")
	}
	node := &tailcfg.DERPNode{
		Name:     "own",
		RegionID: ownRegionID,
		HostName: host,
		DERPPort: o.Port,
		STUNPort: o.STUNPort,
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.Is4() {
			node.IPv4 = ip.String()
		} else {
			node.IPv6 = ip.String()
		}
	}
	if fp := NormalizeFingerprint(o.CertSHA256); fp != "" {
		if len(fp) != 64 {
			return nil, fmt.Errorf("certificate fingerprint must be 64 hex characters, got %d", len(fp))
		}
		if _, err := strconv.ParseUint(fp[:16], 16, 64); err != nil {
			return nil, errors.New("certificate fingerprint must be hexadecimal")
		}
		node.CertName = "sha256-raw:" + fp
	}
	return &tailcfg.DERPRegion{
		RegionID:   ownRegionID,
		RegionCode: "own",
		RegionName: "self-hosted",
		Nodes:      []*tailcfg.DERPNode{node},
	}, nil
}

// NearestPublicRegion fetches the public DERP map (cached on disk) and picks
// the lowest-latency region. It runs once, when a server identity is created,
// and the chosen region is then embedded in the identity so later startups
// need no network access to it.
func NearestPublicRegion(ctx context.Context) (*tailcfg.DERPRegion, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	dm, err := tailcat.FetchDERPMap(ctx, tailcat.ExpandForServer, diskDERPMapCache{})
	if err != nil {
		return nil, fmt.Errorf("fetching the public relay list: %w", err)
	}
	if len(dm.Regions) == 0 {
		return nil, errors.New("the public relay list is empty")
	}
	id, err := tailcat.PickBestRegion(ctx, dm)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		// No region answered the latency probes (UDP blocked?). Fall back to
		// the lowest-numbered region rather than failing outright.
		for rid := range dm.Regions {
			if id == 0 || rid < id {
				id = rid
			}
		}
	}
	return trimRegion(dm.Regions[id]), nil
}

// trimRegion copies r keeping only what a tailcat address needs: at most two
// relay nodes, without geo or legacy fields.
func trimRegion(r *tailcfg.DERPRegion) *tailcfg.DERPRegion {
	out := &tailcfg.DERPRegion{
		RegionID:   r.RegionID,
		RegionCode: r.RegionCode,
		RegionName: r.RegionName,
	}
	for _, n := range r.Nodes {
		if n.STUNOnly {
			continue
		}
		nn := *n
		nn.CanPort80 = false
		nn.STUNTestIP = ""
		out.Nodes = append(out.Nodes, &nn)
		if len(out.Nodes) == 2 {
			break
		}
	}
	return out
}

// sameRelay reports whether the identity's stored region already describes
// the relay in want (same first node), so the address need not change.
func sameRelay(have []*tailcfg.DERPRegion, want *tailcfg.DERPRegion) bool {
	if len(have) == 0 || want == nil || len(have[0].Nodes) == 0 || len(want.Nodes) == 0 {
		return false
	}
	a, b := have[0].Nodes[0], want.Nodes[0]
	return have[0].RegionID == want.RegionID &&
		a.HostName == b.HostName && a.DERPPort == b.DERPPort && a.STUNPort == b.STUNPort &&
		a.CertName == b.CertName && a.IPv4 == b.IPv4 && a.IPv6 == b.IPv6
}

// RegionLatency measures the round-trip time to a relay region the way
// tailcat picks regions: a STUN probe over UDP, falling back to HTTPS.
func RegionLatency(ctx context.Context, r *tailcfg.DERPRegion) (time.Duration, error) {
	if r == nil {
		return 0, errors.New("no relay region")
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	// tailcat has no TUN device for netns to route around; leaving netns on
	// makes loopback/UDP probes bind to the default interface and time out.
	netns.SetEnabled(false)
	nc := &netcheck.Client{NetMon: netmon.NewStatic(), Logf: logger.Discard}
	if err := nc.Standalone(ctx, ":0"); err != nil {
		return 0, err
	}
	dm := &tailcfg.DERPMap{Regions: map[tailcfg.DERPRegionID]*tailcfg.DERPRegion{r.RegionID: r}}
	rep, err := nc.GetReport(ctx, dm, &netcheck.GetReportOpts{})
	if err != nil {
		return 0, err
	}
	d, ok := rep.RegionLatency[r.RegionID]
	if !ok {
		return 0, errors.New("no reply from relay")
	}
	return d, nil
}
