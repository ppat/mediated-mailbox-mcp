// Package scan is the data-access subsection for a message's position relative to the content
// scanner (ADR-0016, ADR-0093). It holds the read of the messages waiting for a scan, the writes of a
// scan verdict and a skip, the delisting transition's read and write of the stored sender class
// (ADR-0037), and the backlog count a scanning workload emits. They sit apart from the reads in
// db/messages, so the roles admitted there are never planned against a statement that writes messages
// or reads the stored sender class.
package scan
