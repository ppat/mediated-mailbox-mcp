// Package jobruns is the data-access subsection for the batch workloads' runs, which are the job_runs
// table, each run's timeline in job_run_events and its per-item failures in job_run_failures (ADR-0016).
// It holds the reads of a run's failures too, which join each item's message with its sender's domain
// for the UI to classify under the policy in force when it reads (ADR-0136).
package jobruns
