package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// report says what the demonstration found.
func (r result) report() string {
	var w strings.Builder
	verdict := "PASS"
	if !r.held() {
		verdict = "FAIL"
	}
	checks := r.checks
	if checks == "" {
		checks = "unset, so rapid's default"
	}
	fmt.Fprintf(&w, "%s %s\n", verdict, r.patch)
	fmt.Fprintf(&w, "  control: %s\n  removes: %s\n", r.preamble.control, r.preamble.removes)
	fmt.Fprintf(&w, "  every run had RAPID_SEED=%s and RAPID_CHECKS %s\n", r.seed, checks)
	if r.surviving() {
		fmt.Fprintln(&w, "  surviving mutant: every test stayed green with the mechanism removed, so the tests are vacuous or the mechanism is redundant")
	}
	for _, id := range r.red {
		fmt.Fprintf(&w, "  went red: %s\n", id)
	}
	for _, pkg := range slices.Sorted(maps.Keys(r.broken)) {
		fmt.Fprintf(&w, "  package %s failed without a failing test, so the patch broke it rather than removing a mechanism\n%s", pkg, indent(r.broken[pkg]))
	}
	for _, name := range r.preamble.tests {
		state := "went red"
		if slices.Contains(r.stayedGreen, name) {
			state = "did not go red"
		}
		fmt.Fprintf(&w, "  required %s: %s\n", name, state)
	}
	for _, path := range r.strays {
		fmt.Fprintf(&w, "  the working tree differs from before the patch at %s\n", path)
	}
	if r.notRestored != "" {
		fmt.Fprintf(&w, "  the tests were not green again after the patch was removed\n%s", indent(r.notRestored))
	}
	fmt.Fprintln(&w)
	return w.String()
}

// ledgerEntry is one patch given in the run. The control is empty when the patch's preamble could
// not be read, and the result is nil when the patch could not be judged or never ran.
type ledgerEntry struct {
	control string
	res     *result
}

// ledgerRows returns one row of the table in docs/MUTATIONS.md per control, in the order the
// controls first appear. A row names each removal of the control with the tests it turned red, and
// a surviving mutant marks the row open, as the ledger keeps it until the tests are fixed. A control
// gets a row only when every one of its patches held or survived. The others are returned as
// incomplete. The evidence pointer is left for the author to fill in.
func ledgerRows(entries []ledgerEntry) (rows, incomplete []string) {
	var controls []string
	byControl := map[string][]ledgerEntry{}
	for _, e := range entries {
		if e.control == "" {
			continue
		}
		if _, seen := byControl[e.control]; !seen {
			controls = append(controls, e.control)
		}
		byControl[e.control] = append(byControl[e.control], e)
	}
	for _, c := range controls {
		es := byControl[c]
		if slices.ContainsFunc(es, func(e ledgerEntry) bool { return e.res == nil || (!e.res.held() && !e.res.surviving()) }) {
			incomplete = append(incomplete, c)
			continue
		}
		var removals, reds []string
		open := false
		for i, e := range es {
			r := e.res
			number := ""
			if len(es) > 1 {
				number = fmt.Sprintf("(%d) ", i+1)
			}
			removals = append(removals, number+cell(r.preamble.removes))
			if r.surviving() {
				open = true
				reds = append(reds, number+"none, a surviving mutant")
				continue
			}
			reds = append(reds, number+redTests(r.red))
		}
		date := es[0].res.date.Format("2006-01-02") + " · EVIDENCE"
		if open {
			date += " · open, a surviving mutant"
		}
		rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s |", cell(c), strings.Join(removals, "<br>"), strings.Join(reds, "<br>"), date))
	}
	return rows, incomplete
}

// redTests names the tests that went red, grouped by package.
func redTests(red []testID) string {
	byPackage := map[string][]string{}
	for _, id := range red {
		byPackage[id.pkg] = append(byPackage[id.pkg], "`"+cell(id.name)+"`")
	}
	var parts []string
	for _, pkg := range slices.Sorted(maps.Keys(byPackage)) {
		parts = append(parts, strings.Join(byPackage[pkg], ", ")+" in `"+cell(pkg)+"`")
	}
	return strings.Join(parts, ", ")
}

// cell escapes the one character that ends a table cell.
func cell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

func indent(s string) string {
	lines := strings.SplitAfter(s, "\n")
	var b strings.Builder
	for _, line := range lines {
		if line != "" {
			b.WriteString("    " + line)
		}
	}
	if !strings.HasSuffix(b.String(), "\n") && b.Len() > 0 {
		b.WriteString("\n")
	}
	return b.String()
}
