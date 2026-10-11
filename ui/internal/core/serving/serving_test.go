package serving_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving"
)

func valid() serving.Config {
	return serving.Config{
		Listen: ":8443", ProbeListen: ":8080", TLSCert: "/tls/cert", TLSKey: "/tls/key",
		SyncInterval: 1, HeuristicsInterval: 1, StreamInterval: 1,
		DefaultTheme: "system", StreamReconnectMax: 1_000_000, StreamPollInterval: 1_000_000,
		ConsentRedirect: "http://127.0.0.1:47823/",
	}
}

// TestTLSIsBothFilesOrNeither admits both TLS files, serving TLS, and neither, serving plain HTTP, a
// blank path counting as unset, and refuses one without the other, naming the one missing (ADR-0127).
func TestTLSIsBothFilesOrNeither(t *testing.T) {
	admitted := []struct {
		name, cert, key string
		tls             bool
	}{
		{"both files", "/tls/cert", "/tls/key", true},
		{"neither file", "", "", false},
		{"both blank", " ", "\t", false},
	}
	for _, tc := range admitted {
		c := valid()
		c.TLSCert, c.TLSKey = tc.cert, tc.key
		if err := serving.Validate(c); err != nil {
			t.Errorf("%s was refused: %v", tc.name, err)
		}
		if got := serving.ServesTLS(c); got != tc.tls {
			t.Errorf("%s serves TLS %v, want %v", tc.name, got, tc.tls)
		}
	}
	certOnly := "tls_cert is set and tls_key is not, and the UI serves TLS with both or plain HTTP with neither"
	keyOnly := "tls_key is set and tls_cert is not, and the UI serves TLS with both or plain HTTP with neither"
	refused := []struct{ name, cert, key, want string }{
		{"a certificate without its key", "/tls/cert", "", certOnly},
		{"a key without its certificate", "", "/tls/key", keyOnly},
		{"a certificate with a blank key", "/tls/cert", "  ", certOnly},
		{"a key with a blank certificate", " ", "/tls/key", keyOnly},
	}
	for _, tc := range refused {
		c := valid()
		c.TLSCert, c.TLSKey = tc.cert, tc.key
		if err := serving.Validate(c); err == nil || err.Error() != tc.want {
			t.Errorf("%s gave %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestTheConfigurationIsValidated covers every refusal apart from the TLS files' and the consent
// redirect's, and the valid configuration.
func TestTheConfigurationIsValidated(t *testing.T) {
	if err := serving.Validate(valid()); err != nil {
		t.Fatalf("a valid configuration was refused: %v", err)
	}
	// A threshold of 0 disables its rule, so every threshold at 0 is valid.
	disabled := valid()
	disabled.AttentionBacklogShare, disabled.AttentionMaskCount, disabled.AttentionServeFactor, disabled.AttentionGapDays = 0, 0, 0, 0
	if err := serving.Validate(disabled); err != nil {
		t.Fatalf("every worth-a-look rule disabled was refused: %v", err)
	}
	cases := []struct {
		name   string
		change func(*serving.Config)
		want   string
	}{
		{"no listen", func(c *serving.Config) { c.Listen = " " }, "listen is empty"},
		{"no probe listen", func(c *serving.Config) { c.ProbeListen = "" }, "probe_listen is empty"},
		{"a zero sync interval", func(c *serving.Config) { c.SyncInterval = 0 }, "sync_interval is not positive"},
		{"a negative heuristics interval", func(c *serving.Config) { c.HeuristicsInterval = -1 }, "heuristics_interval is not positive"},
		{"a zero stream interval", func(c *serving.Config) { c.StreamInterval = 0 }, "stream_interval is not positive"},
		{"an unknown default theme", func(c *serving.Config) { c.DefaultTheme = "dim" }, "default_theme is not system, dark or light"},
		{"a reconnection ceiling under a millisecond", func(c *serving.Config) { c.StreamReconnectMax = 999_999 }, "stream_reconnect_max is under a millisecond"},
		{"a polling interval under a millisecond", func(c *serving.Config) { c.StreamPollInterval = 0 }, "stream_poll_interval is under a millisecond"},
		{"a negative backlog share", func(c *serving.Config) { c.AttentionBacklogShare = -0.5 }, "attention_backlog_share is negative"},
		{"a negative masking count", func(c *serving.Config) { c.AttentionMaskCount = -1 }, "attention_mask_count is negative"},
		{"a negative serve factor", func(c *serving.Config) { c.AttentionServeFactor = -2 }, "attention_serve_factor is negative"},
		{"a negative gap window", func(c *serving.Config) { c.AttentionGapDays = -7 }, "attention_gap_days is negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.change(&c)
			if err := serving.Validate(c); err == nil || err.Error() != tc.want {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

// TestTheConsentRedirectIsALoopbackAddressWithAPort admits an http address on a loopback IP literal with
// an explicit port, and refuses a value of any other shape, so a consent never redirects anywhere
// something could listen for its code (docs/UI.md section 18.1, VERIFICATIONS, the consent redirect
// row).
func TestTheConsentRedirectIsALoopbackAddressWithAPort(t *testing.T) {
	for _, ok := range []string{"http://127.0.0.1:47823/", "http://127.0.0.1:47823", "http://127.4.5.6:1/", "http://[::1]:65535/"} {
		c := valid()
		c.ConsentRedirect = ok
		if err := serving.Validate(c); err != nil {
			t.Errorf("%q was refused: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"", "https://127.0.0.1:47823/", "http://localhost:47823/", "http://127.0.0.1/", "http://127.0.0.1:/",
		"http://10.0.0.1:47823/", "http://128.0.0.1:47823/", "http://127.0.0.1:0/", "http://127.0.0.1:65536/",
		"http://127.0.0.1:047823/", "http://127.0.0.1:47823/callback", "http://127.0.0.1:47823/?x=1",
		"http://127.0.0.1:47823/#f", "http://user@127.0.0.1:47823/", "http://[::2]:47823/", "http://[::1]/",
		"http://127.0.0.01:47823/", "http://127.0.0:47823/", "127.0.0.1:47823",
	} {
		c := valid()
		c.ConsentRedirect = bad
		err := serving.Validate(c)
		if err == nil || err.Error() != "consent_redirect is not an http address on a loopback IP literal with an explicit port" {
			t.Errorf("%q gave %v", bad, err)
		}
	}
}
