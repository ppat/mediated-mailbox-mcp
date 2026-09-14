package check_test

import (
	"fmt"
	"slices"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
)

// The checks over statement files (ADR-0066). Each name is what a violation file's want annotation
// states.
const (
	checkStar       = "star-select"
	checkGroupCase  = "grouping-case-parameter"
	checkCitextCast = "citext-parameter-cast"
	checkPagedSort  = "paged-sort-identity"
	checkAccount    = "account-predicate"
	checkSuppress   = "suppression-annotation"
)

var statementChecks = []string{checkStar, checkGroupCase, checkCitextCast, checkPagedSort, checkAccount, checkSuppress}

// suppressionAnnotation is sqlc's per-query switch that turns its rule surface off. ADR-0066 bans it
// from statement files, and it cannot reach this pass.
const suppressionAnnotation = "@sqlc-vet-disable"

// checkStatements runs every statement-file check over one parsed file.
func checkStatements(s schema, f sqlFile) []finding {
	var out []finding
	for i, line := range strings.Split(f.src, "\n") {
		if strings.Contains(line, suppressionAnnotation) {
			out = append(out, finding{file: f.path, line: i + 1, check: checkSuppress, text: "the generator's suppression annotation is banned from statement files"})
		}
	}
	for _, raw := range f.stmts {
		c := &statementCheck{file: f, raw: raw, schema: s}
		c.starSelects()
		c.groupingCases()
		c.query(raw.GetStmt(), nil)
		c.resolveObligations()
		out = append(out, c.findings...)
	}
	return out
}

type statementCheck struct {
	file     sqlFile
	raw      *pg.RawStmt
	schema   schema
	findings []finding
	// obligations are the account-keyed table references that need an account predicate. They are
	// resolved once the whole statement is read, because a subquery can tie its table to a table of
	// the query around it.
	obligations []*rangeRef
}

func (c *statementCheck) report(location int32, check, format string, args ...any) {
	c.findings = append(c.findings, c.file.at(c.raw, location, check, format, args...))
}

// starSelects refuses any star, bare or qualified, anywhere in the statement.
func (c *statementCheck) starSelects() {
	walk(c.raw.GetStmt(), func(n *pg.Node) bool {
		if ref := n.GetColumnRef(); ref != nil && slices.ContainsFunc(ref.GetFields(), func(f *pg.Node) bool { return f.GetAStar() != nil }) {
			c.report(ref.GetLocation(), checkStar, "a star select gains every column a future migration adds")
		}
		return true
	})
}

// groupingCases refuses a case expression over a parameter in any grouping position, including one
// reached through an ordinal or an output column name.
func (c *statementCheck) groupingCases() {
	walk(c.raw.GetStmt(), func(n *pg.Node) bool {
		sel := n.GetSelectStmt()
		if sel == nil {
			return true
		}
		for _, item := range sel.GetGroupClause() {
			walk(outputExpression(item, sel.GetTargetList()), func(g *pg.Node) bool {
				if ce := g.GetCaseExpr(); ce != nil && containsParameter(g) {
					c.report(ce.GetLocation(), checkGroupCase, "a case expression over a parameter in a grouping position lets an undeclared dimension reach the database")
					return false
				}
				return g.GetSelectStmt() == nil
			})
		}
		return true
	})
}

// outputExpression resolves a grouping or sort item written as an ordinal or as an output column's
// name to the expression it stands for.
func outputExpression(item *pg.Node, targets []*pg.Node) *pg.Node {
	if k := item.GetAConst(); k != nil && k.GetIval() != nil {
		if i := int(k.GetIval().GetIval()); i >= 1 && i <= len(targets) {
			return targets[i-1].GetResTarget().GetVal()
		}
	}
	if ref := item.GetColumnRef(); ref != nil && len(ref.GetFields()) == 1 {
		name := names(ref.GetFields())
		for _, target := range targets {
			if rt := target.GetResTarget(); len(name) == 1 && rt.GetName() == name[0] {
				return rt.GetVal()
			}
		}
	}
	return item
}

