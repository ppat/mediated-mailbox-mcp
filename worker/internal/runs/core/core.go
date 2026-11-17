// Package core holds the decisions about a job kind's runs that both job kinds of the worker make the
// same way, as values (ADR-0040, ADR-0117). Begin decides how a run of a pass starts from its latest
// recorded run, which both of backfill's passes use, and Seed decides when a job's latest success is
// counted from at its ensure, from what its runs record (ADR-0119).
package core

import "github.com/ppat/mediated-mailbox-mcp/core/mail"

// RunState is a recorded run's state.
type RunState uint8

const (
	// Running is a run still running, or one that stopped without recording its end.
	Running RunState = iota + 1
	// Succeeded is a run that ended its pass.
	Succeeded
	// Failed is a run that ended without ending its pass.
	Failed
)

// Latest is the account's latest recorded run of a pass whose progress is a P.
type Latest[P any] struct {
	// Found is false when no run of the pass was ever recorded.
	Found    bool
	RunID    string
	State    RunState
	Progress P
}

// Start is how a run of a pass whose progress is a P starts.
type Start[P any] struct {
	// Skip is set when the pass has ended for the account and is not due again, so the run does no
	// work.
	Skip bool
	// Reopen is set when the pass had ended and is due again, so the shell records it as not ended
	// as the run starts (ADR-0120).
	Reopen bool
	// ResumedFrom is the run this one resumes, empty for a fresh pass.
	ResumedFrom string
	// Abandon is set when the run resumed is still recorded as running, which it is only when it
	// stopped without recording its end. The shell records it as failed as this run starts.
	Abandon bool
	// From is the progress the run starts from.
	From P
}

// Begin decides how a run of a pass starts for an account whose pass has or has not ended, and is or
// is not due again because the index holds work a scanner other than the one in force decided, from
// its latest recorded run. Both of backfill's passes start this way. A pass that ended and is not due
// is skipped, and one that ended and is due is reopened (ADR-0120). A run that stopped, whether it
// recorded its failure or not, is resumed from its checkpoint and counters. With no run, or when the
// latest one succeeded while the pass is recorded as not ended, which is how a pass is asked to run
// again, a fresh pass starts from the first page.
func Begin[P any](ended, due bool, latest Latest[P]) Start[P] {
	switch {
	case ended && !due:
		return Start[P]{Skip: true}
	case !latest.Found || latest.State == Succeeded:
		return Start[P]{Reopen: ended}
	default:
		return Start[P]{Reopen: ended, ResumedFrom: latest.RunID, Abandon: latest.State == Running, From: latest.Progress}
	}
}

// Seed returns when a job's latest success is counted from at the job's ensure, from what its runs
// record: its latest recorded success, a run that succeeded or a step it made durable, and the start
// of its earliest run while it has none, so a worker restarting without the job succeeding leaves it
// ageing. A job whose runs record nothing counts from the ensure, which Seed returns as zero
// (ADR-0119). Backfill's job counts from the ensure as well while it has no work outstanding, which
// its own core decides first.
func Seed(latestSuccess, earliestStart mail.UnixMilli) mail.UnixMilli {
	if latestSuccess != 0 {
		return latestSuccess
	}
	return earliestStart
}
