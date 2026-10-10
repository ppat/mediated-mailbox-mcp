// Package senderclassification is the data-access subsection for the reads of the senders table that read
// the stored sender class, which the mediator may not be granted (ADR-0002, ADR-0118). The UI's policy
// screens read it to show what each rule matches and how far the stored classes have caught up, and
// its sender picker to search the stored senders (docs/UI.md section 8.7). It sits one directory below
// db/senders, so a role admitted to db/senders is never planned against a statement that reads the
// stored sender class.
package senderclassification
