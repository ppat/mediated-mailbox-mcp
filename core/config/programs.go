package config

import (
	"errors"
	"strconv"
	"strings"
)

// API is the API server's configuration. The process's parts and the accounts' part are inline, so
// every key they hold sits at the top of the tree beside the API server's own. No account and no
// provider credential is configuration. Both come from the database (ADR-0080).
type API struct {
	Process    `yaml:",inline"`
	Operations `yaml:",inline"`
	Serving    `yaml:",inline"`
	// TokenFile names the mounted file holding the bearer token clients present.
	TokenFile string `yaml:"token_file" settings:"required"`
	Accounts  `yaml:",inline"`
	// ProviderTimeout bounds each provider call a body request makes, after which the call is the
	// provider's failure (ADR-0101).
	ProviderTimeout Duration `yaml:"provider_timeout"`
	Database        Database `yaml:"database"`
	Credential      Keys     `yaml:"credential"`
	// Scanner is the scanner's part, the same part backfill reads, whose pattern tier the serve-time
	// pattern check runs (ADR-0002, ADR-0005). No value in it switches the check off.
	Scanner Scanner `yaml:"scanner"`
}

// maxProviderTimeout is the longest provider timeout the API server accepts.
const maxProviderTimeout = 5 * minute

// DefaultAPI is the API server's defaults on a node with processors processors. The pool size is the
// driver's own default. The token file, the database's host, name, user and password file and the
// key files have no default, since a default path assumes the environment and the project names no
// database user (ADR-0143).
func DefaultAPI(processors int) API {
	return API{
		Process:         DefaultProcess(),
		Operations:      DefaultOperations(),
		Serving:         DefaultServing(),
		Accounts:        DefaultAccounts(),
		ProviderTimeout: 30 * second,
		Database:        DefaultDatabase(DriverPoolSize(processors)),
		Scanner:         DefaultScanner(),
	}
}

// Check refuses a configuration the API server cannot serve with. Its token file is named, its
// provider timeout positive and at most five minutes, and every part it has passes its own check.
func (c API) Check() error {
	return first(
		c.Process.Check,
		c.Operations.Check,
		c.Serving.Check,
		func() error {
			if strings.TrimSpace(c.TokenFile) == "" {
				return errors.New("token_file is empty")
			}
			return nil
		},
		c.Accounts.Check,
		func() error {
			if c.ProviderTimeout <= 0 || c.ProviderTimeout > maxProviderTimeout {
				return errors.New("provider_timeout is not positive and at most 5m0s")
			}
			return nil
		},
		c.Database.Check,
		c.Credential.Check,
		scanner(c.Scanner),
	)
}

// UI is the UI's configuration, under the keys docs/UI.md section 18.1 declares. The process's parts
// are inline, so every key they hold sits at the top of the tree beside the UI's own. Only the values
// the UI reads are declared.
type UI struct {
	Process            `yaml:",inline"`
	Operations         `yaml:",inline"`
	Serving            `yaml:",inline"`
	SyncInterval       Duration `yaml:"sync_interval"`
	HeuristicsInterval Duration `yaml:"heuristics_interval"`
	StreamInterval     Duration `yaml:"stream_interval"`
	DefaultTheme       string   `yaml:"default_theme"`
	StreamReconnectMax Duration `yaml:"stream_reconnect_max"`
	StreamPollInterval Duration `yaml:"stream_poll_interval"`
	// The worth-a-look thresholds of docs/UI.md section 8.1, each 0 to disable its rule.
	AttentionBacklogShare float64 `yaml:"attention_backlog_share"`
	AttentionMaskCount    int64   `yaml:"attention_mask_count"`
	AttentionServeFactor  float64 `yaml:"attention_serve_factor"`
	AttentionGapDays      int64   `yaml:"attention_gap_days"`
	// TokenKeyFile names a mounted key behind the request token and the consent attempt's seal, which
	// replicas share. Unset, the UI generates one at its assembly (ADR-0061, ADR-0111).
	TokenKeyFile string `yaml:"token_key_file"`
	// ConsentRedirect is the loopback address a consent redirects the browser to, where nothing
	// listens (docs/UI.md sections 8.12 and 18.1).
	ConsentRedirect string `yaml:"consent_redirect"`
	// IdentityHeader names the header an authenticating proxy forwards the operator's identity in, and
	// OperatorName the identity a decision or a policy write records when no header is declared
	// (ADR-0084), so every write has one.
	IdentityHeader string   `yaml:"identity_header"`
	OperatorName   string   `yaml:"operator_name"`
	Database       Database `yaml:"database"`
	// Credential is the credential part the API server and the worker hold, under the same keys. Its
	// public key is the one the UI seals credentials and client secrets to. Its private keys are the
	// keyring the programs that call a provider open credentials with, which only the UI's
	// client-secret package holds, and it opens a client's secret for a consent's code exchange and
	// nothing else (ADR-0081, ADR-0092).
	Credential Keys `yaml:"credential"`
}

