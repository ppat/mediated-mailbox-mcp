// Package record is the data-access subsection for a scanning workload recording the scan gate's
// decisions (ADR-0016, ADR-0093). It holds the write to the decisions table, apart from the reads in
// db/scangatedecisions, so a role admitted to read decisions is never planned against a statement
// that writes them.
package record
