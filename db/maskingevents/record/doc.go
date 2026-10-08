// Package maskingrecord is the data-access subsection for recording the masks applied to subjects as messages
// are ingested (ADR-0003, ADR-0016). It holds the writes to the masking_events table, apart from the
// reads in db/maskingevents, so a role admitted to read masking events is never planned against a
// statement that writes them.
package maskingrecord