// rangeRef is one table reference in a FROM list, a join, or an insert, update or delete target.
type rangeRef struct {
	name     string
	alias    string
	table    *table // nil when the reference is not a table of the schema, such as a common table expression
	location int32
	// connected is set once the reference is known to be tied to an account parameter.
	connected bool
	// through lists references of enclosing queries this one is tied to by equality.
	through []*rangeRef
	// insertValue is set for an insert whose account value comes from a column of a query.
	insertValue *rangeRef
}

type scope struct {
	refs  []*rangeRef
	ctes  []string
	outer *scope
}

func (s *scope) isCTE(name string) bool {
	for ; s != nil; s = s.outer {
		if slices.Contains(s.ctes, name) {
			return true
		}
	}
	return false
}

// resolve finds the reference a column reference names, searching enclosing queries outward. known
// is false when the column may belong to a reference whose columns the schema does not describe.
func (s *scope) resolve(ref *pg.ColumnRef) (r *rangeRef, col string, known bool) {
	fields := names(ref.GetFields())
	if len(fields) == 0 || len(fields) != len(ref.GetFields()) {
		return nil, "", false
	}
	col = fields[len(fields)-1]
	for level := s; level != nil; level = level.outer {
		if len(fields) >= 2 {
			q := fields[len(fields)-2]
			for _, candidate := range level.refs {
				if candidate.alias == q || (candidate.alias == "" && candidate.name == q) {
					return candidate, col, candidate.table != nil
				}
			}
			continue
		}
		var matches []*rangeRef
		opaque := false
		for _, candidate := range level.refs {
			if candidate.table == nil {
				opaque = true
			} else if _, ok := candidate.table.columns[col]; ok {
				matches = append(matches, candidate)
			}
		}
		switch {
		case len(matches) == 1 && !opaque:
			return matches[0], col, true
		case len(matches) > 0 || opaque:
			return nil, col, false
		}
	}
	return nil, col, false
}

// addFrom adds the references of a FROM list item to the scope and returns the join conditions it
// carries.
func (c *statementCheck) addFrom(sc *scope, item *pg.Node, outer *scope) []*pg.Node {
	switch {
	case item.GetRangeVar() != nil:
		sc.refs = append(sc.refs, c.rangeVar(sc, item.GetRangeVar()))
	case item.GetJoinExpr() != nil:
		j := item.GetJoinExpr()
		conds := c.addFrom(sc, j.GetLarg(), outer)
		conds = append(conds, c.addFrom(sc, j.GetRarg(), outer)...)
		if j.GetQuals() != nil {
			conds = append(conds, j.GetQuals())
		}
		return conds
	case item.GetRangeSubselect() != nil:
		rs := item.GetRangeSubselect()
		c.query(rs.GetSubquery(), outer)
		sc.refs = append(sc.refs, &rangeRef{alias: rs.GetAlias().GetAliasname()})
	default:
		alias := ""
		walk(item, func(n *pg.Node) bool {
			if a := n.GetAlias(); a != nil && alias == "" {
				alias = a.GetAliasname()
			}
			return true
		})
		sc.refs = append(sc.refs, &rangeRef{alias: alias})
	}
	return nil
}

func (c *statementCheck) rangeVar(sc *scope, rv *pg.RangeVar) *rangeRef {
	r := &rangeRef{name: rv.GetRelname(), alias: rv.GetAlias().GetAliasname(), location: rv.GetLocation()}
	if (rv.GetSchemaname() == "" || rv.GetSchemaname() == "public") && !sc.isCTE(r.name) {
		r.table = c.schema[r.name]
	}
	if r.table != nil {
		if _, keyed := r.table.columns[accountColumn]; keyed {
			if _, exempt := predicateExceptions[r.name]; !exempt {
				c.obligations = append(c.obligations, r)
			}
		}
	}
	return r
}

