package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/helmchart"
)

// chartDir is the chart, relative to this package.
const chartDir = "../../packaging/chart"

// TestTheChartRendersAFileTheWorkerStartsWith renders the chart with every key it writes into the worker's file set,
// loads that file through the configuration library as the process does, validates it and builds the scanner from
// it, so a key the chart writes that the worker does not read, or a value of the wrong type, fails here rather than
// at the pod's start (ADR-0052, ADR-0078). The loaded configuration is compared with the values written out below,
// each job kind's role and section included, so a value the chart drops or moves is caught too.
func TestTheChartRendersAFileTheWorkerStartsWith(t *testing.T) {
	objects := helmchart.Render(t, chartDir, "-f", "testdata/chart-values.yaml")
	path := filepath.Join(t.TempDir(), "worker.yaml")
	if err := os.WriteFile(path, []byte(helmchart.ConfigFile(t, objects, "worker.yaml")), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := settings.Load(defaults(), []string{"--config-file=" + path}, nil)
	if err != nil {
		t.Fatalf("the worker refuses the chart's file: %v", err)
	}
	if err := errors.Join(host.ValidateProcess(loaded.Config.Process), validate(loaded.Config)); err != nil {
		t.Fatalf("the worker's validation refuses the chart's file: %v", err)
	}
	if _, err := buildScanner(loaded); err != nil {
		t.Fatalf("the worker refuses the chart's scanner section: %v", err)
	}
	mounted := "/etc/mediated-mailbox/secrets/"
	want := Configuration{
		Process: host.Process{ProbeListen: ":8080", LogLevel: "error"},
		Section: Section{
			ReloadInterval: 90 * time.Second,
			Backfill:       Backfill{Concurrency: 5},
			Sync:           Sync{SyncInterval: 3 * time.Minute, FirstWindow: 48 * time.Hour, DecisionsPerTick: 50, Concurrency: 6},
		},
		Database: Database{
			Host: "db.example", Port: 6432, Name: "mediated_mailbox", SSLMode: "require", SSLRootCert: mounted + "ca/ca.crt",
			Backfill: Role{User: "mailbox_backfill", PasswordFile: mounted + "database/backfill/password", PoolSize: 3},
			Sync:     Role{User: "mailbox_sync", PasswordFile: mounted + "database/sync/password", PoolSize: 2},
		},
		Credential: credentialcore.Config{
			PublicKeyFile:   mounted + "credential-sealing-key-pair/credential.pub",
			PrivateKeyFiles: []string{mounted + "credential-sealing-key-pair/old.key", mounted + "credential-sealing-key-pair/new.key"},
		},
		Scanner: scan.Config{
			Triggers:   map[string][]string{"en": {"code"}, "no": {"kode"}},
			LinkWords:  []string{"token"},
			LinkParams: []string{"code"},
			Window:     6, DenseLength: 20, DenseEntropy: 3.5,
			Weights:   scan.Weights{Entropy: 1, Mix: 1, Length: 1, Proximity: 0.5, Position: 1},
			Threshold: 0.7, SubjectThreshold: 0.65,
		},
	}
	if diff := cmp.Diff(want, loaded.Config, compare.Options); diff != "" {
		t.Errorf("the worker's configuration from the chart's file (-want +got):\n%s", diff)
	}
}

// TestTheChartAdmitsEveryWorkerKeyItDoesNotRender reads the values schema's blocks for the worker and checks that,
// with the keys the chart renders itself, they name exactly the keys of the worker's configuration types: the root,
// each job kind's block under database and each job kind's own section, and the scanner section the chart renders
// into the worker and the API server. A key the worker gains that the schema does not admit, or one the schema admits
// that the worker would refuse, fails here.
func TestTheChartAdmitsEveryWorkerKeyItDoesNotRender(t *testing.T) {
	jobKinds := helmchart.SchemaKeys(t, chartDir, "worker", "jobKinds")
	root := append(helmchart.SchemaKeys(t, chartDir, "worker", "config"), "credential", "database", "probe_listen", "scanner")
	root = append(root, jobKinds...)
	slices.Sort(root)
	for _, c := range []struct {
		name string
		typ  reflect.Type
		got  []string
	}{
		{"the root", reflect.TypeFor[Configuration](), root},
		{"the database section", reflect.TypeFor[Database](), sorted(append([]string{"host", "name", "port", "sslmode", "sslrootcert"}, jobKinds...))},
		{"backfill's block under database", reflect.TypeFor[Role](), roleKeys(t, "backfill")},
		{"delta sync's block under database", reflect.TypeFor[Role](), roleKeys(t, "sync")},
		{"backfill's section", reflect.TypeFor[Backfill](), helmchart.SchemaKeys(t, chartDir, "worker", "jobKinds", "backfill", "config")},
		{"delta sync's section", reflect.TypeFor[Sync](), sorted(append(helmchart.SchemaKeys(t, chartDir, "worker", "jobKinds", "sync", "config"), "sync_interval"))},
		{"the scanner section", reflect.TypeFor[scan.Config](), helmchart.SchemaKeys(t, chartDir, "sharedConfig", "scanner")},
		{"the scanner's weights", reflect.TypeFor[scan.Weights](), helmchart.SchemaKeys(t, chartDir, "sharedConfig", "scanner", "weights")},
	} {
		if diff := cmp.Diff(helmchart.YAMLKeys(c.typ), c.got); diff != "" {
			t.Errorf("%s (-configuration type +schema and chart):\n%s", c.name, diff)
		}
	}
}

// roleKeys returns the worker's keys under database.<kind> that the chart renders from the job kind's database
// block in the values schema, its camelCase keys turned into the worker's names, with the password file the chart
// always renders, sorted. A schema key with no rendering written out here fails the test.
func roleKeys(t *testing.T, kind string) []string {
	t.Helper()
	renders := map[string]string{"user": "user", "poolSize": "pool_size"}
	keys := []string{"password_file"}
	for _, k := range helmchart.SchemaKeys(t, chartDir, "worker", "jobKinds", kind, "database") {
		name, ok := renders[k]
		if !ok {
			t.Errorf("worker.jobKinds.%s.database.%s has no rendering this test knows", kind, k)
			continue
		}
		keys = append(keys, name)
	}
	return sorted(keys)
}

func sorted(keys []string) []string {
	slices.Sort(keys)
	return keys
}
