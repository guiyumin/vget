package tailcat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/tailcat"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"

	"github.com/guiyumin/vget/internal/core/config"
)

// Identity is a server's persistent tailcat identity: node key, WireGuard
// pre-shared key and relay region, i.e. everything its tailcat address is
// made of. It lives in ~/.config/vget/tailcat/server.json (mode 0600) and is
// kept out of config.yml, which is world-readable and printed by
// "vget config show".
type Identity struct {
	pk *tailcat.PrivateKey
}

// ErrIdentityBusy is returned by LockServerIdentity when another vget process
// is already serving with the persistent identity. A relay only keeps the
// last connection for a given key, so two servers would fight over it.
var ErrIdentityBusy = errors.New("the server identity is already in use by another vget process (use --ephemeral for a one-off address)")

// Dir returns the directory holding identity files (~/.config/vget/tailcat).
func Dir() (string, error) {
	d, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "tailcat"), nil
}

// ServerIdentityPath returns the path of the persistent server identity.
func ServerIdentityPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "server.json"), nil
}

// ClientIdentityPath returns the path of the persistent client key.
func ClientIdentityPath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "client.json"), nil
}

func readIdentity(path string) (*tailcat.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pk tailcat.PrivateKey
	if err := json.Unmarshal(data, &pk); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if pk.Private.IsZero() {
		return nil, fmt.Errorf("%s: missing private key", path)
	}
	return &pk, nil
}

