package core_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

func valid() core.Config {
	return core.Config{PublicKeyFile: "/run/keys/public", PrivateKeyFiles: []string{"/run/keys/old", "/run/keys/new"}}
}

// A value that names no key file is refused, so the keyring is never built from an empty path.
func TestAnUnusableValueIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		change func(*core.Config)
		want   string
	}{
		{"an empty public key path", func(c *core.Config) { c.PublicKeyFile = "" }, "credential.public_key_file is empty"},
		{"no private key", func(c *core.Config) { c.PrivateKeyFiles = nil }, "credential.private_key_files names no file"},
		{"an empty list of private keys", func(c *core.Config) { c.PrivateKeyFiles = []string{} }, "credential.private_key_files names no file"},
		{"an empty first private key path", func(c *core.Config) { c.PrivateKeyFiles[0] = "" }, "credential.private_key_files entry 0 is empty"},
		{"an empty later private key path", func(c *core.Config) { c.PrivateKeyFiles[1] = "" }, "credential.private_key_files entry 1 is empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			config := valid()
			c.change(&config)
			err := core.Validate(config)
			if err == nil {
				t.Fatalf("Validate accepted it, want %q", c.want)
			}
			if diff := cmp.Diff(c.want, err.Error(), compare.Options); diff != "" {
				t.Errorf("error (-want +got):\n%s", diff)
			}
		})
	}
}

// One private key, and several during a key replacement, are accepted.
func TestAUsableValueIsAccepted(t *testing.T) {
	for _, keys := range [][]string{{"/run/keys/current"}, {"/run/keys/old", "/run/keys/new"}} {
		config := valid()
		config.PrivateKeyFiles = keys
		if err := core.Validate(config); err != nil {
			t.Errorf("%v: %v", keys, err)
		}
	}
}

// The credential section is pinned field by field, so a new value, one meant to hold key material
// included, is a visible change (ADR-0078).
func TestTheSectionIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core", "Config",
		"PublicKeyFile string",
		"PrivateKeyFiles []string",
	)
}
