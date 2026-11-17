package config_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

const (
	second = config.Duration(1_000_000_000)
	minute = 60 * second
	hour   = 60 * minute
)

// The API server's defaults are the process's, its two listeners and the info level, a reload each
// minute, the port, the TLS mode that fails closed, the driver's own pool size on the processors
// given, the scanner part the application ships and a provider timeout of thirty seconds. Nothing
// names a database user, a token file or a key file (ADR-0143).
func TestTheAPIDefaults(t *testing.T) {
	want := config.API{
		Process:         config.Process{LogLevel: "info"},
		Operations:      config.Operations{ProbeListen: ":8080"},
		Serving:         config.Serving{Listen: ":8443"},
		Accounts:        config.Accounts{AccountReloadInterval: minute},
		ProviderTimeout: 30 * second,
		Database:        config.Database{Port: 5432, SSLMode: "verify-full", PoolSize: 6},
		Scanner:         config.DefaultScanner(),
	}
	if diff := cmp.Diff(want, config.DefaultAPI(6), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	if got := config.DefaultAPI(2).Database.PoolSize; got != 4 {
		t.Errorf("on two processors the pool size is %d, want 4", got)
	}
}

func validAPI() config.API {
	c := config.DefaultAPI(4)
	c.TokenFile = "/run/token"
	c.Database.Host, c.Database.Name, c.Database.User, c.Database.PasswordFile = "db", "mailbox", "mediated_mailbox_app", "/run/password"
	c.Credential = validKeys()
	return c
}

// A configuration the API server cannot serve with is refused, each refusal named by its key, and
// the first refusal in the order the parts are checked is the one reported. No account reaches the
// API server from its configuration (ADR-0080).
func TestTheAPIConfigurationIsChecked(t *testing.T) {
	if err := validAPI().Check(); err != nil {
		t.Fatalf("a valid configuration was refused: %v", err)
	}
	refused(t, validAPI, config.API.Check, []refusal[config.API]{
		{"a level that names none", func(c *config.API) { c.LogLevel = "loud" }, `log_level "loud" is not one of debug, info, warn and error`},
		{"an empty probe_listen", func(c *config.API) { c.ProbeListen = "" }, "probe_listen is empty"},
		{"one TLS file", func(c *config.API) { c.TLSCert = "/tls/cert" }, "tls_cert is set and tls_key is not, and the process serves TLS with both or plain HTTP with neither"},
		{"a blank token file path", func(c *config.API) { c.TokenFile = " " }, "token_file is empty"},
		{"a reload interval of zero", func(c *config.API) { c.AccountReloadInterval = 0 }, "account_reload_interval is not positive"},
		{"a negative reload interval", func(c *config.API) { c.AccountReloadInterval = -second }, "account_reload_interval is not positive"},
		{"a provider timeout of zero", func(c *config.API) { c.ProviderTimeout = 0 }, "provider_timeout is not positive and at most 5m0s"},
		{"a provider timeout above five minutes", func(c *config.API) { c.ProviderTimeout = 5*minute + second }, "provider_timeout is not positive and at most 5m0s"},
		{"an empty database host", func(c *config.API) { c.Database.Host = "" }, "database.host is empty"},
		{"an empty database user", func(c *config.API) { c.Database.User = "" }, "database.user is empty"},
		{"a database pool size of zero", func(c *config.API) { c.Database.PoolSize = 0 }, "database.pool_size 0 is not positive"},
		{"no private key", func(c *config.API) { c.Credential.PrivateKeyFiles = nil }, "credential.private_key_files names no file"},
		{"a scanner with nothing to match", func(c *config.API) { c.Scanner.Triggers = nil }, "scanner: scan: no trigger word is configured"},
		{"the token file before the database", func(c *config.API) { c.TokenFile, c.Database.Host = "", "" }, "token_file is empty"},
		{"the serving part before the token file", func(c *config.API) { c.Listen, c.TokenFile = "", "" }, "listen is empty"},
		{"the credential part before the scanner", func(c *config.API) { c.Credential.PublicKeyFile, c.Scanner.LinkWords = "", nil }, "credential.public_key_file is empty"},
	})
	if err := (config.API{Process: config.DefaultProcess()}).Check(); err == nil {
		t.Error("the zero API configuration with a level was accepted")
	}
	five := validAPI()
	five.ProviderTimeout = 5 * minute
	if err := five.Check(); err != nil {
		t.Errorf("a provider timeout of five minutes was refused: %v", err)
	}
}

// The UI's defaults are the process's, its two listeners and the info level, the port, the TLS mode
// that fails closed, the driver's own pool size on the processors given, and the records' intervals
// (docs/UI.md section 18.1). Nothing names a database user (ADR-0143).
func TestTheUIDefaults(t *testing.T) {
	want := config.UI{
		Process:            config.Process{LogLevel: "info"},
		Operations:         config.Operations{ProbeListen: ":8080"},
		Serving:            config.Serving{Listen: ":8443"},
		SyncInterval:       5 * minute,
		HeuristicsInterval: 24 * hour,
		StreamInterval:     2 * second,
		DefaultTheme:       "system",
		StreamReconnectMax: 30 * second,
		StreamPollInterval: 5 * second,
		// The worth-a-look thresholds are docs/UI.md section 8.1's starting values.
		AttentionBacklogShare: 5,
		AttentionMaskCount:    20,
		AttentionServeFactor:  2,
		AttentionGapDays:      7,
		ConsentRedirect:       "http://127.0.0.1:47823/",
		OperatorName:          "operator",
		Database:              config.Database{Port: 5432, SSLMode: "verify-full", PoolSize: 8},
	}
	if diff := cmp.Diff(want, config.DefaultUI(8), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
}

func validUI() config.UI {
	c := config.DefaultUI(4)
	c.Database = validDatabase()
	c.Credential = validKeys()
	return c
}

// A configuration the UI cannot serve with is refused, each refusal named by its key, in the order
// its parts are checked. A worth-a-look threshold of 0 disables its rule, so every threshold at 0 is
// valid (docs/UI.md section 18.1).
func TestTheUIConfigurationIsChecked(t *testing.T) {
	if err := validUI().Check(); err != nil {
		t.Fatalf("a valid configuration was refused: %v", err)
	}
	disabled := validUI()
	disabled.AttentionBacklogShare, disabled.AttentionMaskCount, disabled.AttentionServeFactor, disabled.AttentionGapDays = 0, 0, 0, 0
	if err := disabled.Check(); err != nil {
		t.Fatalf("every worth-a-look rule disabled was refused: %v", err)
	}
	least := validUI()
	least.SyncInterval, least.HeuristicsInterval, least.StreamInterval = 1, 1, 1
	least.StreamReconnectMax, least.StreamPollInterval = 1_000_000, 1_000_000
	if err := least.Check(); err != nil {
		t.Fatalf("the least intervals were refused: %v", err)
	}
	refused(t, validUI, config.UI.Check, []refusal[config.UI]{
		{"an empty listen", func(c *config.UI) { c.Listen = "" }, "listen is empty"},
		{"an empty probe_listen", func(c *config.UI) { c.ProbeListen = " " }, "probe_listen is empty"},
		{"a pool size of zero", func(c *config.UI) { c.Database.PoolSize = 0 }, "database.pool_size 0 is not positive"},
		{"an empty database user", func(c *config.UI) { c.Database.User = "" }, "database.user is empty"},
		{"a zero sync interval", func(c *config.UI) { c.SyncInterval = 0 }, "sync_interval is not positive"},
		{"a negative heuristics interval", func(c *config.UI) { c.HeuristicsInterval = -1 }, "heuristics_interval is not positive"},
		{"a zero stream interval", func(c *config.UI) { c.StreamInterval = 0 }, "stream_interval is not positive"},
		{"an unknown default theme", func(c *config.UI) { c.DefaultTheme = "dim" }, "default_theme is not system, dark or light"},
		{"a reconnection ceiling under a millisecond", func(c *config.UI) { c.StreamReconnectMax = 999_999 }, "stream_reconnect_max is under a millisecond"},
		{"a polling interval under a millisecond", func(c *config.UI) { c.StreamPollInterval = 0 }, "stream_poll_interval is under a millisecond"},
		{"a negative backlog share", func(c *config.UI) { c.AttentionBacklogShare = -0.5 }, "attention_backlog_share is negative"},
		{"a negative masking count", func(c *config.UI) { c.AttentionMaskCount = -1 }, "attention_mask_count is negative"},
		{"a negative serve factor", func(c *config.UI) { c.AttentionServeFactor = -2 }, "attention_serve_factor is negative"},
		{"a negative gap window", func(c *config.UI) { c.AttentionGapDays = -7 }, "attention_gap_days is negative"},
		{"a consent redirect by name", func(c *config.UI) { c.ConsentRedirect = "http://localhost:47823/" }, "consent_redirect is not an http address on a loopback IP literal with an explicit port"},
		{"an empty list of private keys", func(c *config.UI) { c.Credential.PrivateKeyFiles = []string{} }, "credential.private_key_files names no file"},
		{"the database before the UI's own", func(c *config.UI) { c.Database.Host, c.SyncInterval = "", 0 }, "database.host is empty"},
		{"the UI's own before the credential part", func(c *config.UI) { c.DefaultTheme, c.Credential.PublicKeyFile = "", "" }, "default_theme is not system, dark or light"},
	})
}

// The consent redirect is an http address on a loopback IP literal with an explicit port, and a
// value of any other shape is refused, so a consent never redirects anywhere something could listen
// for its code (docs/UI.md section 18.1, VERIFICATIONS, the consent redirect row).
func TestTheConsentRedirectIsALoopbackAddressWithAPort(t *testing.T) {
	for _, ok := range []string{"http://127.0.0.1:47823/", "http://127.0.0.1:47823", "http://127.4.5.6:1/", "http://[::1]:65535/"} {
		c := validUI()
		c.ConsentRedirect = ok
		if err := c.Check(); err != nil {
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
		c := validUI()
		c.ConsentRedirect = bad
		err := c.Check()
		if err == nil || err.Error() != "consent_redirect is not an http address on a loopback IP literal with an explicit port" {
			t.Errorf("%q gave %v", bad, err)
		}
	}
}

// The worker's defaults are the process's, its probe listener and the info level, the port, a pool
// of four for each job kind, the TLS mode that fails closed, a listing and a reload of the Session
// every minute, ADR-0018's five-minute sync interval, a first window of seven days, two hundred
// decisions a tick, two accounts backfilled at once, four ticking at once and the scanner part the
// application ships. Nothing names a database user (ADR-0078, ADR-0103, ADR-0104, ADR-0105, ADR-0119, ADR-0143).
func TestTheWorkerDefaults(t *testing.T) {
	want := config.Worker{
		Process:        config.Process{LogLevel: "info"},
		Operations:     config.Operations{ProbeListen: ":8080"},
		ReloadInterval: minute,
		Accounts:       config.Accounts{AccountReloadInterval: minute},
		Backfill:       config.Backfill{Concurrency: 2},
		Sync:           config.Sync{SyncInterval: 5 * minute, FirstWindow: 7 * 24 * hour, DecisionsPerTick: 200, Concurrency: 4},
		Database:       config.Database{Port: 5432, SSLMode: "verify-full", PoolSize: 4},
		Scanner:        config.DefaultScanner(),
	}
	if diff := cmp.Diff(want, config.DefaultWorker(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
}

func validWorker() config.Worker {
	c := config.DefaultWorker()
	c.Database = validDatabase()
	c.Credential = validKeys()
	return c
}

// A configuration the worker cannot run with is refused, each refusal named by its key, in the order
// its parts are checked. Every interval, window and bound is positive.
func TestTheWorkerConfigurationIsChecked(t *testing.T) {
	if err := validWorker().Check(); err != nil {
		t.Fatalf("a valid configuration was refused: %v", err)
	}
	refused(t, validWorker, config.Worker.Check, []refusal[config.Worker]{
		{"an empty probe_listen", func(c *config.Worker) { c.ProbeListen = "" }, "probe_listen is empty"},
		{"an empty host", func(c *config.Worker) { c.Database.Host = "" }, "database.host is empty"},
		{"an empty user", func(c *config.Worker) { c.Database.User = "" }, "database.user is empty"},
		{"a pool size of zero", func(c *config.Worker) { c.Database.PoolSize = 0 }, "database.pool_size 0 is not positive"},
		{"a reload interval of zero", func(c *config.Worker) { c.ReloadInterval = 0 }, "reload_interval is not positive"},
		{"no backfill concurrency", func(c *config.Worker) { c.Backfill.Concurrency = 0 }, "backfill.concurrency 0 is not positive"},
		{"a sync interval of zero", func(c *config.Worker) { c.Sync.SyncInterval = 0 }, "sync.sync_interval is not positive"},
		{"a negative first window", func(c *config.Worker) { c.Sync.FirstWindow = -hour }, "sync.first_window is not positive"},
		{"no decisions a tick", func(c *config.Worker) { c.Sync.DecisionsPerTick = 0 }, "sync.decisions_per_tick 0 is not positive"},
		{"no sync concurrency", func(c *config.Worker) { c.Sync.Concurrency = -1 }, "sync.concurrency -1 is not positive"},
		{"an account reload interval of zero", func(c *config.Worker) { c.AccountReloadInterval = 0 }, "account_reload_interval is not positive"},
		{"an empty public key path", func(c *config.Worker) { c.Credential.PublicKeyFile = "" }, "credential.public_key_file is empty"},
		{"a scanner threshold above 1", func(c *config.Worker) { c.Scanner.Threshold = 1.5 }, "scanner: scan: each threshold must be above 0 and at most 1"},
		{"the database before the worker's own", func(c *config.Worker) { c.Database.Name, c.ReloadInterval = "", 0 }, "database.name is empty"},
		{"the worker's own before the credential part", func(c *config.Worker) { c.Sync.SyncInterval, c.Credential.PrivateKeyFiles = 0, nil }, "sync.sync_interval is not positive"},
	})
}

// The migrator's defaults are the process's and the database part's, with the driver's own pool size
// on the processors given. Nothing names the migrator's user or the application user (ADR-0143).
func TestTheMigratorDefaults(t *testing.T) {
	want := config.Migrator{Process: config.Process{LogLevel: "info"}, Database: config.Database{Port: 5432, SSLMode: "verify-full", PoolSize: 5}}
	if diff := cmp.Diff(want, config.DefaultMigrator(5), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
}

// A configuration the migrator cannot connect with is refused, and an empty application user's name,
// which would name no grantee, before the database part.
func TestTheMigratorConfigurationIsChecked(t *testing.T) {
	valid := func() config.Migrator {
		c := config.DefaultMigrator(4)
		c.Database, c.ApplicationUser = validDatabase(), "mediated_mailbox_app"
		return c
	}
	if err := valid().Check(); err != nil {
		t.Fatalf("a valid configuration was refused: %v", err)
	}
	refused(t, valid, config.Migrator.Check, []refusal[config.Migrator]{
		{"a level that names none", func(c *config.Migrator) { c.LogLevel = "Warn" }, `log_level "Warn" is not one of debug, info, warn and error`},
		{"an empty application user", func(c *config.Migrator) { c.ApplicationUser = "" }, "application_user is empty"},
		{"an empty database user", func(c *config.Migrator) { c.Database.User = "" }, "database.user is empty"},
		{"the application user before the database", func(c *config.Migrator) { c.ApplicationUser, c.Database.Host = "", "" }, "application_user is empty"},
	})
}

// The scanner part's defaults pass its check, and a part with several problems reports every one,
// joined, so an operator fixes them in one round. A program's check names the part a scanner
// refusal comes from and keeps the refusal underneath.
func TestTheScannerPartIsChecked(t *testing.T) {
	if err := config.DefaultScanner().Check(); err != nil {
		t.Fatalf("the default was refused: %v", err)
	}
	s := config.DefaultScanner()
	s.Window, s.SubjectThreshold = 0, 0.9
	err := s.Check()
	want := "scan: the window, the dense length and the dense entropy must be positive\n" +
		"scan: the subject threshold is above the body threshold, but masking a subject is tuned for recall"
	if err == nil || err.Error() != want {
		t.Fatalf("Check returned %v, want %q", err, want)
	}
	c := validWorker()
	c.Scanner = s
	wrapped := c.Check()
	if wrapped == nil || wrapped.Error() != "scanner: "+want || errors.Unwrap(wrapped) == nil || errors.Unwrap(wrapped).Error() != want {
		t.Errorf("the worker's check returned %v, want the scanner's refusal named by its part", wrapped)
	}
}

// Each program's type is pinned field by field, so a new value, one meant to hold a secret
// included, is a visible change, and so are its own sections (ADR-0078).
func TestTheProgramTypesArePinned(t *testing.T) {
	mustnotcompile.RequireFields(t, pkg, "API",
		"Process Process",
		"Operations Operations",
		"Serving Serving",
		"TokenFile string",
		"Accounts Accounts",
		"ProviderTimeout Duration",
		"Database Database",
		"Credential Keys",
		"Scanner Scanner",
	)
	mustnotcompile.RequireFields(t, pkg, "UI",
		"Process Process", "Operations Operations", "Serving Serving",
		"SyncInterval Duration", "HeuristicsInterval Duration", "StreamInterval Duration",
		"DefaultTheme string", "StreamReconnectMax Duration", "StreamPollInterval Duration",
		"AttentionBacklogShare float64", "AttentionMaskCount int64", "AttentionServeFactor float64", "AttentionGapDays int64",
		"TokenKeyFile string", "ConsentRedirect string", "IdentityHeader string", "OperatorName string",
		"Database Database", "Credential Keys",
	)
	mustnotcompile.RequireFields(t, pkg, "Worker",
		"Process Process",
		"Operations Operations",
		"ReloadInterval Duration",
		"Accounts Accounts",
		"Backfill Backfill",
		"Sync Sync",
		"Database Database",
		"Credential Keys",
		"Scanner Scanner",
	)
	mustnotcompile.RequireFields(t, pkg, "Backfill", "Concurrency int")
	mustnotcompile.RequireFields(t, pkg, "Sync",
		"SyncInterval Duration",
		"FirstWindow Duration",
		"DecisionsPerTick int",
		"Concurrency int",
	)
	mustnotcompile.RequireFields(t, pkg, "Migrator",
		"Process Process",
		"Database Database",
		"ApplicationUser string",
	)
}
