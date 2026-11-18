// Package reads is the pure rules of the UI's Jobs, System and event stream reads (docs/UI.md
// sections 8.3, 8.9 and 17.5). The shell reads the recorded runs and states, and these decide what a
// workload's state is, how many workloads run, whether an apply can be rolled back, when the next
// heuristics run is due, how long a running pass has left and whether an object changed since the
// stream last sent it, as values (ADR-0040). Instants arrive as nanoseconds since the epoch, so no
// clock is read here.
package reads

import "bytes"

// The derived workload states (docs/UI.md section 11). A workload with no run is not started, one
// with a run in running is running, and one whose runs have all finished is idle.
const (
	NotStarted = "not_started"
	Running    = "running"
	Idle       = "idle"
)

// Run is a recorded run's workload and state, the latest of each of its workload's passes.
type Run struct {
	Workload, State string
}

// WorkloadState returns workload's derived state from the latest run of each of its passes among
// runs.
func WorkloadState(runs []Run, workload string) string {
	result := NotStarted
	for _, r := range runs {
		if r.Workload != workload {
			continue
		}
		if r.State == Running {
			return Running
		}
		result = Idle
	}
	return result
}

// RunningWorkloads counts the workloads with a run in running among runs, a workload counting once.
func RunningWorkloads(runs []Run) int64 {
	running := map[string]bool{}
	for _, r := range runs {
		if r.State == Running {
			running[r.Workload] = true
		}
	}
	return int64(len(running))
}

// RollbackAvailable reports whether an apply whose plan's status is planStatus and identifier planID
// can be rolled back, which its plan being APPLIED with an identifier allows (docs/UI.md section 8.3).
func RollbackAvailable(planStatus, planID *string) bool {
	return planStatus != nil && *planStatus == "APPLIED" && planID != nil
}

// NextRun is when the next run is due, a cadence after the last one started.
func NextRun(lastStarted, cadence int64) int64 {
	return lastStarted + cadence
}

// Estimable reports whether a pass in state, started at started, can have an estimated time left at
// now over the last window of its history: it is running and started a window or more before now
// (docs/UI.md section 8.1).
func Estimable(state string, started, now, window int64) bool {
	return state == Running && started <= now-window
}

// TimeLeft is a running pass's estimated time left in seconds, at page of of pages, from the progress
// events it recorded in the last window of windowSeconds: the pages remaining divided by the pages
// completed in the window. It is nil when the window holds no event or completed no page.
func TimeLeft(page, of, events, done, windowSeconds int64) *int64 {
	if events == 0 || done <= 0 {
		return nil
	}
	seconds := max(0, of-page) * windowSeconds / done
	return &seconds
}

// Changed reports whether an object the stream reads, encoded as read, differs from what the stream
// last sent of it, nil for nothing sent, so its event is sent (docs/UI.md section 17.5).
func Changed(sent, read []byte) bool {
	return !bytes.Equal(sent, read)
}
