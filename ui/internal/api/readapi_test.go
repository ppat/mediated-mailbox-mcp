package api_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/policymanagement"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/reads"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	uiserver "github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/setup"
)

// readAPI mounts the routes of reads, setup and policymanagement over db behind checks, as New mounts
// them, and fills in each package's options what New fills from the pool, the checks and the others,
// the registry, the account check, the attempt's cipher and the count of the base rules. A test sets
// the rest, which New reads from the configuration, and what no configuration carries, a fixed clock
// and the provider fake's consents. A setup's or policy management's clock left nil is the reads'
// clock.
func readAPI(t *testing.T, db api.Database, checks *uiserver.Checks, r reads.Options, s setup.Options, p policymanagement.Options) *api.Endpoint {
	t.Helper()
	datasets := registry.Datasets(lookups)
	r.Database, r.Datasets, r.Admit, r.Lookups = db, datasets, api.Admission(db), lookups
	if r.Metrics == nil {
		r.Metrics = prometheus.NewRegistry()
	}
	s.Database, s.AttemptCipher, s.BaseRules = db, checks.AttemptCipher(), policymanagement.BaseRuleCount
	p.Database, p.Lookups = db, lookups
	if s.Clock == nil {
		s.Clock = r.Clock
	}
	if p.Clock == nil {
		p.Clock = r.Clock
	}
	readRoutes, err := reads.Routes(r)
	if err != nil {
		t.Fatal(err)
	}
	e, err := api.Mount(db, checks, datasets, readRoutes, setup.Routes(s), policymanagement.Routes(p))
	if err != nil {
		t.Fatal(err)
	}
	return e
}
