// Package runclassification is the data-access subsection for the reads of a run's failures, which
// read the failure columns the API server's role is not granted and join each item's message with its
// sender's domain, which the UI classifies under the policy in force when it reads (ADR-0118,
// ADR-0136). It sits one directory below db/jobruns, so a role admitted to db/jobruns is never planned
// against a statement that reads those columns.
package runclassification
