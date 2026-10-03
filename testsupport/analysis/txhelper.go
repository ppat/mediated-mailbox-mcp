package analysis

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

// TxHelper reports a generated data-access function that can run outside the transaction helper,
// the rule ADR-0047 sets so that every unit of data access runs in a transaction that set and
// verified the account. A generated function accepts any database handle, a pool included, and a
// transaction that never set the account sees no rows and raises nothing (db/tx).
//
// A generated subsection is recognised by type, not by a list of names. It is a package below the
// module's db directory, at any depth, whose New returns a pointer to its own Queries type, so a new
// subsection is covered with no edit. Every package is checked, test files included.
//
// db/tx.RunBase, which opens a base-policy transaction naming no account (ADR-0112), is held to the
// same rules as db/tx.Run, and each literal passed to either is one the rules below call a literal
// passed to db/tx.Run.
//
// The rules, each reported where it is broken:
//
//   - A call of a subsection's New sits inside a function literal passed as the last argument of
//     db/tx.Run, and its argument is the transaction parameter of the innermost such literal
//     around it.
//   - The base policy's subsection, db/policyrules/base, is built only from a base-policy
//     transaction's literal, and no other subsection is built from one, so the empty account is
//     never read where an account's rows were meant. A call anywhere else, on any other handle, or on the transaction of an enclosing
//     literal from inside a nested one, which set another account, is reported.
//   - The transaction parameter is never replaced. An assignment to it, a range clause assigning
//     it and taking its address, through which it could be assigned, are reported, so the
//     parameter a call names is always the transaction db/tx.Run handed the literal.
//   - A function passed to db/tx.Run that is not a function literal is reported, since which handle
//     its queries are built on cannot be seen.
//   - (*Queries).WithTx is reported everywhere, since it rebinds queries to any transaction.
//   - A subsection's New used other than by calling it, as a value, is reported.
//
// The statements that are not account-scoped run outside the transaction helper and are exempt: the
// accounts listing, the read of oauth_clients (ADR-0091), delta sync's re-seal of a client secret
// (ADR-0092), and the UI's OAuth client setup, listing each client's identity and adding, replacing and
// removing a client, which belong to no account (ADR-0084, ADR-0106). Each is exempt only when written
// as one chained call, such as accounts.New(h).Accounts(ctx) or setup.New(h).AddClient(ctx, arg),
// with any handle h. Any other use of those packages' New follows the rules above, so a statement later
// added to any of them is not exempt.
//
// The check is lexical and has one known gap. A Queries value built correctly inside the literal
// can escape it, stored in a field, sent on a channel or returned, and be used after db/tx.Run has
// returned, on a connection whose transaction ended. The analyser does not follow values, so it
// does not report that.
var TxHelper = &analysis.Analyzer{
	Name: "txhelper",
	Doc:  "reports a generated data-access function that can run outside the transaction helper",
	Run:  runTxHelper,
}

const txPath = modulePath + "/db/tx"

// txBasePath is the base policy's subsection, whose statements run only in a base-policy transaction,
// the one db/tx.RunBase opens, and the only subsection whose statements run in one (ADR-0112).
const txBasePath = modulePath + "/db/policyrules/base"

// txHelperExceptions maps each exempt subsection to its exempt statements.
var txHelperExceptions = map[string][]string{
	modulePath + "/db/accounts":            {"Accounts"},
	modulePath + "/db/oauthclients":        {"OAuthClients"},
	modulePath + "/db/oauthclients/secret": {"ReplaceSealedClient"},
	modulePath + "/db/oauthclients/setup":  {"SetupClients", "AddClient", "ReplaceClient", "RemoveClient"},
}

