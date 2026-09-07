package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/tailcat"
)

var derpStunPort int

var derpCmd = &cobra.Command{
	Use:   "derp",
	Short: "Self-hosted relay (DERP) tools",
	Long: `Tools for running your own relay instead of the public tailcat relays.

A relay is needed for two vget machines to find each other and as a fallback
when NAT traversal fails. Run Tailscale's derper on any VPS; a bare IP with a
self-signed certificate is enough (no domain, no port 443):

  go install tailscale.com/cmd/derper@latest
  derper -a :8443 -certmode manual -hostname 1.2.3.4 -http-port -1 -stun-port 3478

Open 8443/tcp and 3478/udp, then check it and copy the fingerprint into
"vget serve --derp 1.2.3.4:8443 --derp-cert-sha256 <hex>".`,
}

var derpTestCmd = &cobra.Command{
	Use:   "test <host[:port]>",
	Short: "Check that a relay is reachable and print its certificate fingerprint",
	Args:  cobra.ExactArgs(1),
	RunE:  runDerpTest,
}

func init() {
	derpTestCmd.Flags().IntVar(&derpStunPort, "stun-port", 3478, "the relay's STUN UDP port")
	derpCmd.AddCommand(derpTestCmd)
	rootCmd.AddCommand(derpCmd)
}

func runDerpTest(cmd *cobra.Command, args []string) error {
	cfg := config.LoadOrDefault()
	t := i18n.T(cfg.Language)

	host, port, err := tailcat.ParseHostPort(args[0])
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r := tailcat.TestRelay(ctx, host, port, derpStunPort)

	hostPort := net.JoinHostPort(r.Host, strconv.Itoa(r.Port))
	stunHostPort := net.JoinHostPort(r.Host, strconv.Itoa(r.STUNPort))
	fmt.Printf("DERP %s\n", hostPort)
	if r.Reachable {
		fmt.Printf("  ✓ %s · RTT %s\n", t.Tailcat.DerpReachable, r.RTT.Round(time.Millisecond))
	} else {
		fmt.Printf("  ✗ %s: %v\n", t.Tailcat.DerpUnreachable, r.Err)
	}
	if r.CertSHA256 != "" {
		if r.CertTrusted {
			fmt.Printf("  ✓ %s\n", t.Tailcat.DerpCertTrusted)
		} else {
			fmt.Printf("  ! %s\n", t.Tailcat.DerpCertSelfSigned)
			fmt.Printf("    sha256: %s\n", r.CertSHA256)
			fmt.Printf("    %s\n", fmt.Sprintf(t.Tailcat.DerpCertHint, hostPort, r.CertSHA256))
		}
	}
	if r.STUNOK {
		fmt.Printf("  ✓ %s · %s\n", fmt.Sprintf(t.Tailcat.DerpStunOK, stunHostPort), r.STUNRTT.Round(time.Millisecond))
	} else {
		fmt.Printf("  ✗ %s\n", fmt.Sprintf(t.Tailcat.DerpStunFailed, stunHostPort))
	}
	if !r.Reachable {
		os.Exit(1)
	}
	return nil
}
