package process_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// What an endpoint, a job kind or a task is built from is fixed by the spec's type, the build's
// context, the capabilities and the program's configuration, and what each Build returns is what it
// builds and an error. Init returns only an error, so it can refuse a start and build nothing
// (ADR-0142).
func TestASpecFixesWhatEachPartIsBuiltFrom(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"endpointextra", "as func(ctx context.Context, p process.Capabilities, c config.API) (http.Handler, error) value in struct literal"},
		{"jobextra", "as func(ctx context.Context, p process.Capabilities, c config.Worker) (schedule.Plan, error) value in struct literal"},
		{"taskextra", "as func(ctx context.Context, p process.Capabilities, c config.Migrator) (func(context.Context) error, error) value in struct literal"},
		{"initbuilds", "as func(args []string, environ []string, c config.API) error value in struct literal"},
	} {
		t.Run(c.name, func(t *testing.T) {
			mustnotcompile.Require(t, "./testdata/mustnotcompile/"+c.name, c.want)
		})
	}
}
