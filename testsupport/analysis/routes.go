package analysis

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Routes reports a route the UI registers on a mux other than through its recording mux, the rule
// ADR-0071 sets so that the route check of ADR-0057, which compares the routes registered through the
// recording mux's Handle with the contract document, sees every route the UI serves. It covers every
// package under ui/ and exempts test files, since a route a test registers is never served.
//
// Any use of (*http.ServeMux).Handle, (*http.ServeMux).HandleFunc, http.Handle or http.HandleFunc is
// reported, a call or a method value alike, and so is any use of a method named Handle or HandleFunc
// with the mux's parameters on an interface or a type parameter, which a mux can be held in. Only the
// body of the recording mux's own Handle method is exempt, named by its full name. So a use of those
// functions or methods on the mux beneath the recorder, or on a second mux, is reported anywhere else.
// Not reported are a registration through reflection, one an imported package makes on the default
// mux when it is initialised, such as net/http/pprof's, and one through an interface method one of
// whose parameter types is a type parameter, which ADR-0071 leaves to review. The mediator is out of
// scope. Its registrations are its roots' generators and its composition root, which ADR-0071 leaves
// to its import lists and review.
var Routes = &analysis.Analyzer{
	Name: "routes",
	Doc:  "reports a route the UI registers on a mux other than through its recording mux",
	Run:  runRoutes,
}

const routesMessage = "%s registers a route the UI's route check never sees. Register it through the recording mux's Handle (ADR-0057, ADR-0071)"

// recorderHandle is the one function whose body may register a route on a mux.
const recorderHandle = "(*" + modulePath + "/ui/internal/api.recordingMux).Handle"

// routeRegistrars are the refused functions by their full names, each with the finding's opening
// words.
var routeRegistrars = map[string]string{
	"(*net/http.ServeMux).Handle":     "(*http.ServeMux).Handle",
	"(*net/http.ServeMux).HandleFunc": "(*http.ServeMux).HandleFunc",
	"net/http.Handle":                 "http.Handle",
	"net/http.HandleFunc":             "http.HandleFunc",
}

func runRoutes(pass *analysis.Pass) (any, error) {
	path := pass.Pkg.Path()
	if path != modulePath+"/ui" && !strings.HasPrefix(path, modulePath+"/ui/") {
		return nil, nil
	}
	for _, file := range pass.Files {
		// A file not named .go is one the go command generates, such as a test binary's main.
		base := filepath.Base(pass.Fset.File(file.Pos()).Name())
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok {
				if fn, ok := pass.TypesInfo.Defs[fd.Name].(*types.Func); ok && fn.FullName() == recorderHandle {
					continue
				}
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
				if !ok {
					return true
				}
				if name, refused := routeRegistrars[fn.FullName()]; refused {
					pass.Reportf(id.Pos(), routesMessage, name)
				} else if interfaceRegistrar(fn) {
					pass.Reportf(id.Pos(), routesMessage, "the interface method "+fn.Name())
				}
				return true
			})
		}
	}
	return nil, nil
}

// interfaceRegistrars are the parameters of the mux's two registering methods, by method name.
var interfaceRegistrars = map[string]string{
	"Handle":     "string, net/http.Handler",
	"HandleFunc": "string, func(net/http.ResponseWriter, *net/http.Request)",
}

// interfaceRegistrar reports whether fn is a method of an interface, a type parameter's constraint
// included, named and typed like one of the mux's registering methods, so a mux held in such a value
// registers through it. Its parameters are compared with every alias replaced by the type it stands
// for, since an alias is identical to its type and a mux satisfies an interface spelled with one.
func interfaceRegistrar(fn *types.Func) bool {
	sig := fn.Signature()
	want, ok := interfaceRegistrars[fn.Name()]
	if !ok || sig.Recv() == nil || !types.IsInterface(sig.Recv().Type()) || sig.Results().Len() != 0 || sig.Variadic() {
		return false
	}
	return unaliasedParams(sig.Params()) == want
}

// unaliasedParams prints a parameter list the way types.TypeString does, with every alias replaced
// by the type it stands for.
func unaliasedParams(params *types.Tuple) string {
	printed := make([]string, params.Len())
	for i := range printed {
		printed[i] = unaliased(params.At(i).Type())
	}
	return strings.Join(printed, ", ")
}

// unaliased prints t with every alias replaced by the type it stands for, down through pointers and
// through the parameters of a function that returns nothing and is not variadic, the only composite
// types the mux's parameters hold. Any other type is printed as it is, and cannot match them.
func unaliased(t types.Type) string {
	switch t := types.Unalias(t).(type) {
	case *types.Pointer:
		return "*" + unaliased(t.Elem())
	case *types.Signature:
		if t.Results().Len() == 0 && !t.Variadic() {
			return "func(" + unaliasedParams(t.Params()) + ")"
		}
		return types.TypeString(t, nil)
	default:
		return types.TypeString(t, nil)
	}
}
