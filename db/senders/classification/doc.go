// Package senderclassification is the data-access subsection for the UI's reads of the senders table,
// whose senders the UI classifies under the policy in force when it reads (ADR-0136). The policy
// screens read every sender to show what each rule matches and restricts, Home sums the messages of
// the senders the policy restricts, and the sender picker searches the senders with their prior scan
// hits, which ADR-0109 keeps from the API server (docs/UI.md sections 8.1 and 8.7). It sits one
// directory below db/senders, which the API server's list admits, so the API server's code reaches no
// statement that reads the prior scan hits (ADR-0066, ADR-0143).
package senderclassification
