// Package classification is the data-access subsection for the reads of a run's failures, which join
// each item's message and read its stored sender class, which the mediator may not be granted
// (ADR-0002, ADR-0075). It sits one directory below db/jobruns, so a role admitted to db/jobruns is
// never planned against a statement that reads the stored sender class.
package classification
