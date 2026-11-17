// Package policymanagement holds the read API's policy routes and their handlers: an account's policy
// reads and writes, its export and import, and the base policy's, each write with its history row in
// the write's transaction (docs/UI.md sections 8.7 and 8.14, ADR-0084, ADR-0102, ADR-0112). It reads
// the identity a policy write records. Each route carries its handler, and the read API endpoint,
// ui/internal/api, mounts them behind the checks. Its count of the base rules is handed to the setups
// for the installation's read.
package policymanagement
