// Package senders is the data-access subsection for the reads of masking events counted under the domain
// of each event's message's sender, which join the message and read its sender's domain, a column the
// mediator's role is not granted (ADR-0075). It sits one directory below db/maskingevents, so a role
// admitted to db/maskingevents is never planned against a statement that reads the sender's domain.
package senders
