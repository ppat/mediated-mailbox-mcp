package reseal_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/sync/internal/reseal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// gathered returns every scan series reg holds, keyed on its name and label value.
func gathered(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			out[f.GetName()+" "+m.GetLabel()[0].GetValue()] = m.GetGauge().GetValue()
		}
	}
	return out
}

// Each scan sets one series for every account and every OAuth client it holds, 1 for a value on an
// old key or unopenable and 0 for one on the current key or with nothing to seal, and removes the
// series of an account or client a later scan no longer holds, so the series are exactly what the
// last load listed (ADR-0092, ADR-0103).
func TestTheSeriesFollowTheLastScan(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := reseal.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	m.Set(accountload.Scan{Accounts: map[string]bool{"personal": true, "work": false}, Clients: map[string]bool{"gmail": true, "other": false}})
	first := map[string]float64{
		"mediated_mailbox_sync_credential_on_old_key personal": 1, "mediated_mailbox_sync_credential_on_old_key work": 0,
		"mediated_mailbox_sync_client_secret_on_old_key gmail": 1, "mediated_mailbox_sync_client_secret_on_old_key other": 0,
	}
	if diff := cmp.Diff(first, gathered(t, reg), compare.Options); diff != "" {
		t.Errorf("the series after the first scan (-want +got):\n%s", diff)
	}

	m.Set(accountload.Scan{Accounts: map[string]bool{"personal": false}, Clients: map[string]bool{"gmail": false}})

	second := map[string]float64{
		"mediated_mailbox_sync_credential_on_old_key personal": 0, "mediated_mailbox_sync_client_secret_on_old_key gmail": 0,
	}
	if diff := cmp.Diff(second, gathered(t, reg), compare.Options); diff != "" {
		t.Errorf("the series after a scan listing fewer (-want +got):\n%s", diff)
	}
}
