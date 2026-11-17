package analysis

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Callers reports a call of one of the connections' two held methods from outside the one package
// each is held to, the rules ADR-0071 sets so that each stays where its record puts it.
//
//   - The attach of the connections to the Session, (*sessions.Session).Attach, is used only in
//     process/connections, so code that holds the Session reaches a credential only through the
//     connections' own package, which the list of a job kind that calls no provider refuses
//     (ADR-0117, ADR-0142).
//   - The re-seal of the stored credentials to a new key, (*connections.Connections).Reseal, is used
//     only in delta sync's package, worker/internal/deltasync, since only delta sync re-seals
//     (ADR-0092).
//
// Any use of either method outside its package is reported, a call, a method value and a method
// expression alike, in every package of the module, and so is any use of a method of the same name and
// the same signature on an interface or a type parameter, which the method's receiver can be held in.
// Test files are exempt, since a call a test makes is never served. Left to review are a call made
// through reflection, and one through an interface method whose name or signature differs from the
// held method's, such as one returning the scan through another type, which no rule here can tell
// from an unrelated method.
var Callers = &analysis.Analyzer{
	Name: "callers",
	Doc:  "reports a call of the connections' attach or re-seal from outside the one package each is held to",
	Run:  runCallers,
}

// heldMethod is one held method, its full name, its name, its parameters and results as
// unaliasedParams prints them, the package it is held to, and the finding.
type heldMethod struct {
	name    string
	method  string
	params  string
	results string
	only    string
	message string
}

var heldMethods = []heldMethod{
	{
		name: "(*" + modulePath + "/process/sessions.Session).Attach", method: "Attach",
		params: modulePath + "/process/sessions.Connections", results: "error",
		only:    modulePath + "/process/connections",
		message: "uses the Session's Attach outside process/connections, so code holding the Session could attach connections of its own. The connections attach themselves when they are built (ADR-0071, ADR-0142)",
	},
	{
		name: "(*" + modulePath + "/process/connections.Connections).Reseal", method: "Reseal",
		params: "context.Context", results: modulePath + "/process/connections.Scan",
		only:    modulePath + "/worker/internal/deltasync",
		message: "uses the connections' Reseal outside delta sync's package, and only delta sync re-seals the stored credentials (ADR-0071, ADR-0092)",
	},
}

func runCallers(pass *analysis.Pass) (any, error) {
	path := pass.Pkg.Path()
	if !within(path, modulePath) {
		return nil, nil
	}
	for _, file := range pass.Files {
		// A file not named .go is one the go command generates, such as a test binary's main.
		base := filepath.Base(pass.Fset.File(file.Pos()).Name())
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
			if !ok {
				return true
			}
			for _, h := range heldMethods {
				if path != h.only && (fn.FullName() == h.name || sameInterfaceMethod(fn, h)) {
					pass.Reportf(id.Pos(), "%s", h.message)
				}
			}
			return true
		})
	}
	return nil, nil
}

// sameInterfaceMethod reports whether fn is a method of an interface, a type parameter's constraint
// included, with the held method's name and the same parameters and results, which a value of the
// held method's receiver satisfies. Its types are compared with every alias replaced by the type it
// stands for, since an alias is identical to its type.
func sameInterfaceMethod(fn *types.Func, h heldMethod) bool {
	sig := fn.Signature()
	if fn.Name() != h.method || sig.Recv() == nil || !types.IsInterface(sig.Recv().Type()) || sig.Variadic() {
		return false
	}
	return unaliasedParams(sig.Params()) == h.params && unaliasedParams(sig.Results()) == h.results
}
