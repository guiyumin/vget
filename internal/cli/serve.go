package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/serve"
	"github.com/guiyumin/vget/internal/core/tailcat"
)

// serveFlags are shared by serve, send and recv; each command registers the
// subset that makes sense for it.
type serveFlags struct {
	user         string
	pass         string
	noAuth       bool
	listen       string
	useTailcat   bool
	ephemeral    bool
	rw           bool
	inbox        string
	allow        []string
	derp         string
	derpCert     string
	derpSTUNPort int
	once         bool
	verbose      bool
}

var (
	serveOpts serveFlags
	sendOpts  serveFlags
	recvOpts  serveFlags
)

var serveCmd = &cobra.Command{
	Use:   "serve [dir]",
	Short: "Share a directory as a library, reachable without a public IP",
	Long: `Share a directory as a read-only WebDAV library with Basic Auth.

By default the library gets a tailcat address: an encrypted tunnel that works
across NATs with no public IP, domain, certificate or account. The address is
stable across restarts (identity in ~/.config/vget/tailcat/server.json) and
contains a secret, so hand it out like a password.

Examples:
  vget serve ~/library --user alice --pass secret
  vget serve ~/library --user alice --pass secret --inbox ~/inbox   # accept uploads at /inbox
  vget serve ~/library --rw --user alice --pass secret               # writable library
  vget serve ~/library --listen :7777 --tailcat=false                # plain TCP (LAN, Tailscale, reverse proxy)
  vget serve ~/library --derp 1.2.3.4:8443 --derp-cert-sha256 <hex>  # self-hosted relay

The other side:
  vget config webdav add mylib <tc-address> --user alice --pass secret
  vget ls mylib:/

Defaults can live in config.yml under "serve:" (dir, user, password, tailcat,
listen, rw, inbox, derp, allow); the account is re-read on every request.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return (&serveRun{cmd: cmd, f: &serveOpts, mode: modeServe, args: args}).run()
	},
}

var sendCmd = &cobra.Command{
	Use:   "send <file>...",
	Short: "Send files to another machine through a one-time address",
	Long: `Serve the given files (or directory trees) read-only under a one-time
tailcat address. No account is needed: the address itself is the credential
and dies with this command.

  A$ vget send big.iso
  B$ vget <tc-address>

Use --once to exit as soon as everything has been downloaded, or --user/--pass
to require a password as well.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return (&serveRun{cmd: cmd, f: &sendOpts, mode: modeSend, args: args}).run()
	},
}

var recvCmd = &cobra.Command{
	Use:   "recv [dir]",
	Short: "Receive files from another machine into a write-only inbox",
	Long: `Open a one-time address that accepts uploads into dir (default: the
current directory). Senders can only add files: they cannot list, read or
overwrite anything.

  A$ vget recv ~/inbox
  B$ vget push reply.zip <tc-address>`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return (&serveRun{cmd: cmd, f: &recvOpts, mode: modeRecv, args: args}).run()
	},
}

func init() {
	addTransportFlags := func(cmd *cobra.Command, f *serveFlags) {
		cmd.Flags().StringVar(&f.listen, "listen", "", "also serve plain HTTP on this address, e.g. :7777")
		cmd.Flags().StringSliceVar(&f.allow, "allow", nil, "only accept these client keys (see 'vget identity'), comma-separated")
		cmd.Flags().StringVar(&f.derp, "derp", "", "self-hosted relay host[:port] (default: the public tailcat relays)")
		cmd.Flags().StringVar(&f.derpCert, "derp-cert-sha256", "", "relay certificate fingerprint, for a self-signed relay (see 'vget derp test')")
		cmd.Flags().IntVar(&f.derpSTUNPort, "derp-stun-port", 0, "relay STUN port (default 3478)")
		cmd.Flags().BoolVar(&f.verbose, "verbose", false, "log tailcat internals")
	}
	addAuthFlags := func(cmd *cobra.Command, f *serveFlags) {
		cmd.Flags().StringVar(&f.user, "user", "", "account name clients must present")
		cmd.Flags().StringVar(&f.pass, "pass", "", "account password")
	}

	addAuthFlags(serveCmd, &serveOpts)
	serveCmd.Flags().BoolVar(&serveOpts.noAuth, "no-auth", false, "serve without any account (anyone with the address can read)")
	serveCmd.Flags().BoolVar(&serveOpts.useTailcat, "tailcat", true, "open a tailcat address")
	serveCmd.Flags().BoolVar(&serveOpts.ephemeral, "ephemeral", false, "use a throwaway identity instead of the persistent one")
	serveCmd.Flags().BoolVar(&serveOpts.rw, "rw", false, "let clients write to the library")
	serveCmd.Flags().StringVar(&serveOpts.inbox, "inbox", "", "directory accepting uploads at /inbox (write-only)")
	addTransportFlags(serveCmd, &serveOpts)

	addAuthFlags(sendCmd, &sendOpts)
	sendCmd.Flags().BoolVar(&sendOpts.once, "once", false, "exit once everything has been downloaded")
	addTransportFlags(sendCmd, &sendOpts)

	addAuthFlags(recvCmd, &recvOpts)
	addTransportFlags(recvCmd, &recvOpts)

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(recvCmd)
}

