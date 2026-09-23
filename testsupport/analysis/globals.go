package analysis

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

// Globals reports package-level state in a pure core, the rule ADR-0071 sets for ADR-0040's pure
// cores. A package-level variable a composition root sets would be a dependency no parameter shows,
// and one a core writes would be state carried from one call to the next.
//
// Four rules follow. A non-test file of a pure-core package declares no package-level variable
// other than the blank identifier and an error value, and an error value has no initial value or
// is made by errors.New from a constant message, so it holds nothing a call could change. No file
// anywhere writes a pure-core package's package-level variable other than by its declaration,
// whether by assignment, by increment or decrement, as a range loop's iteration variable, or by
// taking its address, and writing through a field, an index or a pointer counts as writing the
// variable. No file links to a pure-core package's symbol with a go:linkname directive, which would
// reach the variable around its declaration. And a non-test file of a pure-core package carries no
// line directive.
//
// A variable a pure core's test file declares is not part of the core, so its own package and that
// package's external tests may write it, and no other package can see it. Export data gives a
// variable the file name a line directive names, so a directive would make a variable of the core
// pass for a test's in the external tests, which is why the fourth rule refuses one. Inside the
// package the file name is read with directives ignored as well. A pure-core package is one whose
// path this module places under a directory named core, as the pure-core import list in
// .golangci.yaml matches it.
//
// A link and a line directive are reported at the file's package clause, because a comment after
// either directive would stop the go command reading it as one.
var Globals = &analysis.Analyzer{
	Name: "globals",
	Doc:  "reports package-level state in a pure core",
	Run:  runGlobals,
}

const modulePath = "github.com/ppat/mediated-mailbox-mcp"

// pureCore matches the path of a pure-core package relative to the module. That is core itself, a
// library's core, and a deployable's internal core, each with the packages beneath it.
var pureCore = regexp.MustCompile(`^(core|[^/]+/core|[^/]+/internal/core)(/|$)`)

func isPureCorePath(path string) bool {
	rel, ok := strings.CutPrefix(path, modulePath+"/")
	return ok && pureCore.MatchString(rel)
}

func isPureCore(pkg *types.Package) bool { return pkg != nil && isPureCorePath(pkg.Path()) }

const (
	declaredMessage = "a pure core declares no package-level variable other than an error value, because one set from outside would be a dependency no parameter shows (ADR-0040)"
	errorMessage    = "a pure core's error value has no initial value or is made by errors.New from a constant message, so it holds nothing a call could change (ADR-0040)"
	writtenMessage  = "writes %s, a pure core's package-level variable, which only its declaration may set (ADR-0040)"
	linkedMessage   = "links to %s in a pure core, which reaches its package-level state around its declaration (ADR-0040)"
	lineMessage     = "a pure core's file carries a line directive, which would let its variables pass for a test file's (ADR-0040)"
)

func runGlobals(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// A file not named .go is one the go command generates, such as a test binary's main.
		name := pass.Fset.File(file.Pos()).Name()
		if isPureCore(pass.Pkg) && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			checkDeclarations(pass, file)
			checkLineDirectives(pass, file)
		}
		for _, group := range file.Comments {
			for _, c := range group.List {
				fields := strings.Fields(c.Text)
				if len(fields) < 3 || fields[0] != "//go:linkname" {
					continue
				}
				if i := strings.LastIndex(fields[2], "."); i > 0 && isPureCorePath(fields[2][:i]) {
					pass.Reportf(file.Package, linkedMessage, fields[2])
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if s.Tok != token.DEFINE {
					for _, lhs := range s.Lhs {
						reportWrite(pass, lhs)
					}
				}
			case *ast.IncDecStmt:
				reportWrite(pass, s.X)
			case *ast.RangeStmt:
				if s.Tok == token.ASSIGN {
					reportWrite(pass, s.Key)
					reportWrite(pass, s.Value)
				}
			case *ast.UnaryExpr:
				if s.Op == token.AND {
					reportWrite(pass, s.X)
				}
			}
			return true
		})
	}
	return nil, nil
}

// checkDeclarations reports the package-level variables of one non-test file of a pure core.
func checkDeclarations(pass *analysis.Pass, file *ast.File) {
	errorType := types.Universe.Lookup("error").Type()
	for _, d := range file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, id := range vs.Names {
				obj := pass.TypesInfo.Defs[id]
				switch {
				case id.Name == "_" || obj == nil:
				case !types.Identical(obj.Type(), errorType):
					pass.Reportf(id.Pos(), "%s", declaredMessage)
				case len(vs.Values) == 0:
				case len(vs.Values) != len(vs.Names) || !constantErrorsNew(pass, vs.Values[i]):
					pass.Reportf(id.Pos(), "%s", errorMessage)
				}
			}
		}
	}
}

// checkLineDirectives reports a line directive in one non-test file of a pure core, once, at the
// file's package clause.
func checkLineDirectives(pass *analysis.Pass, file *ast.File) {
	for _, group := range file.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//line ") || strings.HasPrefix(c.Text, "/*line ") {
				pass.Reportf(file.Package, "%s", lineMessage)
				return
			}
		}
	}
}

// constantErrorsNew reports whether expr calls errors.New with a constant message.
func constantErrorsNew(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !isFunc(typeutil.StaticCallee(pass.TypesInfo, call), "errors", "New") {
		return false
	}
	return pass.TypesInfo.Types[call.Args[0]].Value != nil
}

// reportWrite reports expr when writing it writes a pure core's package-level variable.
func reportWrite(pass *analysis.Pass, expr ast.Expr) {
	if expr == nil {
		return
	}
	if v := written(pass, expr); v != nil {
		pass.Reportf(expr.Pos(), writtenMessage, v.Name())
	}
}

// written returns the package-level variable of a pure core that writing expr writes, or nil. A
// write through a field, an index or a pointer writes the variable the expression starts from.
func written(pass *analysis.Pass, expr ast.Expr) *types.Var {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return written(pass, e.X)
	case *ast.Ident:
		return pureCoreVar(pass, pass.TypesInfo.Uses[e])
	case *ast.SelectorExpr:
		if v := pureCoreVar(pass, pass.TypesInfo.Uses[e.Sel]); v != nil {
			return v
		}
		return written(pass, e.X)
	case *ast.IndexExpr:
		return written(pass, e.X)
	case *ast.IndexListExpr:
		return written(pass, e.X)
	case *ast.StarExpr:
		return written(pass, e.X)
	}
	return nil
}

// pureCoreVar returns obj when it is a package-level variable of a pure-core package that the
// package being analysed may not write.
func pureCoreVar(pass *analysis.Pass, obj types.Object) *types.Var {
	v, ok := obj.(*types.Var)
	if !ok || v.Pkg() == nil || v.Pkg().Scope().Lookup(v.Name()) != v || !isPureCore(v.Pkg()) {
		return nil
	}
	own := pass.Pkg.Path() == v.Pkg().Path() || pass.Pkg.Path() == v.Pkg().Path()+"_test"
	if own && strings.HasSuffix(pass.Fset.PositionFor(v.Pos(), false).Filename, "_test.go") {
		return nil
	}
	return v
}
