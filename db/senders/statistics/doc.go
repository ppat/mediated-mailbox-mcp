// Package statistics is the data-access subsection for the rebuild of the senders table's statistics
// from the stored messages and the counts of prior scan hits the scan gate reads (ADR-0016, ADR-0017,
// ADR-0093). It holds them apart from the reads in db/senders, so a role admitted to read the
// statistics is never planned against a statement that writes them. Delta sync's removal of a sender
// with no stored message sits in db/messages/change.
package statistics
