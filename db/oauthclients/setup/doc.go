// Package setup is the data-access subsection for the UI's OAuth client setup, listing each client's
// identity, adding, replacing and removing a client, and reading a client's identity when a consent
// finishes (ADR-0084, ADR-0106, ADR-0107). Its statements read no client secret and write a sealed one.
// Only the UI's list admits it, so no other role is planned against them.
package setup
