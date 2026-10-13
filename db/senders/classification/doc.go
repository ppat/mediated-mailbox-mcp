// Package senderclassification is the data-access subsection for the UI's reads of the senders table,
// whose senders the UI classifies under the policy in force when it reads (ADR-0136). The policy
// screens read every sender to show what each rule matches and restricts, Home sums the messages of
// the senders the policy restricts, and the sender picker searches the senders with their prior scan
// hits, which the API server's role is not granted (docs/UI.md sections 8.1 and 8.7, ADR-0118). It sits
// one directory below db/senders, so a role admitted to db/senders is never planned against a statement
// that reads the prior scan hits.
package senderclassification
