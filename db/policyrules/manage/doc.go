// Package manage is the data-access subsection for the UI's policy management of an account's own
// rules, adding, editing and lifting them, and the read of the rules its policy screen lists
// (ADR-0084, ADR-0110). It sits one directory below db/policyrules, so the code that loads the policy
// reaches none of the UI's writes (ADR-0066, ADR-0143). The base policy's statements sit in db/policyrules/base.
package manage