// DefaultUI is the UI's defaults on a node with processors processors. The intervals shown on the
// jobs cards are ADR-0018's sync interval and ADR-0117's heuristics run. The server's stream poll and
// the browser stream client's backoff and fallback poll are ADR-0058's, and the browser follows the
// OS theme. The worth-a-look thresholds are section 8.1's starting values. A consent redirects to
// 127.0.0.1 on a high port a web server on the operator's computer is unlikely to answer on, which an
// installation may name otherwise (section 8.12). With no identity header declared, a write records
// the operator name set here, which configuration may replace. The pool size is the driver's own
// default.
func DefaultUI(processors int) UI {
	return UI{
		Process:               DefaultProcess(),
		Operations:            DefaultOperations(),
		Serving:               DefaultServing(),
		SyncInterval:          5 * minute,
		HeuristicsInterval:    24 * hour,
		StreamInterval:        2 * second,
		DefaultTheme:          "system",
		StreamReconnectMax:    30 * second,
		StreamPollInterval:    5 * second,
		AttentionBacklogShare: 5,
		AttentionMaskCount:    20,
		AttentionServeFactor:  2,
		AttentionGapDays:      7,
		ConsentRedirect:       "http://127.0.0.1:47823/",
		OperatorName:          "operator",
		Database:              DefaultDatabase(DriverPoolSize(processors)),
	}
}

// Check refuses a configuration the UI cannot serve with. Every interval is positive, the default
// theme is system, dark or light, no worth-a-look threshold is negative, the consent redirect is a
// loopback address with an explicit port, and every part it has passes its own check.
func (c UI) Check() error {
	return first(
		c.Process.Check,
		c.Operations.Check,
		c.Serving.Check,
		c.Database.Check,
		func() error {
			switch {
			case c.SyncInterval <= 0:
				return errors.New("sync_interval is not positive")
			case c.HeuristicsInterval <= 0:
				return errors.New("heuristics_interval is not positive")
			case c.StreamInterval <= 0:
				return errors.New("stream_interval is not positive")
			case c.DefaultTheme != "system" && c.DefaultTheme != "dark" && c.DefaultTheme != "light":
				return errors.New("default_theme is not system, dark or light")
			case c.StreamReconnectMax < millisecond:
				return errors.New("stream_reconnect_max is under a millisecond")
			case c.StreamPollInterval < millisecond:
				return errors.New("stream_poll_interval is under a millisecond")
			case c.AttentionBacklogShare < 0:
				return errors.New("attention_backlog_share is negative")
			case c.AttentionMaskCount < 0:
				return errors.New("attention_mask_count is negative")
			case c.AttentionServeFactor < 0:
				return errors.New("attention_serve_factor is negative")
			case c.AttentionGapDays < 0:
				return errors.New("attention_gap_days is negative")
			case !loopbackRedirect(c.ConsentRedirect):
				return errors.New("consent_redirect is not an http address on a loopback IP literal with an explicit port")
			}
			return nil
		},
		c.Credential.Check,
	)
}

