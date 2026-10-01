// Package record is the data-access subsection for the mediator's recording of each body it serves or
// denies (ADR-0002, ADR-0016). It holds the write to the audit_log table, apart from the reads in
// db/auditlog, so a role admitted to read the audit log is never planned against a statement that
// writes it.
package record
