// Package due is delta sync's due decision, which the worker's scheduler asks of an account's delta
// sync job at every wake (ADR-0119, ADR-0040). It takes the time and the job's last tick as values.
package due

import "github.com/ppat/mediated-mailbox-mcp/core/mail"

// Decide reports whether a tick is due at at, which is when the job was asked. One is due when the
// job has not ticked since the process started, which ticked reports, or when the sync interval,
// intervalMillis milliseconds, has passed since last, the latest time on the ticker's phase at or
// before the ask the last tick was made at, whether it succeeded or failed, so a failed tick waits for
// the next interval as every tick does (ADR-0103). The scheduler asks a timed job on its ticker's
// phase, and at is the time on that phase it asked at, so ticks asked one interval apart are due
// whatever delayed each one's start. A tick asked off the phase, at a backoff's end, is recorded at
// the time on the phase before it, so the ticker's next tick is due.
func Decide(ticked bool, last, at mail.UnixMilli, intervalMillis int64) bool {
	return !ticked || int64(at-last) >= intervalMillis
}

// Seed returns when the account's delta sync job's latest success is counted from at the job's
// ensure, from what its ticks record: its latest recorded success, a tick that succeeded or a step it
// made durable, and the start of its earliest tick while it has none, so a worker restarting without
// the job succeeding leaves it ageing. A job whose ticks record nothing counts from the ensure, which
// Seed returns as zero (ADR-0119).
func Seed(latestSuccess, earliestStart mail.UnixMilli) mail.UnixMilli {
	if latestSuccess != 0 {
		return latestSuccess
	}
	return earliestStart
}
