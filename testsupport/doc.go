// Package testsupport is the project's shared test tooling, published as
// mediated-mailbox-testsupport. It is a narrow, named exception to the rule that shared code is
// pure.
//
// Each piece is one subpackage, so a test's import list names exactly the pieces that test may use.
// The programs sit under cmd and run through tool directives in go.mod. Non-test code never imports
// anything here. Nothing belongs in this package itself.
package testsupport
