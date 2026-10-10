// Package due is backfill's due decision, which the worker's scheduler asks of an account's backfill
// job at every wake (ADR-0119, ADR-0040). It takes recorded state and what this process has done as
// values and names the work there is, and the passes' own decisions say what each pass does.
package due

import "github.com/ppat/mediated-mailbox-mcp/core/mail"

// State is what recorded state says of an account's backfill, its completion flags and the mark
// that starts its second pass over, as account_state holds them (ADR-0017, ADR-0120).
type State struct {
	FirstEnded, SecondEnded, Restart bool
}

// Work is the work an account's backfill has.
type Work struct {
	// Start is the run-start step, which returns to pending the verdicts another scanner made and the
	// gate skips the gate no longer decides as the same skip, and masks again from the store the
	// subjects stored unmasked under another scanner (ADR-0120, ADR-0121).
	Start bool
	// Passes is set when a pass is not ended, or the second is marked to start over, so the run opens
	// each pass, which skips one that has ended.
	Passes bool
}

// Any reports whether there is any work.
func (w Work) Any() bool { return w.Start || w.Passes }

// Decide returns the account's work. The run-start step is due once for each account a process
// serves, until started reports this process has made it, because its inputs, the scanner and the
// gate's thresholds, change only across a restart (ADR-0121). The passes are due while either has
// not ended or the second is marked to start over, and the run-start step opens them too, since it
// may reopen them.
func Decide(s State, started bool) Work {
	return Work{Start: !started, Passes: !started || !s.FirstEnded || !s.SecondEnded || s.Restart}
}

// Seed returns when the account's backfill job's latest success is counted from at the job's ensure,
// from what its runs record. A job with work outstanding, a pass not ended or the second marked to
// start over, counts from its latest recorded success, a page made durable or a run that succeeded,
// and from the start of its earliest run while it has none, so a worker restarting without the job
// succeeding leaves it ageing. A job with no work outstanding, and one whose runs record nothing,
// count from the ensure, which Seed returns as zero, since neither has anything that can fall behind
// (ADR-0119).
func Seed(s State, latestSuccess, earliestStart mail.UnixMilli) mail.UnixMilli {
	if !Decide(s, true).Passes {
		return 0
	}
	if latestSuccess != 0 {
		return latestSuccess
	}
	return earliestStart
}
