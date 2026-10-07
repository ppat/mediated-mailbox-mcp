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
	fmt.Fprintf(&w, "  both runs had RAPID_SEED=%s and RAPID_CHECKS %s\n", r.seed, checks)
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
	fmt.Fprintln(&w)
	return w.String()
}

// ledgerEntry is one patch given in the run. The control is empty when the patch's preamble could
// not be read, and the result is nil when the patch could not be judged or never ran.
type ledgerEntry struct {
	control string
	res     *result
}

// ledgerRows returns one row of the ledger docs/MUTATIONS.md defines per control, in the order the
// controls first appear, each in the form that file states: a section headed by the control, a line
// for the date and evidence, and under each removal of the control a line per package naming the
// tests it turned red. A surviving mutant marks the row open, as the ledger keeps it until the tests
// are fixed or the mechanism is deleted as redundant. A control gets a row only when every one of
// its patches held or survived. The others are returned as incomplete. The evidence pointer is left
// for the author to fill in.
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
		var breaks []string
		open := false
		for i, e := range es {
			r := e.res
			label := "Break"
			if len(es) > 1 {
				label = fmt.Sprintf("Break (%d)", i+1)
			}
			lines := []string{fmt.Sprintf("- **%s:** %s", label, r.preamble.removes)}
			if r.surviving() {
				open = true
				lines = append(lines, "  - **Went red:** none, a surviving mutant")
			} else {
				lines = append(lines, redTests(r.red)...)
			}
			breaks = append(breaks, strings.Join(lines, "\n"))
		}
		date := es[0].res.date.Format("2006-01-02") + " · EVIDENCE"
		if open {
			date += " · open, a surviving mutant"
		}
		rows = append(rows, fmt.Sprintf("## %s\n\n- **Date · evidence:** %s\n%s", c, date, strings.Join(breaks, "\n")))
	}
	return rows, incomplete
}

// redTests names the tests that went red, one line per package.
func redTests(red []testID) []string {
	byPackage := map[string][]string{}
	for _, id := range red {
		byPackage[id.pkg] = append(byPackage[id.pkg], "`"+id.name+"`")
	}
	var lines []string
	for _, pkg := range slices.Sorted(maps.Keys(byPackage)) {
		lines = append(lines, "  - **Went red in `"+pkg+"`:** "+strings.Join(byPackage[pkg], ", "))
	}
	return lines
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