// query checks one select, insert, update or delete, and every query nested in it.
func (c *statementCheck) query(n *pg.Node, outer *scope) {
	sc := &scope{outer: outer}
	var with *pg.WithClause
	var conds, exprs []*pg.Node
	switch {
	case n.GetSelectStmt() != nil:
		sel := n.GetSelectStmt()
		with = sel.GetWithClause()
		c.withClause(sc, with)
		if sel.GetOp() != pg.SetOperation_SETOP_NONE {
			c.query(&pg.Node{Node: &pg.Node_SelectStmt{SelectStmt: sel.GetLarg()}}, sc)
			c.query(&pg.Node{Node: &pg.Node_SelectStmt{SelectStmt: sel.GetRarg()}}, sc)
			if sel.GetLimitCount() != nil || sel.GetLimitOffset() != nil {
				c.report(-1, checkPagedSort, "the row identity of a paged set operation cannot be determined, so its sort cannot be shown to end with it")
			}
			return
		}
		for _, item := range sel.GetFromClause() {
			conds = append(conds, c.addFrom(sc, item, outer)...)
		}
		if sel.GetWhereClause() != nil {
			conds = append(conds, sel.GetWhereClause())
		}
		exprs = append(exprs, sel.GetTargetList()...)
		exprs = append(exprs, sel.GetGroupClause()...)
		exprs = append(exprs, sel.GetHavingClause())
		exprs = append(exprs, sel.GetSortClause()...)
		exprs = append(exprs, sel.GetValuesLists()...)
		if sel.GetLimitCount() != nil || sel.GetLimitOffset() != nil {
			c.pagedSort(sc, sel)
		}
	case n.GetInsertStmt() != nil:
		ins := n.GetInsertStmt()
		c.withClause(sc, ins.GetWithClause())
		target := c.rangeVar(sc, ins.GetRelation())
		c.insertAccount(sc, target, ins)
		exprs = append(exprs, ins.GetReturningList()...)
		if oc := ins.GetOnConflictClause(); oc != nil {
			exprs = append(exprs, oc.GetTargetList()...)
			exprs = append(exprs, oc.GetWhereClause())
		}
		sc.refs = append(sc.refs, target)
	case n.GetUpdateStmt() != nil:
		up := n.GetUpdateStmt()
		c.withClause(sc, up.GetWithClause())
		sc.refs = append(sc.refs, c.rangeVar(sc, up.GetRelation()))
		for _, item := range up.GetFromClause() {
			conds = append(conds, c.addFrom(sc, item, outer)...)
		}
		if up.GetWhereClause() != nil {
			conds = append(conds, up.GetWhereClause())
		}
		exprs = append(exprs, up.GetTargetList()...)
		exprs = append(exprs, up.GetReturningList()...)
	case n.GetDeleteStmt() != nil:
		del := n.GetDeleteStmt()
		c.withClause(sc, del.GetWithClause())
		sc.refs = append(sc.refs, c.rangeVar(sc, del.GetRelation()))
		for _, item := range del.GetUsingClause() {
			conds = append(conds, c.addFrom(sc, item, outer)...)
		}
		if del.GetWhereClause() != nil {
			conds = append(conds, del.GetWhereClause())
		}
		exprs = append(exprs, del.GetReturningList()...)
	default:
		return
	}
	c.accountEqualities(sc, conds)
	for _, e := range append(exprs, conds...) {
		c.expression(sc, e)
	}
}

func (c *statementCheck) withClause(sc *scope, with *pg.WithClause) {
	for _, cte := range with.GetCtes() {
		sc.ctes = append(sc.ctes, cte.GetCommonTableExpr().GetCtename())
	}
	for _, cte := range with.GetCtes() {
		c.query(cte.GetCommonTableExpr().GetCtequery(), sc)
	}
}

// expression checks casts compared against case-insensitive columns within one expression, and
// checks each query nested in it against the scope it sits in.
func (c *statementCheck) expression(sc *scope, e *pg.Node) {
	walk(e, func(n *pg.Node) bool {
		if n.GetSelectStmt() != nil || n.GetInsertStmt() != nil || n.GetUpdateStmt() != nil || n.GetDeleteStmt() != nil {
			c.query(n, sc)
			return false
		}
		if a := n.GetAExpr(); a != nil && a.GetLexpr() != nil {
			c.citextComparison(sc, a, a.GetLexpr(), a.GetRexpr())
			c.citextComparison(sc, a, a.GetRexpr(), a.GetLexpr())
		}
		return true
	})
}

