// Package placement holds the cases the analyser's tests check, each want naming the finding the
// line must produce.
package placement

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
	"pgregory.net/rapid"
)

func draw(t *rapid.T) int      { return 0 }
func classify(n int) string    { return "" }
func prop(t rapid.TB, n int)   {}

// Called from test functions, both are where they belong.
func TestAtTopLevel(t *testing.T) {
	property.Report(t, draw, classify, nil)
	property.Check(t, draw, prop)
}

func TestReportInsideAProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		property.Report(t, draw, classify, nil) // want "property.Report is reachable from inside a property"
	})
}

func TestCheckInsideAProperty(t *testing.T) {
	property.Check(t, draw, func(tb rapid.TB, n int) {
		property.Check(t, draw, prop) // want "property.Check is reachable from inside a property"
	})
}

// helper is called from a property below, so its call is inside that property.
func helper(t *testing.T) {
	property.Report(t, draw, classify, nil) // want "property.Report is reachable from inside a property"
	recurse(t)
}

func recurse(t *testing.T) { helper(t) }

func TestThroughAHelper(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) { helper(t) })
}

func TestInsideARepeatAction(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		rt.Repeat(map[string]func(*rapid.T){
			"act": func(*rapid.T) {
				property.Check(t, draw, prop) // want "property.Check is reachable from inside a property"
			},
		})
	})
}

func TestInsideAGenerator(t *testing.T) {
	_ = rapid.Custom(func(rt *rapid.T) int {
		property.Report(t, draw, classify, nil) // want "property.Report is reachable from inside a property"
		return 0
	})
}

func namedProperty(rt *rapid.T) {
	property.Check(nil, draw, prop) // want "property.Check is reachable from inside a property"
}

func TestANamedProperty(t *testing.T) {
	_ = rapid.MakeCheck(namedProperty)
}

// A report classifier is inside the report's own property.
func TestInsideAClassifier(t *testing.T) {
	property.Report(t, draw, func(n int) string {
		property.Report(t, draw, classify, nil) // want "property.Report is reachable from inside a property"
		return ""
	}, nil)
}

// A property assigned to a variable before it is handed to rapid is still a property.
func TestAPropertyInAVariable(t *testing.T) {
	prop := func(rt *rapid.T) {
		property.Report(t, draw, classify, nil) // want "property.Report is reachable from inside a property"
	}
	rapid.Check(t, prop)
}

// A closure called by its variable's name inside a property runs inside that property.
func TestAClosureCalledInsideAProperty(t *testing.T) {
	store := func() {
		property.Check(t, draw, prop) // want "property.Check is reachable from inside a property"
	}
	rapid.Check(t, func(rt *rapid.T) { store() })
}

// Functions handed to Map, Filter and Deferred run while rapid generates.
func TestInsideMapFilterAndDeferred(t *testing.T) {
	g := rapid.Custom(draw)
	_ = rapid.Map(g, func(n int) int {
		property.Report(t, draw, classify, nil) // want "property.Report is reachable from inside a property"
		return n
	})
	_ = g.Filter(func(n int) bool {
		property.Check(t, draw, prop) // want "property.Check is reachable from inside a property"
		return true
	})
	_ = rapid.Deferred(func() *rapid.Generator[int] {
		property.Report(t, draw, classify, nil) // want "property.Report is reachable from inside a property"
		return g
	})
}

// A closure assigned to a variable but never handed to rapid or called inside a property is not
// inside one.
func TestAClosureOutsideAnyProperty(t *testing.T) {
	later := func() { property.Report(t, draw, classify, nil) }
	later()
}
