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
	// A threshold of 0 disables its rule, so every threshold at 0 is valid.
	disabled := valid()
	disabled.AttentionBacklogShare, disabled.AttentionMaskCount, disabled.AttentionServeFactor, disabled.AttentionGapDays = 0, 0, 0, 0
	if err := serving.Validate(disabled, false); err != nil {
		t.Fatalf("every worth-a-look rule disabled was refused: %v", err)
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
			if err := serving.Validate(c, true); err == nil || err.Error() != tc.want {
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
		if err := serving.Validate(c, false); err != nil {
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
		err := serving.Validate(c, false)
		if err == nil || err.Error() != "consent_redirect is not an http address on a loopback IP literal with an explicit port" {
			t.Errorf("%q gave %v", bad, err)
		}
	}
}
