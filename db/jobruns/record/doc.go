// Package runrecord is the data-access subsection for a workload recording its own runs, their timeline
// and their per-item failures (ADR-0016, ADR-0022). It holds the writes to the job tables and the read
// a workload resumes from, apart from the reads in db/jobruns, so a role admitted to read runs is never
// planned against a statement that writes them.
package runrecord