type serveMode int

const (
	modeServe serveMode = iota
	modeSend
	modeRecv
)

type serveRun struct {
	cmd  *cobra.Command
	f    *serveFlags
	mode serveMode
	args []string

	cfg *config.Config
	t   *i18n.Translations
}

func (r *serveRun) changed(name string) bool { return r.cmd.Flags().Changed(name) }

func (r *serveRun) run() error {
	r.cfg = config.LoadOrDefault()
	r.t = i18n.T(r.cfg.Language)
	t := r.t

	opts := serve.Options{OnEvent: r.onEvent}
	useTailcat := true
	ephemeral := r.f.ephemeral
	listen := r.f.listen
	allow := r.f.allow

	switch r.mode {
	case modeServe:
		dir := r.cfg.Serve.Dir
		if len(r.args) == 1 {
			dir = r.args[0]
		}
		if dir == "" {
			dir = "."
		}
		opts.Dir = dir
		opts.ReadWrite = r.f.rw || r.cfg.Serve.RW
		opts.Inbox = r.f.inbox
		if opts.Inbox == "" {
			opts.Inbox = r.cfg.Serve.Inbox
		}
		if r.changed("tailcat") {
			useTailcat = r.f.useTailcat
		} else {
			useTailcat = r.cfg.Serve.TailcatEnabled()
		}
		if listen == "" {
			listen = r.cfg.Serve.Listen
		}
		if len(allow) == 0 {
			allow = r.cfg.Serve.Allow
		}
	case modeSend:
		opts.Files = r.args
		opts.Once = r.f.once
		ephemeral = true
	case modeRecv:
		dir := "."
		if len(r.args) == 1 {
			dir = r.args[0]
		}
		opts.Inbox = dir
		ephemeral = true
	}

	// Authentication: flags win; a served library falls back to the config
	// account, re-read on every request; one-time modes default to none.
	switch {
	case r.f.noAuth:
	case r.f.user != "":
		user, pass := r.f.user, r.f.pass
		opts.Auth = func() (string, string, bool) { return user, pass, true }
	case r.mode == modeServe && r.cfg.Serve.User != "":
		opts.Auth = func() (string, string, bool) {
			c := config.LoadOrDefault()
			return c.Serve.User, c.Serve.Password, c.Serve.User != ""
		}
	case r.mode == modeServe:
		return errors.New(t.Tailcat.AuthRequired)
	}

	handler, err := serve.NewHandler(opts)
	if err != nil {
		return err
	}
	if !useTailcat && listen == "" {
		return errors.New("nothing to listen on: keep tailcat enabled or give --listen")
	}

	var listeners []net.Listener
	var tc *tailcat.Server
	var created, relayChanged bool
	if useTailcat {
		derp, err := r.derpOption()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		var id *tailcat.Identity
		id, created, relayChanged, err = tailcat.LoadServerIdentity(ctx, tailcat.ServerIdentityOptions{
			Ephemeral: ephemeral,
			DERP:      derp,
			Status:    func(s string) { fmt.Fprintln(os.Stderr, "  "+s) },
		})
		cancel()
		if err != nil {
			return err
		}
		if !ephemeral {
			release, err := tailcat.LockServerIdentity()
			if err != nil {
				return err
			}
			defer release()
		}
		var logf func(string, ...any)
		if r.f.verbose {
			logf = log.Printf
		}
		tc, err = tailcat.Listen(tailcat.ListenOptions{Identity: id, Allow: allow, Logf: logf})
		if err != nil {
			return err
		}
		listeners = append(listeners, tc.Listener())
	}
	if listen != "" {
		ln, err := net.Listen("tcp", listen)
		if err != nil {
			if tc != nil {
				tc.Close(context.Background())
			}
			return fmt.Errorf("listen %s: %w", listen, err)
		}
		listeners = append(listeners, ln)
	}

	r.printBanner(handler, opts, tc, listeners, created, relayChanged)

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 30 * time.Second}
	errCh := make(chan error, len(listeners))
	for _, ln := range listeners {
		go func(ln net.Listener) {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}(ln)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		fmt.Println()
		fmt.Println("  " + t.Tailcat.Stopping)
	case <-handler.Done():
		fmt.Println("  ✓ " + t.Tailcat.TransferComplete)
	case err := <-errCh:
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	if tc != nil {
		_ = tc.Close(ctx)
	}
	return nil
}

