package app

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/helmchart"
)

// chartDir is the chart, relative to this package.
const chartDir = "../../packaging/chart"

// TestTheChartSetsAConfigurationTheMigratorStartsWith renders the chart with every value it sets in the migration
// Job's environment, takes that environment as the migrator's, loads it through the configuration library as the
// migrator does, and validates it. The library refuses a variable under the prefix that names no key, so a name the
// chart misspells, or a value of the wrong type, fails here rather than at the hook's start (ADR-0052, ADR-0078,
// ADR-0129). The loaded configuration is compared with the values written out below, so a value the chart drops or
// moves is caught too.
func TestTheChartSetsAConfigurationTheMigratorStartsWith(t *testing.T) {
	var environ []string
	for _, o := range helmchart.Render(t, chartDir, "-f", "testdata/chart-values.yaml") {
		if o["kind"] != "Job" {
			continue
		}
		var job struct {
			Spec struct {
				Template struct {
					Spec struct {
						Containers []struct {
							Env []struct{ Name, Value string }
						}
					}
				}
			}
		}
		text, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(text, &job); err != nil {
			t.Fatal(err)
		}
		for _, c := range job.Spec.Template.Spec.Containers {
			for _, e := range c.Env {
				environ = append(environ, e.Name+"="+e.Value)
			}
		}
	}
	if len(environ) == 0 {
		t.Fatal("the chart renders no migration Job, or one with no environment")
	}
	loaded, err := settings.Load(defaults(), nil, environ)
	if err != nil {
		t.Fatalf("the migrator refuses the Job's environment: %v", err)
	}
	if err := errors.Join(host.ValidateProcess(loaded.Config.Process), validate(loaded.Config)); err != nil {
		t.Fatalf("the migrator's validation refuses the Job's configuration: %v", err)
	}
	mounted := "/etc/mediated-mailbox/secrets/"
	want := Configuration{
		Process: host.Process{LogLevel: "debug"},
		Database: dbconnectcore.Config{
			Host: "db.example", Port: 6432, Name: "mediated_mailbox", User: "mailbox_migrator", SSLMode: "verify-ca",
			SSLRootCert: mounted + "ca/ca.crt", PasswordFile: mounted + "database/password", PoolSize: defaults().Database.PoolSize,
		},
	}
	if diff := cmp.Diff(want, loaded.Config, compare.Options); diff != "" {
		t.Errorf("the migrator's configuration from the Job's environment (-want +got):\n%s", diff)
	}
}

// TestTheChartAdmitsEveryMigratorKeyItDoesNotRender reads the values schema's migration.config block and checks that,
// with the keys the chart sets itself, it names exactly the keys of the migrator's configuration type. A key the
// migrator gains that the schema does not admit, or one the schema admits that the migrator would refuse, fails here.
func TestTheChartAdmitsEveryMigratorKeyItDoesNotRender(t *testing.T) {
	chartOwned := []string{"database"}
	got := append(helmchart.SchemaKeys(t, chartDir, "migration", "config"), chartOwned...)
	slices.Sort(got)
	if diff := cmp.Diff(helmchart.YAMLKeys(reflect.TypeFor[Configuration]()), got); diff != "" {
		t.Errorf("the migrator's keys (-configuration type +schema and chart):\n%s", diff)
	}
}