// citextComparison refuses a cast parameter on one side of a comparison whose other side is a
// case-insensitive column, or a column whose type cannot be resolved.
func (c *statementCheck) citextComparison(sc *scope, a *pg.A_Expr, columnSide, paramSide *pg.Node) {
	ref := columnSide.GetColumnRef()
	if ref == nil {
		return
	}
	cast := isCastParameter(paramSide)
	for _, item := range paramSide.GetList().GetItems() {
		cast = cast || isCastParameter(item)
	}
	if !cast {
		return
	}
	r, col, known := sc.resolve(ref)
	switch {
	case !known:
		c.report(a.GetLocation(), checkCitextCast, "column %s is compared with a cast parameter and its type cannot be resolved, so it may be case-insensitive", col)
	case r.table.columns[col].typ == "citext":
		c.report(a.GetLocation(), checkCitextCast, "a cast parameter compared against case-insensitive column %s.%s silently changes the comparison", r.name, col)
	}
}

// accountEqualities records which references the conditions tie to an account parameter, through
// equalities joined by AND. A predicate under OR or NOT does not restrict the rows, so it is not
// followed.
func (c *statementCheck) accountEqualities(sc *scope, conds []*pg.Node) {
	var equalities [][2]*pg.Node
	var conjuncts func(n *pg.Node)
	conjuncts = func(n *pg.Node) {
		if b := n.GetBoolExpr(); b != nil && b.GetBoolop() == pg.BoolExprType_AND_EXPR {
			for _, arg := range b.GetArgs() {
				conjuncts(arg)
			}
			return
		}
		if a := n.GetAExpr(); a != nil && a.GetKind() == pg.A_Expr_Kind_AEXPR_OP && slices.Equal(names(a.GetName()), []string{"="}) && a.GetLexpr() != nil {
			equalities = append(equalities, [2]*pg.Node{a.GetLexpr(), a.GetRexpr()})
		}
	}
	for _, cond := range conds {
		conjuncts(cond)
	}
	// Tie references together until nothing changes, which settles chains of equalities.
	for changed := true; changed; {
		changed = false
		for _, eq := range equalities {
			l, lParam := c.accountSide(sc, eq[0])
			r, rParam := c.accountSide(sc, eq[1])
			switch {
			case l != nil && rParam:
				if inScope(sc, l) && !l.connected {
					l.connected, changed = true, true
				}
			case r != nil && lParam:
				if inScope(sc, r) && !r.connected {
					r.connected, changed = true, true
				}
			case l != nil && r != nil:
				if l.connected != r.connected && inScope(sc, l) && inScope(sc, r) {
					l.connected, r.connected, changed = true, true, true
				}
				if !inScope(sc, r) && !slices.Contains(l.through, r) {
					l.through = append(l.through, r)
				}
				if !inScope(sc, l) && !slices.Contains(r.through, l) {
					r.through = append(r.through, l)
				}
			}
		}
	}
}

func inScope(sc *scope, r *rangeRef) bool { return slices.Contains(sc.refs, r) }

// accountSide returns the reference whose account column n names, or whether n is a parameter.
func (c *statementCheck) accountSide(sc *scope, n *pg.Node) (*rangeRef, bool) {
	if isParameter(n) || isCastParameter(n) {
		return nil, true
	}
	if ref := n.GetColumnRef(); ref != nil {
		if r, col, known := sc.resolve(ref); known && col == accountColumn {
			return r, false
		}
	}
	return nil, false
}

// insertAccount checks that an insert into an account-keyed table supplies the account as a
// parameter, or from an account column its query ties to a parameter.
func (c *statementCheck) insertAccount(sc *scope, target *rangeRef, ins *pg.InsertStmt) {
	if target.table == nil {
		return
	}
	if _, keyed := target.table.columns[accountColumn]; !keyed {
		return
	}
	index := slices.IndexFunc(ins.GetCols(), func(n *pg.Node) bool { return n.GetResTarget().GetName() == accountColumn })
	if index < 0 {
		c.report(target.location, checkAccount, "an insert into account-keyed table %s must name the %s column", target.name, accountColumn)
		return
	}
	sel := ins.GetSelectStmt().GetSelectStmt()
	if rows := sel.GetValuesLists(); len(rows) > 0 {
		for _, row := range rows {
			items := row.GetList().GetItems()
			if index >= len(items) || !isParameter(items[index]) && !isCastParameter(items[index]) {
				c.report(target.location, checkAccount, "an insert into account-keyed table %s must take %s from a parameter", target.name, accountColumn)
			}
		}
		target.connected = true
		for _, row := range rows {
			c.expression(sc, row)
		}
		return
	}
	// An insert from a query takes its account from that query's output column.
	inner := &scope{outer: sc}
	c.withClause(inner, sel.GetWithClause())
	var conds []*pg.Node
	for _, item := range sel.GetFromClause() {
		conds = append(conds, c.addFrom(inner, item, sc)...)
	}
	if sel.GetWhereClause() != nil {
		conds = append(conds, sel.GetWhereClause())
	}
	c.accountEqualities(inner, conds)
	for _, e := range append(sel.GetTargetList(), conds...) {
		c.expression(inner, e)
	}
	targets := sel.GetTargetList()
	if index < len(targets) {
		value := targets[index].GetResTarget().GetVal()
		if r, param := c.accountSide(inner, value); param {
			target.connected = true
		} else if r != nil {
			target.insertValue = r
			return
		}
	}
	if !target.connected {
		c.report(target.location, checkAccount, "an insert into account-keyed table %s must take %s from a parameter or a tied account column", target.name, accountColumn)
		target.connected = true
	}
}