// derpOption resolves the self-hosted relay from flags, then config.
func (r *serveRun) derpOption() (*tailcat.DERPOption, error) {
	host, port := "", 0
	if r.f.derp != "" {
		h, p, err := tailcat.ParseHostPort(r.f.derp)
		if err != nil {
			return nil, err
		}
		host, port = h, p
	}
	cert, stun := r.f.derpCert, r.f.derpSTUNPort
	if host == "" && r.mode == modeServe && r.cfg.Serve.DERP.Host != "" {
		host, port = r.cfg.Serve.DERP.Host, r.cfg.Serve.DERP.Port
		if cert == "" {
			cert = r.cfg.Serve.DERP.CertSHA256
		}
		if !r.changed("derp-stun-port") {
			stun = r.cfg.Serve.DERP.STUNPort
		}
	}
	if host == "" {
		return nil, nil
	}
	return &tailcat.DERPOption{Host: host, Port: port, STUNPort: stun, CertSHA256: cert}, nil
}

func (r *serveRun) printBanner(h *serve.Handler, opts serve.Options, tc *tailcat.Server, listeners []net.Listener, created, relayChanged bool) {
	t := r.t
	abs := func(p string) string {
		if a, err := filepath.Abs(p); err == nil {
			return a
		}
		return p
	}

	fmt.Println()
	switch r.mode {
	case modeServe:
		mode := t.Tailcat.ReadOnly
		if opts.ReadWrite {
			mode = t.Tailcat.ReadWrite
		}
		fmt.Printf("📚 %s\n", fmt.Sprintf(t.Tailcat.Serving, abs(opts.Dir), mode))
		if opts.Inbox != "" {
			fmt.Printf("   %s\n", fmt.Sprintf(t.Tailcat.InboxAt, abs(opts.Inbox), h.InboxPath()))
		}
		if opts.Auth == nil {
			fmt.Printf("   ⚠️  %s\n", t.Tailcat.NoAuthWarning)
		}
	case modeSend:
		line := fmt.Sprintf(t.Tailcat.ServingFiles, h.FileCount, formatSize(h.TotalSize))
		if opts.Once {
			line += ", " + t.Tailcat.ExitsAfterDownload
		}
		fmt.Printf("📦 %s:\n", line)
	case modeRecv:
		fmt.Printf("📥 %s\n", fmt.Sprintf(t.Tailcat.Receiving, abs(opts.Inbox)))
	}

	var addr string
	if tc != nil {
		addr = tc.Addr()
		if r.mode == modeServe {
			fmt.Printf("🐈 %s\n   %s\n", t.Tailcat.AddressLabel, addr)
			if created {
				fmt.Printf("   %s\n", fmt.Sprintf(t.Tailcat.IdentityCreated, tc.Relay()))
			}
			if relayChanged {
				fmt.Printf("   ⚠️  %s\n", t.Tailcat.RelayChanged)
			}
		} else {
			fmt.Printf("🐈 %s\n", addr)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		lat, err := tc.RelayLatency(ctx)
		cancel()
		if err != nil {
			fmt.Printf("   %s: %s · %s\n", t.Tailcat.Relay, tc.Relay(), t.Tailcat.RelayNoReply)
		} else {
			fmt.Printf("   %s: %s %s · %s\n", t.Tailcat.Relay, tc.Relay(), lat.Round(time.Millisecond), t.Tailcat.RelayDirectHint)
		}
	}
	var httpURL string
	for _, ln := range listeners {
		if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
			host := tcpAddr.IP.String()
			if tcpAddr.IP.IsUnspecified() {
				host = localIP()
			}
			httpURL = fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprint(tcpAddr.Port)))
			fmt.Printf("🔌 %s %s\n", t.Tailcat.AlsoListening, httpURL)
		}
	}

	fmt.Println()
	target := addr
	if target == "" {
		target = httpURL
	}
	switch r.mode {
	case modeServe:
		fmt.Println(t.Tailcat.OnOtherMachine)
		name := "mylib"
		if opts.Dir != "" {
			if base := filepath.Base(abs(opts.Dir)); base != "" && base != "/" && base != "." {
				name = strings.Map(func(c rune) rune {
					if c == ' ' || c == ':' || c == '/' {
						return '-'
					}
					return c
				}, strings.ToLower(base))
			}
		}
		add := fmt.Sprintf("   vget config webdav add %s %s", name, target)
		if r.f.user != "" {
			add += " --user " + r.f.user
		} else if opts.Auth != nil {
			add += " --user " + r.cfg.Serve.User
		}
		fmt.Println(add)
		fmt.Printf("   vget ls %s:/\n", name)
		if h.HasInbox() {
			fmt.Printf("   vget push <file> %s:%s/\n", name, h.InboxPath())
		}
	case modeSend:
		fmt.Println(t.Tailcat.OnOtherSide)
		if r.f.user != "" {
			fmt.Printf("   vget %s --user %s --pass ...\n", target, r.f.user)
		} else {
			fmt.Printf("   vget %s\n", target)
		}
	case modeRecv:
		fmt.Println(t.Tailcat.OnOtherSide)
		if r.f.user != "" {
			fmt.Printf("   vget push <file> %s --user %s --pass ...\n", target, r.f.user)
		} else {
			fmt.Printf("   vget push <file> %s\n", target)
		}
	}
	fmt.Println()
	fmt.Printf("  %s\n", t.Tailcat.PressCtrlC)
}

