package hunkmatch_test

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/mutproof/internal/hunkmatch"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// requireAgreesWithGit requires the package to give every hunk of the patch one place, and the files
// it reads the patch as leaving to be the files git apply leaves, byte for byte.
func requireAgreesWithGit(t *testing.T, root, patch string) {
	t.Helper()
	if err := hunkmatch.Check(root, []byte(patch)); err != nil {
		t.Fatalf("the patch was refused: %v", err)
	}
	_, files, err := hunkmatch.Place(root, []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	got := gitApplied(t, root, patch, slices.Sorted(maps.Keys(files)))
	if diff := cmp.Diff(got, files, compare.Options); diff != "" {
		t.Errorf("the files git apply leaves (-) and those the package reads the patch as leaving (+):\n%s", diff)
	}
}

// requireRefused requires git apply --check to accept the patch and the package to refuse it, naming
// the lines given.
func requireRefused(t *testing.T, root, patch, want string) {
	t.Helper()
	requireGitApplies(t, root, patch)
	if err := hunkmatch.Check(root, []byte(patch)); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("got %v, want a refusal holding %q", err, want)
	}
}

func TestHunkMatchRefusesAHunkWhoseContextRepeats(t *testing.T) {
	root := scratchRepo(t, map[string]string{"f.txt": twoBlocks})
	patch := diffOf(t, root, 1, map[string]string{"f.txt": strings.Replace(twoBlocks, "same 2\n", "mutated\n", 1)})
	requireRefused(t, root, patch, "f.txt: hunk 1 (@@ -11,3 +11,3 @@")
	requireRefused(t, root, patch, "matches at lines 11, 21")
}

// TestHunkMatchRefusesARepeatAtTheHeaderLine moves the second match to the line the hunk's header
// names, where git apply takes it with no offset at all.
func TestHunkMatchRefusesARepeatAtTheHeaderLine(t *testing.T) {
	root := scratchRepo(t, map[string]string{"f.txt": twoBlocks})
	patch := diffOf(t, root, 1, map[string]string{"f.txt": strings.Replace(twoBlocks, "same 2\n", "mutated\n", 1)})
	writeFile(t, root+"/f.txt", strings.Replace(twoBlocks, lines(10, func(i int) string { return "line " + strconv.Itoa(i) }), "", 1))
	requireRefused(t, root, patch, "matches at lines 1, 11")
}

// TestHunkMatchAcceptsAUniqueMatchAtAnOffset moves the hunk's only match 5 lines below its header
// line, as an edit above it does.
func TestHunkMatchAcceptsAUniqueMatchAtAnOffset(t *testing.T) {
	root := scratchRepo(t, map[string]string{"f.txt": twoBlocks})
	patch := diffOf(t, root, 3, map[string]string{"f.txt": strings.Replace(twoBlocks, "same 2\n", "mutated\n", 1)})
	writeFile(t, root+"/f.txt", "new 1\nnew 2\nnew 3\nnew 4\nnew 5\n"+twoBlocks)
	requireAgreesWithGit(t, root, patch)
}

// TestHunkMatchCountsByGitsAnchors uses hunks whose context repeats elsewhere in the file but which
// git apply holds to the start or the end of the file, so each has one place.
func TestHunkMatchCountsByGitsAnchors(t *testing.T) {
	file := "a\nb\nc\nd\na\nb\nc\nd\n"
	cases := map[string]string{
		"start": "a\nB\nc\nd\na\nb\nc\nd\n",
		"end":   "a\nb\nc\nd\na\nb\nc\nD\n",
	}
	for name, edited := range cases {
		t.Run(name, func(t *testing.T) {
			root := scratchRepo(t, map[string]string{"f.txt": file})
			patch := diffOf(t, root, 2, map[string]string{"f.txt": edited})
			requireAgreesWithGit(t, root, patch)
		})
	}
}

// TestHunkMatchReadsTheFileAsEarlierHunksLeaveIt uses a patch whose first hunk removes one copy of
// a repeated block and whose second hunk edits the other copy, so the second hunk matches twice in
// the file as it stands but once after the first hunk, where git apply matches it.
func TestHunkMatchReadsTheFileAsEarlierHunksLeaveIt(t *testing.T) {
	root := scratchRepo(t, map[string]string{"f.txt": twoBlocks})
	edited := strings.Replace(twoBlocks, "line 10\nsame 1\nsame 2\nsame 3\n", "line 10\n", 1)
	edited = strings.Replace(edited, "same 2\n", "mutated\n", 1)
	patch := diffOf(t, root, 1, map[string]string{"f.txt": edited})
	if strings.Count(patch, "@@ -") != 2 {
		t.Fatalf("the patch does not hold two hunks:\n%s", patch)
	}
	requireAgreesWithGit(t, root, patch)
}

