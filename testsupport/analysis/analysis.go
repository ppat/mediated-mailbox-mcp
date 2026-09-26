// Package analysis holds the project's go vet analysers, as a library. Placement carries the rules
// ADR-0069 sets on property tests, Globals the rule ADR-0071 sets against package-level state in a
// pure core, and Environment the rule ADR-0078 sets against reading the environment outside a
// deployable's composition root. The program under testsupport/cmd/vetcheck runs them through go
// vet, beside golangci-lint.
//
// The analysers honour no suppression comment, which is why they run under go vet rather than as
// golangci-lint plugins. The environment rule is scoped by path, and forbidigo could carry that
// scope only through an exclusion naming a linter that stands in for a control, which ADR-0071
// refuses.
//
// Code runs inside a property when it is a function handed to rapid to run for each generated case,
// or anything such a function calls in the same package. rapid re-runs a failing property many times
// while reducing it, so code there does not run once per case. Two calls must never be reachable from
// there.
//
//   - property.Report, whose count of the generated mix would include every re-run.
//   - property.Check, which writes the failing-case store, and would write it on every re-run.
//
// The functions handed to rapid are the function arguments of rapid.Check, rapid.MakeCheck,
// rapid.Custom, rapid.Map, rapid.Deferred and (*rapid.Generator).Filter, the actions of
// (*rapid.T).Repeat, and the draw, property and classify arguments of property.Check and
// property.Report. A function literal is followed whether it is written in place or assigned to a
// variable first. A method value is not followed, nor is a function literal passed as an argument
// to a helper that calls it.
package analysis

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	rapidPath    = "pgregory.net/rapid"
	propertyPath = "github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// Placement reports ADR-0069's placement rules for property tests.
var Placement = &analysis.Analyzer{
	Name: "placement",
	Doc:  "reports property-test code placed where rapid does not run it as written",
	Run:  run,
}

// messages are the findings, one per call that must not run inside a property.
var messages = map[string]string{
	"Report": "property.Report is reachable from inside a property, where rapid re-runs a failing case and every re-run would be counted. Call it from its own test",
	"Check":  "property.Check is reachable from inside a property, where it would write the failing-case store on every re-run. Call it from a test function",
}

func run(pass *analysis.Pass) (any, error) {
	decls := map[*types.Func]*ast.FuncDecl{}
	for _, file := range pass.Files {
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				if fn, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func); ok {
					decls[fn] = fd
				}
			}
		}
	}
	// closures maps a variable to every function literal assigned to it, so a closure passed to rapid
	// or called inside a property by its variable's name is followed like a declared function.
	closures := map[types.Object][]ast.Node{}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				for i, rhs := range s.Rhs {
					if lit, ok := ast.Unparen(rhs).(*ast.FuncLit); ok && i < len(s.Lhs) {
						if id, ok := s.Lhs[i].(*ast.Ident); ok {
							if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
								closures[obj] = append(closures[obj], lit.Body)
							}
						}
					}
				}
			case *ast.ValueSpec:
				for i, v := range s.Values {
					if lit, ok := ast.Unparen(v).(*ast.FuncLit); ok && i < len(s.Names) {
						if obj := pass.TypesInfo.ObjectOf(s.Names[i]); obj != nil {
							closures[obj] = append(closures[obj], lit.Body)
						}
					}
				}
			}
			return true
		})
	}
	w := walker{pass: pass, decls: decls, closures: closures, visited: map[ast.Node]bool{}, reported: map[finding]bool{}}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for _, body := range w.propertyBodies(call) {
				w.walk(body)
			}
			return true
		})
	}
	return nil, nil
}

type walker struct {
	pass     *analysis.Pass
	decls    map[*types.Func]*ast.FuncDecl
	closures map[types.Object][]ast.Node
	visited  map[ast.Node]bool
	reported map[finding]bool
}

// finding is one rule's report at one call, so a call reached from two properties is reported once.
type finding struct {
	call ast.Node
	rule string
}

// callee returns the function a call statically calls, generic instantiations resolved to their
// origin, or nil.
func (w *walker) callee(call *ast.CallExpr) *types.Func {
	fn := typeutil.StaticCallee(w.pass.TypesInfo, call)
	if fn == nil {
		return nil
	}
	return fn.Origin()
}

func isFunc(fn *types.Func, path, name string) bool {
	return fn != nil && fn.Pkg() != nil && fn.Pkg().Path() == path && fn.Name() == name
}

// isRepeat reports whether fn is the Repeat method of rapid's T.
func isRepeat(fn *types.Func) bool { return isMethod(fn, "Repeat") }

// isMethod reports whether fn is the method name of one of rapid's types.
func isMethod(fn *types.Func, name string) bool {
	if fn == nil || fn.Name() != name || fn.Pkg() == nil || fn.Pkg().Path() != rapidPath {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() != nil
}

// propertyBodies returns the function arguments of call that rapid runs for each generated case.
func (w *walker) propertyBodies(call *ast.CallExpr) []ast.Node {
	fn := w.callee(call)
	switch {
	case isFunc(fn, rapidPath, "Check"), isFunc(fn, rapidPath, "MakeCheck"), isFunc(fn, rapidPath, "Custom"),
		isFunc(fn, rapidPath, "Map"), isFunc(fn, rapidPath, "Deferred"), isMethod(fn, "Filter"),
		isFunc(fn, propertyPath, "Check"), isFunc(fn, propertyPath, "Report"):
		var bodies []ast.Node
		for _, arg := range call.Args {
			bodies = append(bodies, w.functions(arg)...)
		}
		return bodies
	case isRepeat(fn):
		var bodies []ast.Node
		for _, arg := range call.Args {
			lit, ok := ast.Unparen(arg).(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, elt := range lit.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					bodies = append(bodies, w.functions(kv.Value)...)
				}
			}
		}
		return bodies
	default:
		return nil
	}
}

// functions returns the bodies expr can stand for when it is a function literal, names a function
// declared in this package, or names a variable a function literal is assigned to.
func (w *walker) functions(expr ast.Expr) []ast.Node {
	switch e := ast.Unparen(expr).(type) {
	case *ast.FuncLit:
		return []ast.Node{e.Body}
	case *ast.Ident:
		obj := w.pass.TypesInfo.Uses[e]
		if fn, ok := obj.(*types.Func); ok {
			if fd := w.decls[fn.Origin()]; fd != nil {
				return []ast.Node{fd.Body}
			}
		}
		return w.closures[obj]
	case *ast.IndexExpr:
		return w.functions(e.X)
	case *ast.IndexListExpr:
		return w.functions(e.X)
	}
	return nil
}

// walk reports every forbidden call reachable from body, following calls to functions declared in
// this package.
func (w *walker) walk(body ast.Node) {
	if w.visited[body] {
		return
	}
	w.visited[body] = true
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn := w.callee(call)
		for name, msg := range messages {
			if isFunc(fn, propertyPath, name) && !w.reported[finding{call, name}] {
				w.reported[finding{call, name}] = true
				w.pass.Reportf(call.Pos(), "%s", msg)
			}
		}
		if fn != nil {
			if fd := w.decls[fn]; fd != nil {
				w.walk(fd.Body)
			}
		}
		if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
			for _, body := range w.closures[w.pass.TypesInfo.Uses[id]] {
				w.walk(body)
			}
		}
		return true
	})
}
