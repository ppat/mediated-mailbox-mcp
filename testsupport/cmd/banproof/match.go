package main

// match pairs findings with wants. A finding and a want pair when they sit on the same file and
// line, name the same tool, and the want's pattern matches the finding's text. Each is used at most
// once. It returns the wants no finding satisfied and the findings no want expected.
//
// Pairing is a maximum bipartite matching rather than first come first served, so two wants on one
// line whose patterns both match one of two findings cannot starve each other.
func match(wants []want, findings []finding) (unmet []want, unexpected []finding) {
	type key struct {
		file string
		line int
	}
	wantsAt := map[key][]int{}
	for i, w := range wants {
		k := key{w.file, w.line}
		wantsAt[k] = append(wantsAt[k], i)
	}
	pairedWant := make([]int, len(wants)) // index of the finding paired with each want, or -1
	for i := range pairedWant {
		pairedWant[i] = -1
	}
	fits := func(w want, f finding) bool { return w.tool == f.tool && w.re.MatchString(f.text) }

	var augment func(fi int, seen map[int]bool) bool
	augment = func(fi int, seen map[int]bool) bool {
		f := findings[fi]
		for _, wi := range wantsAt[key{f.file, f.line}] {
			if seen[wi] || !fits(wants[wi], f) {
				continue
			}
			seen[wi] = true
			if pairedWant[wi] < 0 || augment(pairedWant[wi], seen) {
				pairedWant[wi] = fi
				return true
			}
		}
		return false
	}
	paired := make([]bool, len(findings))
	for fi := range findings {
		paired[fi] = augment(fi, map[int]bool{})
	}
	for fi, ok := range paired {
		if !ok {
			unexpected = append(unexpected, findings[fi])
		}
	}
	for wi, fi := range pairedWant {
		if fi < 0 {
			unmet = append(unmet, wants[wi])
		}
	}
	return unmet, unexpected
}