// TestHunkMatchNeverMatchesOverAnEarlierHunk uses a second hunk whose preimage also matches the
// lines the first hunk writes, and whose header names those lines. git apply never matches a hunk
// over lines an earlier hunk wrote, so the second hunk has one place, the file's own copy further
// down.
func TestHunkMatchNeverMatchesOverAnEarlierHunk(t *testing.T) {
	file := strings.Replace(lines(20, func(i int) string { return "line " + strconv.Itoa(i) }), "line 16\nline 17\nline 18\n", "x\ny\nz\n", 1)
	root := scratchRepo(t, map[string]string{"f.txt": file})
	patch := "control: a control\n\n" +
		"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n" +
		"@@ -2,3 +2,5 @@\n line 2\n-line 3\n+x\n+y\n+z\n line 4\n" +
		"@@ -16,3 +3,3 @@\n x\n-y\n+Y\n z\n"
	requireGitApplies(t, root, patch)
	requireAgreesWithGit(t, root, patch)
}

// TestHunkMatchComparesLineEndings uses a hunk with no anchor whose preimage, with its line endings
// ignored, also matches a block of lines ending in a carriage return. git compares the line endings,
// so the hunk has one place.
func TestHunkMatchComparesLineEndings(t *testing.T) {
	file := "x\ny\na\r\nb\r\nc\r\nz\na\nb\nc\nw\n"
	root := scratchRepo(t, map[string]string{"f.txt": file})
	patch := diffOf(t, root, 1, map[string]string{"f.txt": "x\ny\na\r\nb\r\nc\r\nz\na\nB\nc\nw\n"})
	if !strings.Contains(patch, "@@ -7,3 +7,3 @@") {
		t.Fatalf("the patch is not one unanchored hunk at line 7:\n%s", patch)
	}
	requireAgreesWithGit(t, root, patch)
}

// TestHunkMatchComparesALastLineWithoutANewlineAsAPrefix uses a hunk with no anchor whose preimage
// ends in the file's last line, which has no newline. git compares that line as a prefix, so it also
// matches the same text followed by whitespace and a newline earlier in the file, and the hunk has
// two places. Once lines are added above them, git puts the hunk on the first copy, which its author
// did not mean.
func TestHunkMatchComparesALastLineWithoutANewlineAsAPrefix(t *testing.T) {
	cases := map[string]string{
		"a newline":                             "a\nb\nc\na\nb\nc",
		"spaces, a tab, a return and a newline": "a\nb\nc \t\r\na\nb\nc",
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			root := scratchRepo(t, map[string]string{"f.txt": file})
			edited := file[:strings.LastIndex(file, "b")] + "B" + file[strings.LastIndex(file, "b")+1:]
			patch := diffOf(t, root, 1, map[string]string{"f.txt": edited})
			if !strings.Contains(patch, "@@ -4,3 +4,3 @@") || !strings.Contains(patch, `\ No newline at end of file`) {
				t.Fatalf("the patch is not one unanchored hunk at line 4 ending without a newline:\n%s", patch)
			}
			requireRefused(t, root, patch, "matches at lines 1, 4")

			writeFile(t, root+"/f.txt", "p\nq\nr\n"+file)
			requireRefused(t, root, patch, "matches at lines 4, 7")
			hunks, files, err := hunkmatch.Place(root, []byte(patch))
			if err != nil {
				t.Fatal(err)
			}
			if hunks[0].Chosen != 4 {
				t.Errorf("the package reads git apply as taking line %d, want 4, the copy the author did not mean", hunks[0].Chosen)
			}
			got := gitApplied(t, root, patch, []string{"f.txt"})
			if diff := cmp.Diff(got, files, compare.Options); diff != "" {
				t.Errorf("the files git apply leaves (-) and those the package reads the patch as leaving (+):\n%s", diff)
			}
		})
	}
}

// TestHunkMatchFindsALastLineWithoutANewlineBeforeOtherLines adds a line after the file's last line,
// which had no newline. git compares the hunk's last line as a prefix, so the hunk still matches
// once.
func TestHunkMatchFindsALastLineWithoutANewlineBeforeOtherLines(t *testing.T) {
	root := scratchRepo(t, map[string]string{"f.txt": "x\na\nb\nc"})
	patch := diffOf(t, root, 1, map[string]string{"f.txt": "x\na\nB\nc"})
	if !strings.Contains(patch, "@@ -2,3 +2,3 @@") || !strings.Contains(patch, `\ No newline at end of file`) {
		t.Fatalf("the patch is not one unanchored hunk at line 2 ending without a newline:\n%s", patch)
	}
	writeFile(t, root+"/f.txt", "x\na\nb\nc\nd\n")
	requireGitApplies(t, root, patch)
	requireAgreesWithGit(t, root, patch)
}

