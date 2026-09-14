// Package marker holds the designed marker text that synthetic fixtures and generated mail values
// carry.
//
// It sits in the shared pure library rather than in testsupport because the mediator's readiness
// probe builds its known-sensitive fixture from it in non-test code, and non-test code never imports
// testsupport. The import list for non-test code enforces that.
package marker
