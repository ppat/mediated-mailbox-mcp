// Package serving is the pure validation of the UI's own configuration values, which checks the
// merged configuration before anything else starts (ADR-0078). The database section is validated
// where it is declared.
package serving

import "strings"

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
}

// Validate refuses an empty listen or probe_listen, plain HTTP in a binary built without the devloop
// build tag, TLS without both its files, and an interval that is not positive. devLoop is whether
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
	}
	return nil
}

// Refusal is a configuration the UI refuses to start with.
type Refusal struct{ Reason string }

func (r *Refusal) Error() string { return r.Reason }

func refusal(reason string) error { return &Refusal{Reason: reason} }
