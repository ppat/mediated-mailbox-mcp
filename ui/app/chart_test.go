package app

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/helmchart"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/serving"
)

// chartDir is the chart, relative to this package.
const chartDir = "../../packaging/chart"

// TestTheChartRendersAFileTheUIStartsWith renders the chart with every key it writes into the UI's file set, loads
// that file through the configuration library as the process does, and validates it as the UI's start does, so a key
// the chart writes that the UI does not read, or a value of the wrong type, fails here rather than at the pod's start
// (ADR-0052, ADR-0078). The loaded configuration is compared with the values written out below, so a value the chart
// drops or moves is caught too.
func TestTheChartRendersAFileTheUIStartsWith(t *testing.T) {
	objects := helmchart.Render(t, chartDir, "-f", "testdata/chart-values.yaml")
	path := filepath.Join(t.TempDir(), "ui.yaml")
	if err := os.WriteFile(path, []byte(helmchart.ConfigFile(t, objects, "ui.yaml")), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := settings.Load(defaults(), []string{"--config-file=" + path}, nil)
	if err != nil {
		t.Fatalf("the UI refuses the chart's file: %v", err)
	}
	c := loaded.Config
	if err := dbconnectcore.Validate(c.Database); err != nil {
		t.Fatalf("the UI's validation refuses the chart's database section: %v", err)
	}
	if err := serving.Validate(servingConfig(c)); err != nil {
		t.Fatalf("the UI's validation refuses the chart's file: %v", err)
	}
	if err := credentialcore.Validate(c.Credential); err != nil {
		t.Fatalf("the UI's validation refuses the chart's credential section: %v", err)
	}
	mounted := "/etc/mediated-mailbox/secrets/"
	want := Configuration{
		Database: dbconnectcore.Config{
			Host: "db.example", Port: 6432, Name: "mediated_mailbox", User: "mailbox_ui", SSLMode: "verify-ca",
			SSLRootCert: mounted + "ca/ca.crt", PasswordFile: mounted + "database/password", PoolSize: 11,
		},
		Listen:                ":8443",
		ProbeListen:           ":8080",
		TLSCert:               mounted + "tls/tls.crt",
		TLSKey:                mounted + "tls/tls.key",
		SyncInterval:          3 * time.Minute,
		HeuristicsInterval:    12 * time.Hour,
		StreamInterval:        3 * time.Second,
		DefaultTheme:          "dark",
		StreamReconnectMax:    20 * time.Second,
		StreamPollInterval:    4 * time.Second,
		AttentionBacklogShare: 4,
		AttentionMaskCount:    10,
		AttentionServeFactor:  3,
		AttentionGapDays:      5,
		Credential: credentialcore.Config{
			PublicKeyFile:   mounted + "credential-sealing-key-pair/credential.pub",
			PrivateKeyFiles: []string{mounted + "credential-sealing-key-pair/old.key", mounted + "credential-sealing-key-pair/new.key"},
		},
		TokenKeyFile:    mounted + "request-token-key",
		ConsentRedirect: "http://127.0.0.1:50000/",
		IdentityHeader:  "X-Forwarded-User",
		OperatorName:    "owner",
		LogLevel:        "debug",
	}
	if diff := cmp.Diff(want, c, compare.Options); diff != "" {
		t.Errorf("the UI's configuration from the chart's file (-want +got):\n%s", diff)
	}
}

// TestTheChartAdmitsEveryUIKeyItDoesNotRender reads the values schema's ui.config block and checks that, with the
// keys the chart renders itself, it names exactly the keys of the UI's configuration type. A key the UI gains that the
// schema does not admit, or one the schema admits that the UI would refuse, fails here.
func TestTheChartAdmitsEveryUIKeyItDoesNotRender(t *testing.T) {
	chartOwned := []string{"credential", "database", "listen", "probe_listen", "sync_interval", "tls_cert", "tls_key", "token_key_file"}
	got := append(helmchart.SchemaKeys(t, chartDir, "ui", "config"), chartOwned...)
	slices.Sort(got)
	if diff := cmp.Diff(helmchart.YAMLKeys(reflect.TypeFor[Configuration]()), got); diff != "" {
		t.Errorf("the UI's keys (-configuration type +schema and chart):\n%s", diff)
	}
}
