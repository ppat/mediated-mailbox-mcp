// Package serving is the pure validation of the UI's own configuration values, which checks the
// merged configuration before anything else starts (ADR-0078). The database section is validated
// where it is declared.
package serving

import (
	"strconv"
	"strings"
)

// Config is the UI's values apart from its database section (docs/UI.md section 18.1). The intervals
// are in nanoseconds, a time.Duration's own unit, because a pure core imports no time package.
type Config struct {
	Listen             string
	ProbeListen        string
	TLSCert            string
	TLSKey             string
	InsecureHTTP       bool
	SyncInterval       int64
	HeuristicsInterval int64
	StreamInterval     int64
	// The three values the browser reads (docs/UI.md section 18.1).
	DefaultTheme       string
	StreamReconnectMax int64
	StreamPollInterval int64
	// The worth-a-look thresholds (docs/UI.md section 18.1), each 0 to disable its rule.
	AttentionBacklogShare float64
	AttentionMaskCount    int64
	AttentionServeFactor  float64
	AttentionGapDays      int64
	// ConsentRedirect is the loopback address a consent redirects the browser to (docs/UI.md section
	// 8.12).
	ConsentRedirect string
	// IdentityHeader names the header an authenticating proxy forwards the identity in, and
	// OperatorName the identity recorded when no header is declared (ADR-0084).
	IdentityHeader string
	OperatorName   string
}

// Validate refuses an empty listen or probe_listen, plain HTTP in a binary built without the devloop
// build tag, TLS without both its files, an interval that is not positive, a default theme
// other than system, dark or light, a negative worth-a-look threshold, a consent redirect that is
// not a loopback address with an explicit port, and no identity to record, neither an identity header
// nor an operator name. devLoop is whether
// the binary was built with the tag. No image build sets it, so a deployed UI serves TLS only
// (docs/UI.md section 18).
func Validate(c Config, devLoop bool) error {
	switch {
	case strings.TrimSpace(c.Listen) == "":
		return refusal("listen is empty")
	case strings.TrimSpace(c.ProbeListen) == "":
		return refusal("probe_listen is empty")
	case c.InsecureHTTP && !devLoop:
		return refusal("insecure_http is true, and a binary built without the devloop build tag serves TLS only")
	case !c.InsecureHTTP && (c.TLSCert == "" || c.TLSKey == ""):
		return refusal("tls_cert and tls_key are both required unless insecure_http is true")
	case c.SyncInterval <= 0:
		return refusal("sync_interval is not positive")
	case c.HeuristicsInterval <= 0:
		return refusal("heuristics_interval is not positive")
	case c.StreamInterval <= 0:
		return refusal("stream_interval is not positive")
	case c.DefaultTheme != "system" && c.DefaultTheme != "dark" && c.DefaultTheme != "light":
		return refusal("default_theme is not system, dark or light")
	case c.StreamReconnectMax < 1_000_000:
		return refusal("stream_reconnect_max is under a millisecond")
	case c.StreamPollInterval < 1_000_000:
		return refusal("stream_poll_interval is under a millisecond")
	case c.AttentionBacklogShare < 0:
		return refusal("attention_backlog_share is negative")
	case c.AttentionMaskCount < 0:
		return refusal("attention_mask_count is negative")
	case c.AttentionServeFactor < 0:
		return refusal("attention_serve_factor is negative")
	case c.AttentionGapDays < 0:
		return refusal("attention_gap_days is negative")
	case !LoopbackRedirect(c.ConsentRedirect):
		return refusal("consent_redirect is not an http address on a loopback IP literal with an explicit port")
	case strings.TrimSpace(c.IdentityHeader) == "" && strings.TrimSpace(c.OperatorName) == "":
		return refusal("operator_name is required when identity_header is unset, since a policy write records who made it")
	}
	return nil
}

// LoopbackRedirect reports whether s can be a consent's redirect address, where nothing listens. It
// is http, its host a loopback IP literal, 127.0.0.0/8 or [::1], with an explicit port from 1 to
// 65535, and it carries no user, no path but /, no query and no fragment (docs/UI.md section 18.1).
// A name such as localhost is refused, since what it resolves to is not the configuration's to say.
func LoopbackRedirect(s string) bool {
	rest, ok := strings.CutPrefix(s, "http://")
	if !ok || strings.ContainsAny(rest, "?#@") {
		return false
	}
	authority, path, _ := strings.Cut(rest, "/")
	if path != "" {
		return false
	}
	var host, port string
	if strings.HasPrefix(authority, "[") {
		h, p, found := strings.Cut(authority, "]:")
		if !found || h != "[::1" {
			return false
		}
		host, port = "::1", p
	} else {
		i := strings.LastIndex(authority, ":")
		if i < 0 {
			return false
		}
		host, port = authority[:i], authority[i+1:]
		if !loopbackIPv4(host) {
			return false
		}
	}
	n, err := strconv.Atoi(port)
	return host != "" && err == nil && n >= 1 && n <= 65535 && strconv.Itoa(n) == port
}

// loopbackIPv4 reports whether s is a dotted IPv4 literal in 127.0.0.0/8.
func loopbackIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 || parts[0] != "127" {
		return false
	}
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 || strconv.Itoa(n) != p {
			return false
		}
	}
	return true
}

// Refusal is a configuration the UI refuses to start with.
type Refusal struct{ Reason string }

func (r *Refusal) Error() string { return r.Reason }

func refusal(reason string) error { return &Refusal{Reason: reason} }
