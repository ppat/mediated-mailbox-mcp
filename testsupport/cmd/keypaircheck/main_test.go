package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
)

// writePair writes a freshly generated pair's seed and public key into dir.
func writePair(t *testing.T, dir, name string) (privatePath, publicPath string) {
	t.Helper()
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	privatePath, publicPath = filepath.Join(dir, name+".key"), filepath.Join(dir, name+".pub")
	if err := os.WriteFile(privatePath, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publicPath, key.PublicKey().Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return privatePath, publicPath
}

// A pair that belongs together passes, and a public key from another pair, a missing file and a
// missing flag each fail, so a release whose binary wrote an unusable pair fails its check.
func TestOnlyAPairTheLibrarySealsToAndOpensWithPasses(t *testing.T) {
	dir := t.TempDir()
	private, public := writePair(t, dir, "one")
	_, otherPublic := writePair(t, dir, "two")
	if err := run([]string{"-private-key-file", private, "-public-key-file", public}); err != nil {
		t.Fatalf("a matching pair failed: %v", err)
	}
	for name, args := range map[string][]string{
		"another pair's public key": {"-private-key-file", private, "-public-key-file", otherPublic},
		"a missing private key":     {"-private-key-file", filepath.Join(dir, "absent.key"), "-public-key-file", public},
		"no public key named":       {"-private-key-file", private},
	} {
		if err := run(args); err == nil {
			t.Errorf("%s passed", name)
		}
	}
}
