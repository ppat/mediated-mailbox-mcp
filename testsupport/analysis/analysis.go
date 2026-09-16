// Package analysis holds the go vet analyser for the placement rules ADR-0069 sets on property
// tests, as a library. The program under testsupport/cmd/vetcheck runs it through go vet, beside
// golangci-lint.
//
// The analyser honours no suppression comment, which is why it runs under go vet rather than as a
// golangci-lint plugin.
//
// It reports nothing yet. ADR-0069 gives it two rules, a generator report call reachable from a
// property that can fail, and a failing-case store write from inside a property. Both name functions
// of testsupport/property, which do not exist until the property tests are written. Each rule lands
// with those functions and with a violation file that banproof requires go vet to report. Until then
// no CI step runs the analyser, since a run with no rule would pass having checked nothing.
package analysis

import "golang.org/x/tools/go/analysis"

// Analyzer reports ADR-0069's placement rules for property tests.
var Analyzer = &analysis.Analyzer{
	Name: "placement",
	Doc:  "reports property-test code placed where rapid does not run it as written",
	Run:  func(*analysis.Pass) (any, error) { return nil, nil },
}