// onEvent prints what happens while serving.
func (r *serveRun) onEvent(e serve.Event) {
	t := r.t
	stamp := time.Now().Format("15:04:05")
	switch e.Kind {
	case serve.EventSent:
		fmt.Printf("  %s ✓ %s\n", stamp, fmt.Sprintf(t.Tailcat.Sent, strings.TrimPrefix(e.Name, "/"), formatSize(e.Size), describeRemote(e.Remote)))
	case serve.EventReceived:
		fmt.Printf("  %s ✓ %s\n", stamp, fmt.Sprintf(t.Tailcat.Received, e.Name, formatSize(e.Size), e.Path))
	case serve.EventAuthFailed:
		fmt.Printf("  %s ✗ %s\n", stamp, fmt.Sprintf(t.Tailcat.AuthFailed, e.Name, describeRemote(e.Remote)))
	case serve.EventError:
		if r.f.verbose {
			fmt.Printf("  %s ! %s: %v\n", stamp, e.Name, e.Err)
		}
	}
}

// describeRemote turns a client address into something readable: tailcat
// clients have tunnel addresses derived from their keys, which say nothing
// to a human.
func describeRemote(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	if strings.HasPrefix(host, "fd7a:115c:a1e0:") {
		return "tailcat client"
	}
	return host
}

// localIP returns the machine's preferred outbound IPv4 address, for
// showing a usable --listen URL. No packets are sent.
func localIP() string {
	conn, err := net.Dial("udp4", "192.0.2.1:1")
	if err != nil {
		return "localhost"
	}
	defer conn.Close()
	if a, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return a.IP.String()
	}
	return "localhost"
}
