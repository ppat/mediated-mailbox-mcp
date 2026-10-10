package dbmetrics_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/dbmetrics"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The pool's six series are gathered from its statistics. A pool that has served nothing reads its
// ceiling and zero for the rest, and opens no connection to be read.
func TestThePoolIsReadWhenGathered(t *testing.T) {
	config, err := pgxpool.ParseConfig("host=db.invalid")
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 7
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	reg := prometheus.NewRegistry()
	if err := dbmetrics.RegisterPool(reg, pool); err != nil {
		t.Fatal(err)
	}
	want := map[string][]sample{
		"mediated_mailbox_db_pool_acquires_total":             {{Value: 0}},
		"mediated_mailbox_db_pool_waited_acquires_total":      {{Value: 0}},
		"mediated_mailbox_db_pool_acquire_wait_seconds_total": {{Value: 0}},
		"mediated_mailbox_db_pool_canceled_acquires_total":    {{Value: 0}},
		"mediated_mailbox_db_pool_acquired_connections":       {{Value: 0}},
		"mediated_mailbox_db_pool_max_connections":            {{Value: 7}},
	}
	got := map[string][]sample{}
	for name := range want {
		got[name] = gathered(t, reg, name)
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the pool's series (-want +got):\n%s", diff)
	}
}
