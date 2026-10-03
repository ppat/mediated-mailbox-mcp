// Package manage is the data-access subsection for the UI's policy management of an account's own
// rules, adding, editing and lifting them, and the read of the rules its policy screen lists
// (ADR-0084, ADR-0110). It sits one directory below db/policyrules, so the roles that load the policy
// are never planned against the UI's writes. The base policy's statements sit in db/policyrules/base.
package manage