const (
	txOutsideMessage = "calls %s.New outside a function literal passed to tx.Run, so its statements could run in a transaction that did not set the account. Build it inside the literal from the transaction the literal is handed (ADR-0047)"
	txHandleMessage  = "calls %s.New on a handle other than the transaction tx.Run hands the enclosing literal, so its statements could run in a transaction that did not set the account. Build it from the literal's transaction (ADR-0047)"
	txValueMessage   = "uses %s.New as a value, so the handle it is called with cannot be checked. Call it inside a function literal passed to tx.Run (ADR-0047)"
	txWithTxMessage  = "rebinds %s queries with WithTx, so they can run in a transaction that did not set the account. Build them with New inside a function literal passed to tx.Run (ADR-0047)"
	txNamedMessage   = "passes tx.Run a function that is not a function literal, so the handle its queries are built on cannot be checked. Pass a function literal (ADR-0047)"
	txNestedMessage  = "calls %s.New on the transaction of an enclosing literal passed to tx.Run, not the innermost one, so its statements run for another account. Build it from the innermost literal's transaction (ADR-0047)"
	txAssignMessage  = "assigns to the transaction tx.Run hands the literal, so queries built from it could run in a transaction that did not set the account. Leave the parameter as tx.Run hands it (ADR-0047)"
	txAddressMessage = "takes the address of the transaction tx.Run hands the literal, so it could be replaced through the pointer. Leave the parameter as tx.Run hands it (ADR-0047)"
)

// The findings of the base-policy transaction's rule.
const (
	txBaseOtherMessage   = "calls %s.New in a base-policy transaction, which names no account, so its statements would read no account's rows and raise nothing. Only db/policyrules/base runs in tx.RunBase (ADR-0112)"
	txBaseAccountMessage = "calls %s.New in a transaction tx.Run opened for an account, and the base policy's statements run only in a base-policy transaction. Use tx.RunBase (ADR-0112)"
)

// generatedSubsection reports whether pkg is a generated data-access subsection.
func generatedSubsection(pkg *types.Package) bool {
	if pkg == nil {
		return false
	}
	if !strings.HasPrefix(pkg.Path(), modulePath+"/db/") {
		return false
	}
	queries, ok := pkg.Scope().Lookup("Queries").(*types.TypeName)
	if !ok {
		return false
	}
	newFunc, ok := pkg.Scope().Lookup("New").(*types.Func)
	if !ok {
		return false
	}
	results := newFunc.Signature().Results()
	if results.Len() != 1 {
		return false
	}
	ptr, ok := results.At(0).Type().(*types.Pointer)
	return ok && types.Identical(ptr.Elem(), queries.Type())
}

// isGeneratedNew reports whether fn is a generated subsection's New.
func isGeneratedNew(fn *types.Func) bool {
	return fn != nil && fn.Name() == "New" && fn.Signature().Recv() == nil && generatedSubsection(fn.Pkg())
}

// isWithTx reports whether fn is the WithTx method of a generated subsection's Queries.
func isWithTx(fn *types.Func) bool {
	return fn != nil && fn.Name() == "WithTx" && fn.Signature().Recv() != nil && generatedSubsection(fn.Pkg())
}

func runTxHelper(pass *analysis.Pass) (any, error) {
	// literals maps each transaction parameter of a function literal passed to db/tx.Run to that
	// literal.
	literals := map[types.Object]*ast.FuncLit{}
	var bodies []*ast.BlockStmt
	// bases holds the bodies of the literals passed to db/tx.RunBase, the base-policy transactions.
	bases := map[*ast.BlockStmt]bool{}
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			callee := typeutil.StaticCallee(pass.TypesInfo, call)
			baseTx := isFunc(callee, txPath, "RunBase")
			if !baseTx && !isFunc(callee, txPath, "Run") {
				return true
			}
			lit, ok := ast.Unparen(call.Args[len(call.Args)-1]).(*ast.FuncLit)
			if !ok {
				pass.Reportf(call.Args[len(call.Args)-1].Pos(), txNamedMessage)
				return true
			}
			bodies = append(bodies, lit.Body)
			if baseTx {
				bases[lit.Body] = true
			}
			for _, field := range lit.Type.Params.List {
				for _, name := range field.Names {
					if obj := pass.TypesInfo.Defs[name]; obj != nil {
						literals[obj] = lit
					}
				}
			}
			return true
		})
	}
	inside := func(pos token.Pos, body *ast.BlockStmt) bool { return body.Pos() <= pos && pos < body.End() }
	for _, file := range pass.Files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			checkReplaced(pass, n, literals)
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			fn, ok := pass.TypesInfo.Uses[id].(*types.Func)
			if !ok {
				return true
			}
			switch {
			case isWithTx(fn):
				pass.Reportf(id.Pos(), txWithTxMessage, fn.Pkg().Name())
			case isGeneratedNew(fn):
				checkNew(pass, fn, id, stack, literals, bodies, bases, inside)
			}
			return true
		})
	}
	return nil, nil
}

