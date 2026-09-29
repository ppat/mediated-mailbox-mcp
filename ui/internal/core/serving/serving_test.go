package serving_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving"
)

func valid() serving.Config {
	return serving.Config{
		Listen: ":8443", ProbeListen: ":8080", TLSCert: "/tls/cert", TLSKey: "/tls/key",
		SyncInterval: 1, HeuristicsInterval: 1, StreamInterval: 1,
	}
}

// TestPlainHTTPIsRefusedOutsideTheDevLoop is the refusal of plain HTTP in a binary built without the
// devloop build tag, and its admission in one built with it (docs/UI.md section 18).
func TestPlainHTTPIsRefusedOutsideTheDevLoop(t *testing.T) {
	c := valid()
	c.InsecureHTTP, c.TLSCert, c.TLSKey = true, "", ""
	err := serving.Validate(c, false)
	if err == nil || err.Error() != "insecure_http is true, and a binary built without the devloop build tag serves TLS only" {
		t.Fatalf("plain HTTP outside the dev loop: %v", err)
	}
	if err := serving.Validate(c, true); err != nil {
		t.Fatalf("plain HTTP in the dev loop was refused: %v", err)
	}
}

// TestTheConfigurationIsValidated covers every other refusal and the valid configuration.
func TestTheConfigurationIsValidated(t *testing.T) {
	if err := serving.Validate(valid(), false); err != nil {
		t.Fatalf("a valid configuration was refused: %v", err)
	}
	cases := []struct {
		name   string
		change func(*serving.Config)
		want   string
	}{
		{"no listen", func(c *serving.Config) { c.Listen = " " }, "listen is empty"},
		{"no probe listen", func(c *serving.Config) { c.ProbeListen = "" }, "probe_listen is empty"},
		{"no certificate", func(c *serving.Config) { c.TLSCert = "" }, "tls_cert and tls_key are both required unless insecure_http is true"},
		{"no key", func(c *serving.Config) { c.TLSKey = "" }, "tls_cert and tls_key are both required unless insecure_http is true"},
		{"a zero sync interval", func(c *serving.Config) { c.SyncInterval = 0 }, "sync_interval is not positive"},
		{"a negative heuristics interval", func(c *serving.Config) { c.HeuristicsInterval = -1 }, "heuristics_interval is not positive"},
		{"a zero stream interval", func(c *serving.Config) { c.StreamInterval = 0 }, "stream_interval is not positive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.change(&c)
			if err := serving.Validate(c, true); err == nil || err.Error() != tc.want {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}
