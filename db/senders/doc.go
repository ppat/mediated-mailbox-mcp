// Package senders is the data-access subsection for the senders table, the per-account sender
// statistics the scan gate, the heuristics, the client surface's sender listing and the sender class
// term of the index reads read (ADR-0016, ADR-0108, ADR-0109). The class term classifies the domains
// the statistics hold, so it depends on them holding exactly the domains the index stores messages
// under. Its statements read the statistics. Their rebuild and the counts of prior scan hits sit in
// db/senders/statistics, and delta sync's removal of a sender with no stored message in
// db/messages/change.
package senders