// checkNew checks one use of a generated subsection's New, the identifier id, the last node of stack.
func checkNew(pass *analysis.Pass, fn *types.Func, id *ast.Ident, stack []ast.Node, literals map[types.Object]*ast.FuncLit,
	bodies []*ast.BlockStmt, bases map[*ast.BlockStmt]bool, inside func(token.Pos, *ast.BlockStmt) bool,
) {
	name := fn.Pkg().Name()
	// parent returns the node i levels above the identifier, or nil.
	parent := func(i int) ast.Node {
		if len(stack)-1-i < 0 {
			return nil
		}
		return stack[len(stack)-1-i]
	}
	fun := ast.Node(id)
	level := 1
	if sel, ok := parent(1).(*ast.SelectorExpr); ok && sel.Sel == id {
		fun, level = sel, 2
	}
	call, ok := parent(level).(*ast.CallExpr)
	if !ok || call.Fun != fun {
		pass.Reportf(id.Pos(), txValueMessage, name)
		return
	}
	if statements, exempt := txHelperExceptions[fn.Pkg().Path()]; exempt {
		if sel, ok := parent(level + 1).(*ast.SelectorExpr); ok && sel.X == call && slices.Contains(statements, sel.Sel.Name) {
			if outer, ok := parent(level + 2).(*ast.CallExpr); ok && outer.Fun == sel {
				return
			}
		}
	}
	// innermost is the body of the innermost literal passed to db/tx.Run around the call.
	var innermost *ast.BlockStmt
	for _, body := range bodies {
		if inside(call.Pos(), body) && (innermost == nil || inside(body.Pos(), innermost)) {
			innermost = body
		}
	}
	if innermost == nil {
		pass.Reportf(id.Pos(), txOutsideMessage, name)
		return
	}
	if len(call.Args) == 1 {
		if arg, ok := ast.Unparen(call.Args[0]).(*ast.Ident); ok {
			if lit, ok := literals[pass.TypesInfo.Uses[arg]]; ok {
				switch base := fn.Pkg().Path() == txBasePath; {
				case lit.Body != innermost:
					pass.Reportf(id.Pos(), txNestedMessage, name)
				case bases[innermost] && !base:
					pass.Reportf(id.Pos(), txBaseOtherMessage, name)
				case !bases[innermost] && base:
					pass.Reportf(id.Pos(), txBaseAccountMessage, name)
				}
				return
			}
		}
	}
	pass.Reportf(id.Pos(), txHandleMessage, name)
}

// checkReplaced reports n when it assigns to a transaction parameter of a literal passed to
// db/tx.Run, or takes its address.
func checkReplaced(pass *analysis.Pass, n ast.Node, literals map[types.Object]*ast.FuncLit) {
	parameter := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		if !ok {
			return false
		}
		_, ok = literals[pass.TypesInfo.Uses[id]]
		return ok
	}
	switch n := n.(type) {
	case *ast.AssignStmt:
		for _, lhs := range n.Lhs {
			if parameter(lhs) {
				pass.Reportf(lhs.Pos(), txAssignMessage)
			}
		}
	case *ast.RangeStmt:
		if n.Tok != token.ASSIGN {
			return
		}
		for _, e := range []ast.Expr{n.Key, n.Value} {
			if e != nil && parameter(e) {
				pass.Reportf(e.Pos(), txAssignMessage)
			}
		}
	case *ast.UnaryExpr:
		if n.Op == token.AND && parameter(n.X) {
			pass.Reportf(n.Pos(), txAddressMessage)
		}
	}
}
