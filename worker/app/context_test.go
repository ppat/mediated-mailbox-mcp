package app

import (
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/assembly"
)

// mustContext returns the execution context newContext builds, failing the test when it cannot be
// built.
func mustContext(t testing.TB, pools Pools, keys *open.Keyring, scanner scan.Scanner, c config.Worker, reg prometheus.Registerer, logger *slog.Logger) *assembly.Context {
	t.Helper()
	ec, err := newContext(pools, keys, scanner, c.ReloadInterval, reg, logger)
	if err != nil {
		t.Fatal(err)
	}
	return ec
}