func writeIdentity(path string, pk *tailcat.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pk, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ServerIdentityOptions controls LoadServerIdentity.
type ServerIdentityOptions struct {
	// Ephemeral generates a throwaway identity that is never written to disk.
	Ephemeral bool

	// DERP, if non-nil, is a self-hosted relay to use. A persistent identity
	// remembers it, so later runs need no --derp flag. If nil, a persistent
	// identity keeps the relay it was created with; a new identity picks the
	// nearest public relay once.
	DERP *DERPOption

	// Status, if non-nil, receives progress messages for slow steps.
	Status func(string)
}

// LoadServerIdentity returns the server identity described by opts, creating
// the persistent one on first use. created reports that a new persistent
// identity was written; relayChanged reports that an existing identity's
// relay was replaced, which changes its address.
func LoadServerIdentity(ctx context.Context, opts ServerIdentityOptions) (id *Identity, created, relayChanged bool, err error) {
	status := opts.Status
	if status == nil {
		status = func(string) {}
	}
	var want *tailcfg.DERPRegion
	if opts.DERP != nil {
		want, err = opts.DERP.Region()
		if err != nil {
			return nil, false, false, err
		}
	}
	pickRegion := func() (*tailcfg.DERPRegion, error) {
		if want != nil {
			return want, nil
		}
		status("Picking the nearest relay…")
		return NearestPublicRegion(ctx)
	}

	if opts.Ephemeral {
		reg, err := pickRegion()
		if err != nil {
			return nil, false, false, err
		}
		pk := tailcat.NewPrivateKey()
		pk.Public.Region = []*tailcfg.DERPRegion{reg}
		return &Identity{pk}, false, false, nil
	}

	path, err := ServerIdentityPath()
	if err != nil {
		return nil, false, false, err
	}
	pk, err := readIdentity(path)
	switch {
	case err == nil:
		if want != nil && !sameRelay(pk.Public.Region, want) {
			pk.Public.Region = []*tailcfg.DERPRegion{want}
			pk.Public.RegionID = 0
			relayChanged = true
		}
		if len(pk.Public.Region) == 0 {
			reg, err := pickRegion()
			if err != nil {
				return nil, false, false, err
			}
			pk.Public.Region = []*tailcfg.DERPRegion{reg}
			pk.Public.RegionID = 0
			relayChanged = true
		}
		if relayChanged {
			if err := writeIdentity(path, pk); err != nil {
				return nil, false, false, err
			}
		}
		return &Identity{pk}, false, relayChanged, nil
	case errors.Is(err, os.ErrNotExist):
		reg, err := pickRegion()
		if err != nil {
			return nil, false, false, err
		}
		pk = tailcat.NewPrivateKey()
		pk.Public.Region = []*tailcfg.DERPRegion{reg}
		if err := writeIdentity(path, pk); err != nil {
			return nil, false, false, err
		}
		return &Identity{pk}, true, false, nil
	default:
		return nil, false, false, err
	}
}

// LoadExistingServerIdentity returns the persistent server identity, or
// os.ErrNotExist if none has been created yet.
func LoadExistingServerIdentity() (*Identity, error) {
	path, err := ServerIdentityPath()
	if err != nil {
		return nil, err
	}
	pk, err := readIdentity(path)
	if err != nil {
		return nil, err
	}
	return &Identity{pk}, nil
}

// RotateServerIdentity replaces the persistent server's keys (and therefore
// its address), keeping the relay it already uses.
func RotateServerIdentity(ctx context.Context, status func(string)) (*Identity, error) {
	path, err := ServerIdentityPath()
	if err != nil {
		return nil, err
	}
	var reg *tailcfg.DERPRegion
	if old, err := readIdentity(path); err == nil && len(old.Public.Region) > 0 {
		reg = old.Public.Region[0]
	}
	if reg == nil {
		if status != nil {
			status("Picking the nearest relay…")
		}
		reg, err = NearestPublicRegion(ctx)
		if err != nil {
			return nil, err
		}
	}
	pk := tailcat.NewPrivateKey()
	pk.Public.Region = []*tailcfg.DERPRegion{reg}
	if err := writeIdentity(path, pk); err != nil {
		return nil, err
	}
	return &Identity{pk}, nil
}

// LockServerIdentity takes an exclusive lock on the persistent identity for
// the life of the process. It returns ErrIdentityBusy if another vget holds it.
func LockServerIdentity() (release func(), err error) {
	path, err := ServerIdentityPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return lockFile(path + ".lock")
}

// Addr returns the identity's tailcat address, with the relay embedded so
// clients need no network access to a DERP map.
func (id *Identity) Addr() string { return string(id.pk.Public.Addr()) }

// Region returns the identity's relay region.
func (id *Identity) Region() *tailcfg.DERPRegion {
	if len(id.pk.Public.Region) == 0 {
		return nil
	}
	return id.pk.Public.Region[0]
}

// Relay describes the identity's relay, e.g. "302 San Francisco".
func (id *Identity) Relay() string { return describeRegion(id.Region()) }

// SelfHostedRelay reports whether the identity uses a self-hosted relay.
func (id *Identity) SelfHostedRelay() bool {
	r := id.Region()
	return r != nil && r.RegionID == ownRegionID
}

// ClientKey returns this machine's persistent client node key, creating
// ~/.config/vget/tailcat/client.json on first use. If the file cannot be
// written, an ephemeral key is returned so connecting still works.
func ClientKey() key.NodePrivate {
	path, err := ClientIdentityPath()
	if err != nil {
		return key.NewNode()
	}
	if pk, err := readIdentity(path); err == nil {
		return pk.Private
	}
	pk := tailcat.NewPrivateKey()
	pk.Public = tailcat.ConnInfo{} // a client identity is just a node key
	if err := writeIdentity(path, pk); err != nil {
		return pk.Private
	}
	return pk.Private
}

// ClientPublicKey returns the persistent client public key ("nodekey:…"),
// the value a server owner puts in --allow.
func ClientPublicKey() string { return ClientKey().Public().String() }

// ParseAllowedKeys parses "nodekey:…" strings as given to --allow.
func ParseAllowedKeys(list []string) ([]key.NodePublic, error) {
	var keys []key.NodePublic
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		var k key.NodePublic
		if err := k.UnmarshalText([]byte(s)); err != nil {
			return nil, fmt.Errorf("invalid client key %q in --allow: %v", s, err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}
