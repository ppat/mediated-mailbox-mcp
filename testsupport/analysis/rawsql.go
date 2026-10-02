package analysis

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// RawSQL reports a statement the UI runs other than through the data-access library, the rule
// ADR-0071 sets so that the UI's role's read of an OAuth client's sealed secret stays with
// ui/internal/clientsecret, the one package its import lists admit that read's subsection to
// (ADR-0081). It covers every package under ui/ and exempts test files, since a statement a test runs
// is never served.
//
// Any use of a method that runs a statement is reported, a call or a method value alike, whatever
// type declares it, matched by its name and by how its parameters open: Query, QueryRow, Exec and
// ExecParams given a context.Context and a statement's text, Prepare given a context, a name and a
// statement's text, SendBatch and ExecBatch given the driver's batches, CopyFrom given the driver's
// table identifier or a reader and a statement, CopyTo given a writer and a statement, and
// StartPipeline returning the driver's pipeline. So the driver's connection, pool, transaction and
// lower-level connection are reported, and so are a generated subsection's DBTX interface, an
// interface embedding it, and a wrapper or interface the UI declares itself. A parameter spelled
// through an alias counts as the type it stands for. The generated statements sit under db/,
// outside the scope. Not reported are a statement run through reflection, written to the wire
// through the raw network connection or protocol frontend the lower-level connection hands out, and
// run through a method one of whose parameter types is a type parameter, which ADR-0071 leaves to
// review.
var RawSQL = &analysis.Analyzer{
	Name: "rawsql",
	Doc:  "reports a statement the UI runs other than through the data-access library",
	Run:  runRawSQL,
}

const rawSQLMessage = "%s runs a statement the data-access library did not generate. Add the statement to the subsection the UI's import list admits (ADR-0071, ADR-0081)"

// shape is how a statement runner's parameters open, each type printed with aliases replaced, and
// the type of its first result when its name and parameters alone could belong to anything else.
type shape struct {
	params []string
	result string
}

const (
	ctxType    = "context.Context"
	pgxPath    = "github.com/jackc/pgx/v5"
	pgconnPath = "github.com/jackc/pgx/v5/pgconn"
)

// statementRunners are the methods that run a statement, or start a batch or a pipeline that does,
// by name, each with the shapes its parameters may open with: the statement's text, a batch, the
// table rows are copied into, or the reader or writer a copy runs over beside its statement. The
// driver's connection, pool and transaction and its lower-level connection declare them, and any
// type declared with the same names and parameters is matched alike.
var statementRunners = map[string][]shape{
	"Query":         {{params: []string{ctxType, "string"}}},
	"QueryRow":      {{params: []string{ctxType, "string"}}},
	"Exec":          {{params: []string{ctxType, "string"}}},
	"Prepare":       {{params: []string{ctxType, "string", "string"}}},
	"ExecParams":    {{params: []string{ctxType, "string"}}},
	"SendBatch":     {{params: []string{ctxType, "*" + pgxPath + ".Batch"}}},
	"ExecBatch":     {{params: []string{ctxType, "*" + pgconnPath + ".Batch"}}},
	"CopyFrom":      {{params: []string{ctxType, pgxPath + ".Identifier"}}, {params: []string{ctxType, "io.Reader", "string"}}},
	"CopyTo":        {{params: []string{ctxType, "io.Writer", "string"}}},
	"StartPipeline": {{params: []string{ctxType}, result: "*" + pgconnPath + ".Pipeline"}},
}

func runRawSQL(pass *analysis.Pass) (any, error) {
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
		ast.Inspect(file, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
			if ok && statementRunner(fn) {
				pass.Reportf(id.Pos(), rawSQLMessage, "The method "+fn.Name())
			}
			return true
		})
	}
	return nil, nil
}

// statementRunner reports whether fn is a method named like one that runs a statement whose
// parameters open with one of that name's shapes, and whose first result is the shape's result when
// it names one.
func statementRunner(fn *types.Func) bool {
	sig := fn.Signature()
	if sig.Recv() == nil {
		return false
	}
	for _, sh := range statementRunners[fn.Name()] {
		if matches(sig, sh) {
			return true
		}
	}
	return false
}

func matches(sig *types.Signature, sh shape) bool {
	if sig.Params().Len() < len(sh.params) {
		return false
	}
	for i, want := range sh.params {
		if unaliased(sig.Params().At(i).Type()) != want {
			return false
		}
	}
	return sh.result == "" || sig.Results().Len() > 0 && unaliased(sig.Results().At(0).Type()) == sh.result
}
