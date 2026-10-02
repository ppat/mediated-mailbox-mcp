// Package senders is the data-access subsection for the senders table, the per-account sender
// statistics the scan gate, the heuristics and the client surface's sender listing read (ADR-0016,
// ADR-0109). Its statements read the statistics. Their rebuild and the counts of prior scan hits sit
// in db/senders/statistics, and delta sync's removal of a sender with no stored message in
// db/messages/change.
package senders
