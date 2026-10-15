package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/assembly"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
)

// A job kind whose reloads find no accounts loaded never reports itself reloaded, so a worker that
// cannot read its accounts never reports ready. Its pools point at an address nothing listens on, so
// every reload of the execution context fails at its first statement, and the check is read once the
// context has tried and failed and each job kind's reload has run (ADR-0138, ADR-0119).
func TestAReloadThatFailsLeavesItsJobKindNotReady(t *testing.T) {
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	pool := func() *pgxpool.Pool {
		p, err := pgxpool.New(t.Context(), "postgres://nobody@"+address+"/mailbox?connect_timeout=1&sslmode=disable")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		return p
	}
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	private, err := open.ParsePrivateKey(seed)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := open.NewKeyring(private.PublicKey(), private)
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pools := Pools{Backfill: pool(), Sync: pool()}
	ec := mustContext(t, pools, ring, scanner, defaults().Section, registry, slog.New(slog.DiscardHandler))
	parts, err := Assemble(ctx, Needs{
		Section: defaults().Section, Pools: pools, Context: ec,
		Registerer: registry, Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() { done <- parts.Tasks[0].Run(ctx) }()
	go func() { done <- ec.Run(ctx) }()
	defer func() {
		cancel()
		for range 2 {
			if err := <-done; err != nil {
				t.Error(err)
			}
		}
	}()
	// Each job kind's first reload has run once the scheduler has counted a finished run of the kind,
	// since the reload is the only job either kind runs while no account is listed.
	finishedReloads := func() int {
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, f := range families {
			if f.GetName() != "mediated_mailbox_job_runs_finished_total" {
				continue
			}
			for _, m := range f.GetMetric() {
				labels := map[string]string{}
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				if m.GetCounter().GetValue() > 0 {
					n++
				}
			}
		}
		return n
	}
	for deadline := time.Now().Add(10 * time.Second); finishedReloads() < 2 || errors.Is(ec.Latest().Err(), assembly.ErrNotLoaded); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the execution context's load and each job kind's reload did not run within ten seconds")
		}
	}
	for _, c := range parts.Ready {
		if strings.HasSuffix(c.Name, " reload") && c.Check(t.Context()) == nil {
			t.Errorf("the check %s passed while the execution context had loaded no accounts", c.Name)
		}
	}
}