// loopbackRedirect reports whether s can be a consent's redirect address, where nothing listens. It
// is http, its host a loopback IP literal, 127.0.0.0/8 or [::1], with an explicit port from 1 to
// 65535, and it carries no user, no path but /, no query and no fragment (docs/UI.md section 18.1).
// A name such as localhost is refused, since what it resolves to is not the configuration's to say.
func loopbackRedirect(s string) bool {
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

// Worker is the worker's configuration. The process's parts are inline, so every key they hold sits
// at the top of the tree beside the worker's own, with the parts every job kind shares, read once,
// among them the one database part, and a section per job kind. No account and no provider
// credential is configuration. Both come from the database (ADR-0080).
type Worker struct {
	Process    `yaml:",inline"`
	Operations `yaml:",inline"`
	// ReloadInterval is how often each job kind's listing is asked beside the wake each reload of the
	// Session gives it, and how often backfill asks each account whether it has work (ADR-0119).
	ReloadInterval Duration `yaml:"reload_interval"`
	// Accounts is the accounts part, whose interval paces the Session's reload of the accounts, their
	// connections and their policy, and so how soon an account connected, a credential replaced or a
	// policy edited reaches the worker (ADR-0090).
	Accounts `yaml:",inline"`
	Backfill Backfill `yaml:"backfill"`
	Sync     Sync     `yaml:"sync"`
	// Database is the one database part. Its pool size sizes each job kind's pool alike (ADR-0078,
	// ADR-0119).
	Database   Database `yaml:"database"`
	Credential Keys     `yaml:"credential"`
	// Scanner is the scanner's part, one for every job kind that masks or scans (ADR-0120).
	Scanner Scanner `yaml:"scanner"`
}

// Backfill is backfill's section of the worker's configuration.
type Backfill struct {
	// Concurrency is how many accounts' backfills run at once, which bounds the bodies backfill holds
	// at once (ADR-0119).
	Concurrency int `yaml:"concurrency"`
}

// Sync is delta sync's section of the worker's configuration.
type Sync struct {
	// SyncInterval is how often each account ticks, the sync interval (ADR-0018, ADR-0103).
	SyncInterval Duration `yaml:"sync_interval"`
	// FirstWindow is how far back an account with no cursor is reconciled at its first tick
	// (ADR-0105).
	FirstWindow Duration `yaml:"first_window"`
	// DecisionsPerTick bounds how many waiting messages one tick decides for an account (ADR-0104).
	DecisionsPerTick int `yaml:"decisions_per_tick"`
	// Concurrency is how many accounts tick at once, which bounds the bodies delta sync holds at once
	// (ADR-0119).
	Concurrency int `yaml:"concurrency"`
}

// DefaultWorker is the worker's defaults. The reload interval is the API server's, the sync interval
// ADR-0018's, the first window ADR-0105's, and the bound on the waiting messages a tick decides the
// top of ADR-0018's steady-state tick. The pool size, which sizes each job kind's pool alike, is 4,
// since the driver's default follows the node's processors and not the worker's limit (ADR-0078).
// The scanner's vocabulary and tuning are the ones the application ships.
func DefaultWorker() Worker {
	return Worker{
		Process:        DefaultProcess(),
		Operations:     DefaultOperations(),
		ReloadInterval: minute,
		Accounts:       DefaultAccounts(),
		Backfill:       Backfill{Concurrency: 2},
		Sync:           Sync{SyncInterval: 5 * minute, FirstWindow: 7 * 24 * hour, DecisionsPerTick: 200, Concurrency: 4},
		Database:       DefaultDatabase(4),
		Scanner:        DefaultScanner(),
	}
}

// Check refuses a configuration the worker cannot run with. Every interval, window and bound is
// positive, and every part it has passes its own check.
func (c Worker) Check() error {
	return first(
		c.Process.Check,
		c.Operations.Check,
		c.Database.Check,
		func() error {
			switch {
			case c.ReloadInterval <= 0:
				return errors.New("reload_interval is not positive")
			case c.Backfill.Concurrency <= 0:
				return errors.New("backfill.concurrency " + strconv.Itoa(c.Backfill.Concurrency) + " is not positive")
			case c.Sync.SyncInterval <= 0:
				return errors.New("sync.sync_interval is not positive")
			case c.Sync.FirstWindow <= 0:
				return errors.New("sync.first_window is not positive")
			case c.Sync.DecisionsPerTick <= 0:
				return errors.New("sync.decisions_per_tick " + strconv.Itoa(c.Sync.DecisionsPerTick) + " is not positive")
			case c.Sync.Concurrency <= 0:
				return errors.New("sync.concurrency " + strconv.Itoa(c.Sync.Concurrency) + " is not positive")
			}
			return nil
		},
		c.Accounts.Check,
		c.Credential.Check,
		scanner(c.Scanner),
	)
}

// Migrator is the migrator's configuration, the process's part inline at the top of the tree, the
// database part, and the application user's name.
type Migrator struct {
	Process  `yaml:",inline"`
	Database Database `yaml:"database"`
	// ApplicationUser is the name of the user the API server, the UI and the worker connect as, which
	// every grant of the chain is made to. The project names no database user, so it has no default
	// (ADR-0048, ADR-0139, ADR-0143).
	ApplicationUser string `yaml:"application_user" settings:"required"`
}

// DefaultMigrator is the migrator's defaults on a node with processors processors. The pool size is
// the driver's own default.
func DefaultMigrator(processors int) Migrator {
	return Migrator{Process: DefaultProcess(), Database: DefaultDatabase(DriverPoolSize(processors))}
}

// Check refuses an empty application user's name, which would name no grantee, and every part that
// fails its own check.
func (c Migrator) Check() error {
	return first(
		c.Process.Check,
		func() error {
			if c.ApplicationUser == "" {
				return errors.New("application_user is empty")
			}
			return nil
		},
		c.Database.Check,
	)
}

// first returns the first error any of checks returns, in order.
func first(checks ...func() error) error {
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

// scanner returns the scanner part's check, its refusal naming the part, as every refusal a
// program's check makes names the key at fault.
func scanner(s Scanner) func() error {
	return func() error {
		if err := s.Check(); err != nil {
			return &partError{part: "scanner", err: err}
		}
		return nil
	}
}

// partError is a part's refusal, named by the part's key.
type partError struct {
	part string
	err  error
}

func (e *partError) Error() string { return e.part + ": " + e.err.Error() }

func (e *partError) Unwrap() error { return e.err }
