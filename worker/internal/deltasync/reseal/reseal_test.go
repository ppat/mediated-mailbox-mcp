package reseal_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/reseal"
)

// gathered returns every scan series reg holds, keyed on its name, its label's name and its value.
func gathered(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			out[f.GetName()+" "+m.GetLabel()[0].GetName()+"="+m.GetLabel()[0].GetValue()] = m.GetGauge().GetValue()
		}
	}
	return out
}

// Each scan sets one series for every account and every OAuth client it holds, 1 for a value on an
// old key or unopenable and 0 for one on the current key or with nothing to seal, and removes the
// series of an account or client a later scan no longer holds, so the series are exactly what the
// last load listed. A client's series is labelled by its name, so two clients of one provider keep two
// series (ADR-0092, ADR-0103, ADR-0106).
func TestTheSeriesFollowTheLastScan(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := reseal.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	m.Set(accountload.Scan{Accounts: map[string]bool{"personal": true, "work": false}, Clients: map[string]bool{"household": true, "employer": false}})
	first := map[string]float64{
		"mediated_mailbox_credential_on_old_key account=personal": 1, "mediated_mailbox_credential_on_old_key account=work": 0,
		"mediated_mailbox_client_secret_on_old_key client=household": 1, "mediated_mailbox_client_secret_on_old_key client=employer": 0,
	}
	if diff := cmp.Diff(first, gathered(t, reg), compare.Options); diff != "" {
		t.Errorf("the series after the first scan (-want +got):\n%s", diff)
	}

	m.Set(accountload.Scan{Accounts: map[string]bool{"personal": false}, Clients: map[string]bool{"household": false}})

	second := map[string]float64{
		"mediated_mailbox_credential_on_old_key account=personal": 0, "mediated_mailbox_client_secret_on_old_key client=household": 0,
	}
	if diff := cmp.Diff(second, gathered(t, reg), compare.Options); diff != "" {
		t.Errorf("the series after a scan listing fewer (-want +got):\n%s", diff)
	}
}
