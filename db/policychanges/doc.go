// Package policychanges is the data-access subsection for the policy_changes table, the policy history
// (ADR-0102). Its statements append one of the account's own rules' changes, and read the history an
// account's policy screens show, its own changes and the base policy's. The base policy's own appends
// and reads, which run in a base-policy transaction, sit in db/policyrules/base (ADR-0112). Only the UI's
// list admits it.
package policychanges
