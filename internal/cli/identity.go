package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/tailcat"
)

var (
	identityFull   bool
	identityRotate bool
)

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Show this machine's tailcat identity (server address and client key)",
	Long: `Show the persistent tailcat identity used by "vget serve" and the client
key this machine presents when connecting to libraries.

The server address is a secret (it embeds the tunnel's pre-shared key), so it
is masked unless --full is given. --rotate replaces the server keys: the
address changes and every other machine must re-add this library.`,
	Args: cobra.NoArgs,
	RunE: runIdentity,
}

func init() {
	identityCmd.Flags().BoolVar(&identityFull, "full", false, "show the full server address")
	identityCmd.Flags().BoolVar(&identityRotate, "rotate", false, "generate a new server identity (new address)")
	rootCmd.AddCommand(identityCmd)
}

func runIdentity(cmd *cobra.Command, args []string) error {
	cfg := config.LoadOrDefault()
	t := i18n.T(cfg.Language)
	path, _ := tailcat.ServerIdentityPath()

	var id *tailcat.Identity
	var err error
	if identityRotate {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		id, err = tailcat.RotateServerIdentity(ctx, func(s string) { fmt.Fprintln(os.Stderr, "  "+s) })
		if err != nil {
			return err
		}
		fmt.Printf("⚠️  %s\n", t.Tailcat.IdentityRotated)
	} else {
		id, err = tailcat.LoadExistingServerIdentity()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	if id == nil {
		fmt.Printf("🐈 %s: %s\n", t.Tailcat.IdentityAddress, t.Tailcat.IdentityNone)
	} else {
		addr := id.Addr()
		if identityFull {
			fmt.Printf("🐈 %s:\n   %s\n", t.Tailcat.IdentityAddress, addr)
		} else {
			fmt.Printf("🐈 %s: %s  %s\n", t.Tailcat.IdentityAddress, tailcat.Mask(addr), t.Tailcat.IdentityFullHint)
		}
		fmt.Printf("   %s: %s\n", t.Tailcat.Relay, id.Relay())
		fmt.Printf("   file: %s\n", path)
	}
	fmt.Printf("🔑 %s: %s\n   (%s)\n", t.Tailcat.IdentityClientKey, tailcat.ClientPublicKey(), t.Tailcat.IdentityClientHint)
	return nil
}
