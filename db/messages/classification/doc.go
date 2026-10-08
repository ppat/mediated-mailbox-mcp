// Package messageclassification is the data-access subsection for the reads of the messages table that read
// the stored sender class, which the mediator may not be granted (ADR-0002, ADR-0075). It sits one
// directory below db/messages, so a role admitted to db/messages is never planned against a
// statement that reads the stored sender class.
package messageclassification