// TestHunkMatchHoldsAnEndAnchoredLastLineExactly uses a hunk that must match at the end of the file,
// whose last preimage line has no newline, against a file whose last line carries a trailing space.
// git compares such a line exactly, so neither git apply nor the package finds a place for it.
func TestHunkMatchHoldsAnEndAnchoredLastLineExactly(t *testing.T) {
	root := scratchRepo(t, map[string]string{"f.txt": "a\nb\nc "})
	patch := "control: a control\n\n" +
		"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n" +
		"@@ -2,2 +2,2 @@\n b\n-c\n\\ No newline at end of file\n+C\n\\ No newline at end of file\n"
	path := t.TempDir() + "/p.patch"
	writeFile(t, path, patch)
	if out, err := runGit(root, "apply", "--check", path); err == nil {
		t.Fatalf("git apply --check accepts the patch, so the case does not show the exact comparison:\n%s", out)
	}
	hunks, _, err := hunkmatch.Place(root, []byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 1 || len(hunks[0].Matches) != 0 {
		t.Errorf("the package finds %+v, want one hunk with no match", hunks)
	}
}

func TestHunkMatchFollowsNewDeletedAndRenamedFiles(t *testing.T) {
	root := scratchRepo(t, map[string]string{"old.txt": twoBlocks, "gone.txt": "gone\n"})
	git(t, root, "mv", "old.txt", "new.txt")
	writeFile(t, root+"/new.txt", strings.Replace(twoBlocks, "line 25\n", "mutated\n", 1))
	writeFile(t, root+"/added.txt", "added\n")
	git(t, root, "add", "--all")
	git(t, root, "rm", "--quiet", "gone.txt")
	out := git(t, root, "diff", "--cached", "-M", "-U1")
	git(t, root, "reset", "--quiet", "--hard")
	git(t, root, "clean", "--quiet", "-fd")
	patch := "control: a control\n\n" + out
	for _, want := range []string{"rename from old.txt", "new file mode", "deleted file mode"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("the patch lacks %q:\n%s", want, patch)
		}
	}
	requireAgreesWithGit(t, root, patch)

	// The renamed file's hunk is read against old.txt, where its context repeats.
	git(t, root, "mv", "old.txt", "new.txt")
	writeFile(t, root+"/new.txt", strings.Replace(twoBlocks, "same 2\n", "mutated\n", 1))
	git(t, root, "add", "--all")
	renamed := "control: a control\n\n" + git(t, root, "diff", "--cached", "-M", "-U1")
	git(t, root, "reset", "--quiet", "--hard")
	requireRefused(t, root, renamed, "old.txt: hunk 1")
}

func TestHunkMatchIgnoresThePreamble(t *testing.T) {
	root := scratchRepo(t, map[string]string{"f.txt": twoBlocks})
	patch := diffOf(t, root, 3, map[string]string{"f.txt": strings.Replace(twoBlocks, "line 5\n", "mutated\n", 1)})
	patch = "removes: the line after this one is no file header\n--- a/f.txt\n" + patch
	requireAgreesWithGit(t, root, patch)
}

func TestHunkMatchRefusesWhatItCannotRead(t *testing.T) {
	root := scratchRepo(t, map[string]string{"f.txt": "a\nb\n"})
	cases := map[string]struct{ patch, want string }{
		"binary": {
			"diff --git a/f.bin b/f.bin\nGIT binary patch\nliteral 1\nIcmZpc00001\n\n",
			"a binary patch cannot be checked",
		},
		"short hunk": {
			"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,2 +1,2 @@\n-a\n+A\n",
			"fewer lines than its header counts",
		},
		"stray line": {
			"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,2 +1,2 @@\n-a\n+A\n?b\n",
			"is not a context, removed or added line",
		},
		"missing file": {
			"diff --git a/g.txt b/g.txt\n--- a/g.txt\n+++ b/g.txt\n@@ -1,2 +1,2 @@\n-a\n+A\n b\n",
			"g.txt, which the patch changes, does not exist",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := hunkmatch.Check(root, []byte(c.patch)); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
		})
	}
}
