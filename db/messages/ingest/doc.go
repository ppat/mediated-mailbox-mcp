// Package ingest is the data-access subsection for adding messages' metadata to the index as they are
// ingested (ADR-0016, ADR-0017). It holds the writes to the messages table, apart from the reads in
// db/messages, so a role admitted to read messages is never planned against a statement that writes
// them.
package ingest
