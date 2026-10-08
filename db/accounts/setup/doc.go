// Package accountsetup is the data-access subsection for the UI's account setup, writing an account's
// listing row and state row together, replacing its credential, moving it to another client, reading
// what account settings shows and writing its lowered target (ADR-0084, ADR-0091, ADR-0106). It reads
// no credential. Only the UI's list admits it, so no other role is planned against its writes.
package accountsetup
