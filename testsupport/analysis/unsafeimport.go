package analysis

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// UnsafeImport reports an import of unsafe in any non-test file of this module, the rule ADR-0117
// sets because code isolation per job kind in the worker rests on memory safety. Every component's
// import list admits the standard library through one entry, and ADR-0071 gives no list a deny
// entry, so the rule is an analyser. A go:linkname needs the import too, so the rule covers it.
// reflect hands out and takes an unsafe pointer without the import, and reflect.NewAt over a
// pointer Value.UnsafePointer returned reads memory as any type, so the rule also refuses any use
// of the functions in reflectUnsafe, a call, a method value and a method expression alike. Such a
// function reached through an interface value or through reflect's own method lookup is left to
// review, and so is writing the process's own memory through the operating system, such as through
// /proc/self/mem at an address Value.Pointer gives, which needs neither. It exempts test files.
// What the module's dependencies import is the worker's own test's to watch.
var UnsafeImport = &analysis.Analyzer{
	Name: "unsafeimport",
	Doc:  "reports an import of unsafe, or a use of reflect's unsafe pointers, in this module's code",
	Run:  runUnsafeImport,
}

// reflectUnsafe are every one of reflect's functions, by their full names, that takes or returns an
// unsafe.Pointer.
var reflectUnsafe = map[string]bool{
	"reflect.NewAt":                 true,
	"reflect.SliceAt":               true,
	"(reflect.Value).UnsafePointer": true,
	"(reflect.Value).SetPointer":    true,
}

const (
	unsafeMessage        = "imports unsafe, which no code of this project uses, since code isolation per job kind rests on memory safety (ADR-0117)"
	reflectUnsafeMessage = "uses one of reflect's unsafe pointers, which no code of this project uses, since code isolation per job kind rests on memory safety (ADR-0117)"
)

func runUnsafeImport(pass *analysis.Pass) (any, error) {
	path := pass.Pkg.Path()
	if path != modulePath && !strings.HasPrefix(path, modulePath+"/") {
		return nil, nil
	}
	for _, file := range pass.Files {
		// A file not named .go is one the go command generates, such as a test binary's main.
		base := filepath.Base(pass.Fset.File(file.Pos()).Name())
		if !strings.HasSuffix(base, ".go") || strings.HasSuffix(base, "_test.go") {
			continue
		}
		for _, spec := range file.Imports {
			if imported, err := strconv.Unquote(spec.Path.Value); err == nil && imported == "unsafe" {
				pass.Reportf(spec.Pos(), unsafeMessage)
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				if fn, ok := pass.TypesInfo.Uses[id].(*types.Func); ok && reflectUnsafe[fn.FullName()] {
					pass.Reportf(id.Pos(), reflectUnsafeMessage)
				}
			}
			return true
		})
	}
	return nil, nil
}
