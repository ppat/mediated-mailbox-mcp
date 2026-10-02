// Package change is the data-access subsection for delta sync's application of the provider's
// changes to stored messages, under the per-account row-level security policy (ADR-0016). It holds
// the update of a stored message's labels and flags, the removal of a message the provider no
// longer holds and the removal of a sender's statistics once none of its messages is stored
// (ADR-0018), and the read of the stored messages dated in a gap recovery's window (ADR-0105). Only delta sync's list admits it, so no other role is planned against a statement
// that removes a message.
package change