// resolveObligations reports every account-keyed reference not tied to an account parameter.
func (c *statementCheck) resolveObligations() {
	var tied func(r *rangeRef, seen []*rangeRef) bool
	tied = func(r *rangeRef, seen []*rangeRef) bool {
		if r.connected {
			return true
		}
		if slices.Contains(seen, r) {
			return false
		}
		seen = append(seen, r)
		if r.insertValue != nil && tied(r.insertValue, seen) {
			return true
		}
		return slices.ContainsFunc(r.through, func(o *rangeRef) bool { return tied(o, seen) })
	}
	for _, r := range c.obligations {
		if !tied(r, nil) {
			c.report(r.location, checkAccount, "no account predicate ties %s.%s to a parameter", r.name, accountColumn)
		}
	}
}

// pagedSort checks that a paged read's sort ends with the row's identity. The identity is the
// grouping columns of a grouped read, and otherwise the first table's primary key without the
// account column, since the account predicate already fixes that.
func (c *statementCheck) pagedSort(sc *scope, sel *pg.SelectStmt) {
	type colRef struct {
		ref *rangeRef
		col string
	}
	resolve := func(n *pg.Node) (colRef, bool) {
		ref := n.GetColumnRef()
		if ref == nil {
			return colRef{}, false
		}
		r, col, known := sc.resolve(ref)
		return colRef{r, col}, known
	}
	var identity []colRef
	if group := sel.GetGroupClause(); len(group) > 0 {
		for _, item := range group {
			cr, ok := resolve(outputExpression(item, sel.GetTargetList()))
			if !ok {
				c.report(-1, checkPagedSort, "a paged read grouped by an expression has no identity its sort can be shown to end with")
				return
			}
			identity = append(identity, cr)
		}
	} else {
		if len(sc.refs) == 0 || sc.refs[0].table == nil || len(sc.refs[0].table.primaryKey) == 0 {
			c.report(-1, checkPagedSort, "a paged read whose first table has no known primary key has no identity its sort can be shown to end with")
			return
		}
		base := sc.refs[0]
		key := slices.DeleteFunc(slices.Clone(base.table.primaryKey), func(k string) bool { return k == accountColumn })
		if len(key) == 0 {
			key = base.table.primaryKey
		}
		for _, k := range key {
			identity = append(identity, colRef{base, k})
		}
	}
	sort := sel.GetSortClause()
	if len(sort) < len(identity) {
		c.report(-1, checkPagedSort, "a paged read's sort must end with the row identity %s", describe(identity, func(cr colRef) string { return cr.col }))
		return
	}
	var trailing []colRef
	for _, item := range sort[len(sort)-len(identity):] {
		cr, ok := resolve(outputExpression(item.GetSortBy().GetNode(), sel.GetTargetList()))
		if !ok {
			break
		}
		trailing = append(trailing, cr)
	}
	for _, want := range identity {
		if !slices.Contains(trailing, want) {
			c.report(-1, checkPagedSort, "a paged read's sort must end with the row identity %s", describe(identity, func(cr colRef) string { return cr.col }))
			return
		}
	}
}

func describe[T any](items []T, name func(T) string) string {
	var parts []string
	for _, item := range items {
		parts = append(parts, name(item))
	}
	return fmt.Sprintf("(%s)", strings.Join(parts, ", "))
}
