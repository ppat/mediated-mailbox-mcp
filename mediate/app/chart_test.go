package app

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/helmchart"
)

// chartDir is the chart, relative to this package.
const chartDir = "../../packaging/chart"

// TestTheChartRendersAFileTheMediatorStartsWith renders the chart with every key it writes into the mediator's
// file set, loads that file through the configuration library as the process does, and validates it, so a key the
// chart writes that the mediator does not read, or a value of the wrong type, fails here rather than at the pod's
// start (ADR-0052, ADR-0078). The loaded configuration is compared with the values written out below, so a value the
// chart drops or moves is caught too.
func TestTheChartRendersAFileTheMediatorStartsWith(t *testing.T) {
	objects := helmchart.Render(t, chartDir, "-f", "testdata/chart-values.yaml")
	path := filepath.Join(t.TempDir(), "mediator.yaml")
	if err := os.WriteFile(path, []byte(helmchart.ConfigFile(t, objects, "mediator.yaml")), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := settings.Load(defaults(), []string{"--config-file=" + path}, nil)
	if err != nil {
		t.Fatalf("the mediator refuses the chart's file: %v", err)
	}
	if err := validate(loaded.Config); err != nil {
		t.Fatalf("the mediator's validation refuses the chart's file: %v", err)
	}
	if _, err := scan.New(loaded.Config.Scanner, loaded.Revisions[scannerSection]); err != nil {
		t.Fatalf("the scanner refuses the chart's scanner section: %v", err)
	}
	mounted := "/etc/mediated-mailbox/secrets/"
	want := Configuration{
		Listen:                ":8443",
		ProbeListen:           ":8080",
		TLSCert:               mounted + "tls/tls.crt",
		TLSKey:                mounted + "tls/tls.key",
		TokenFile:             mounted + "bearer-token",
		AccountReloadInterval: 2 * time.Minute,
		Database: dbconnectcore.Config{
			Host: "db.example", Port: 6432, Name: "mediated_mailbox", User: "mailbox_mediator", SSLMode: "verify-ca",
			SSLRootCert: mounted + "ca/ca.crt", PasswordFile: mounted + "database/password", PoolSize: 9,
		},
		Credential: credentialcore.Config{
			PublicKeyFile:   mounted + "credential-sealing-key-pair/credential.pub",
			PrivateKeyFiles: []string{mounted + "credential-sealing-key-pair/old.key", mounted + "credential-sealing-key-pair/new.key"},
		},
		Scanner: scan.Config{
			Triggers:   map[string][]string{"en": {"code", "log in"}, "no": {"kode", "yes"}},
			LinkWords:  []string{"token"},
			LinkParams: []string{"code"},
			Window:     6, DenseLength: 20, DenseEntropy: 3.5,
			Weights:   scan.Weights{Entropy: 1, Mix: 0.5, Length: 1, Proximity: 1, Position: 1},
			Threshold: 0.7, SubjectThreshold: 0.65,
		},
		ProviderTimeout: 45 * time.Second,
		LogLevel:        "debug",
	}
	if diff := cmp.Diff(want, loaded.Config, compare.Options); diff != "" {
		t.Errorf("the mediator's configuration from the chart's file (-want +got):\n%s", diff)
	}
}

// TestTheChartAdmitsEveryMediatorKeyItDoesNotRender reads the values schema's mediator.config block and checks that,
// with the keys the chart renders itself, it names exactly the keys of the mediator's configuration type. A key the
// mediator gains that the schema does not admit, or one the schema admits that the mediator would refuse, fails here.
func TestTheChartAdmitsEveryMediatorKeyItDoesNotRender(t *testing.T) {
	chartOwned := []string{"credential", "database", "listen", "probe_listen", "scanner", "tls_cert", "tls_key", "token_file"}
	got := append(helmchart.SchemaKeys(t, chartDir, "mediator", "config"), chartOwned...)
	slices.Sort(got)
	if diff := cmp.Diff(helmchart.YAMLKeys(reflect.TypeFor[Configuration]()), got); diff != "" {
		t.Errorf("the mediator's keys (-configuration type +schema and chart):\n%s", diff)
	}
}
