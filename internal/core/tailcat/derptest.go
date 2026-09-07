package tailcat

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tailscale.com/derp/derpconst"
	"tailscale.com/net/stun"
)

// RelayTestResult is what "vget derp test" reports about a DERP relay.
type RelayTestResult struct {
	Host     string
	Port     int
	STUNPort int

	Reachable bool          // the relay answered a DERP probe over TLS
	RTT       time.Duration // one HTTP round trip on the established connection
	Err       error

	CertSHA256  string // hex SHA-256 of the relay's certificate
	CertTrusted bool   // the certificate verifies against the system roots
	CertSubject string

	STUNOK  bool
	STUNRTT time.Duration
	STUNErr error
}

// TestRelay probes a DERP relay: a TLS connection and DERP probe request on
// port (0 means 443), and a STUN binding request on stunPort (0 means 3478).
// It also captures the certificate fingerprint, which a self-signed relay
// needs passed to "vget serve --derp-cert-sha256".
func TestRelay(ctx context.Context, host string, port, stunPort int) *RelayTestResult {
	if port == 0 {
		port = 443
	}
	if stunPort == 0 {
		stunPort = 3478
	}
	r := &RelayTestResult{Host: host, Port: port, STUNPort: stunPort}
	r.testDERP(ctx)
	r.testSTUN(ctx)
	return r
}

func (r *RelayTestResult) testDERP(ctx context.Context) {
	var leaf *x509.Certificate
	var verifyErr error
	isIP := net.ParseIP(r.Host) != nil
	cfg := &tls.Config{
		InsecureSkipVerify: true, // verification is done below so the fingerprint can be reported either way
		VerifyConnection: func(cs tls.ConnectionState) error {
			pool := x509.NewCertPool()
			for _, c := range cs.PeerCertificates {
				// derper also presents a "derpkey…" meta certificate carrying
				// its public key; it is not the TLS identity.
				if strings.HasPrefix(c.Subject.CommonName, derpconst.MetaCertCommonNamePrefix) {
					continue
				}
				if leaf == nil {
					leaf = c
					continue
				}
				pool.AddCert(c)
			}
			if leaf == nil {
				return errors.New("relay presented no certificate")
			}
			opts := x509.VerifyOptions{Intermediates: pool}
			if !isIP {
				opts.DNSName = r.Host
			}
			_, verifyErr = leaf.Verify(opts)
			return nil
		},
	}
	if !isIP {
		cfg.ServerName = r.Host
	}
	hostPort := net.JoinHostPort(r.Host, strconv.Itoa(r.Port))
	d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 8 * time.Second}, Config: cfg}
	conn, err := d.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		r.Err = err
		return
	}
	defer conn.Close()
	if leaf != nil {
		r.CertSHA256 = fmt.Sprintf("%02x", sha256.Sum256(leaf.Raw))
		r.CertTrusted = verifyErr == nil
		r.CertSubject = leaf.Subject.CommonName
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://"+hostPort+"/derp/probe", nil)
	if err != nil {
		r.Err = err
		return
	}
	req.Header.Set("User-Agent", "vget")
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	t0 := time.Now()
	if err := req.Write(conn); err != nil {
		r.Err = err
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		r.Err = err
		return
	}
	resp.Body.Close()
	r.RTT = time.Since(t0)
	if resp.StatusCode != http.StatusOK {
		r.Err = fmt.Errorf("GET /derp/probe returned %s (is this a DERP relay?)", resp.Status)
		return
	}
	r.Reachable = true
}

func (r *RelayTestResult) testSTUN(ctx context.Context) {
	if r.STUNPort < 0 {
		return
	}
	addr := net.JoinHostPort(r.Host, strconv.Itoa(r.STUNPort))
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "udp", addr)
	if err != nil {
		r.STUNErr = err
		return
	}
	defer conn.Close()
	txID := stun.NewTxID()
	t0 := time.Now()
	if _, err := conn.Write(stun.Request(txID)); err != nil {
		r.STUNErr = err
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		r.STUNErr = errors.New("no STUN reply (UDP port closed or filtered)")
		return
	}
	if _, _, err := stun.ParseResponse(buf[:n]); err != nil {
		r.STUNErr = err
		return
	}
	r.STUNOK = true
	r.STUNRTT = time.Since(t0)
}
