// Package app is the worker's entry package, which holds its spec and nothing else (ADR-0142). The
// spec lists the worker's job kinds, backfill and delta sync, each built by the Build of its own
// package, which returns the plan the process's scheduler keeps the kind's jobs by (ADR-0117,
// ADR-0119). The process starts the worker as it starts every program, builds each job kind with its
// own pool, its own labels and the things the process shares, the Session, the connections and the
// scanner, runs every plan, answers its probes and serves its metrics until it is told to stop. It
// sits outside internal, so a spec other than the worker's own can join its job kinds, and every
// other package of this deployable sits under internal.
package app

import (
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync"
)

// Spec is the worker's spec.
func Spec() process.Spec[config.Worker] {
	return process.Spec[config.Worker]{
		Name: "worker",
		Jobs: []process.Job[config.Worker]{
			{Name: backfill.Kind, Build: backfill.Build},
			{Name: deltasync.Kind, Build: deltasync.Build},
		},
	}
}
