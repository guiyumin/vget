// Package tailcat adapts the tailcat library (Tailscale's data plane without
// its control plane: WireGuard, NAT traversal, DERP relays) to the plain
// net.Listener / net.Conn / http.RoundTripper shapes the rest of vget speaks.
//
// It is the only package in vget that imports tailscale.com or
// github.com/tailscale/tailcat, so an upstream change touches one place.
package tailcat

import (
	"fmt"
	"strings"

	"github.com/tailscale/tailcat"
	"tailscale.com/tailcfg"
)

// BaseURL is the placeholder origin used for HTTP requests over a tailcat
// tunnel. The host is never resolved: every dial goes through the tunnel.
const BaseURL = "http://vget.tailcat"

// Port is the TCP port the vget WebDAV server listens on inside the tunnel.
const Port = 80

// IsAddr reports whether s is a valid tailcat address ("tc" + base64url).
func IsAddr(s string) bool {
	if !strings.HasPrefix(s, "tc") || len(s) < 40 {
		return false
	}
	if strings.ContainsAny(s, "/:.@ \t\r\n") {
		return false
	}
	_, err := tailcat.ParseAddr(tailcat.Addr(s))
	return err == nil
}

// Validate returns an error explaining why s is not a usable tailcat address.
func Validate(s string) error {
	if !strings.HasPrefix(s, "tc") {
		return fmt.Errorf("%q is not a tailcat address (they start with \"tc\")", s)
	}
	ci, err := tailcat.ParseAddr(tailcat.Addr(s))
	if err != nil {
		return fmt.Errorf("invalid tailcat address: %v (if it came from a newer vget, run 'vget update')", err)
	}
	if ci.ServerDiscoPublic.IsZero() {
		return fmt.Errorf("outdated tailcat address; the other side needs to update vget and share a new one")
	}
	return nil
}

// Mask redacts a tailcat address for display. The address is a bearer
// secret, so it is only shown in full when explicitly asked for.
func Mask(addr string) string {
	if len(addr) <= 16 {
		return addr
	}
	return addr[:10] + "…" + addr[len(addr)-4:]
}

// RelayOf describes the relay embedded in a tailcat address, e.g.
// "tc302a.ipn.dev" or "1.2.3.4:8443". It is empty if the address only
// references a region of the public DERP map.
func RelayOf(addr string) string {
	ci, err := tailcat.ParseAddr(tailcat.Addr(addr))
	if err != nil {
		return ""
	}
	if len(ci.Region) > 0 && len(ci.Region[0].Nodes) > 0 {
		return nodeHostPort(ci.Region[0].Nodes[0])
	}
	if ci.RegionID != 0 {
		return fmt.Sprintf("public DERP region %d", ci.RegionID)
	}
	return ""
}

func nodeHostPort(n *tailcfg.DERPNode) string {
	host := n.HostName
	if host == "" {
		host = n.IPv4
	}
	if host == "" {
		host = n.IPv6
	}
	if n.DERPPort != 0 && n.DERPPort != 443 {
		return fmt.Sprintf("%s:%d", host, n.DERPPort)
	}
	return host
}

// describeRegion renders a relay region for humans: "302 San Francisco" for
// a public region, "self-hosted 1.2.3.4:8443" for the user's own relay.
func describeRegion(r *tailcfg.DERPRegion) string {
	if r == nil {
		return ""
	}
	var node string
	if len(r.Nodes) > 0 {
		node = nodeHostPort(r.Nodes[0])
	}
	if r.RegionID == ownRegionID {
		return "self-hosted " + node
	}
	name := r.RegionName
	if name == "" {
		name = node
	}
	return fmt.Sprintf("%d %s", r.RegionID, name)
}
