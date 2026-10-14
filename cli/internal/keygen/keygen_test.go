package keygen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/cli/internal/keygen"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// The subcommand prints the identifier of the key it wrote, and refuses to overwrite a key file, so a
// key in use is never replaced by accident.
func TestTheCommandPrintsTheKeyAndOverwritesNothing(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "credential.key"), filepath.Join(dir, "credential.pub")
	var out bytes.Buffer
	if err := keygen.Run([]string{"-private-key-file", privatePath, "-public-key-file", publicPath}, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	public, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := seal.IDOf(public).String() + "\n"; out.String() != want {
		t.Errorf("printed %q, want %q", out.String(), want)
	}
	seed, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}

	for name, args := range map[string][]string{
		"an existing private key file": {"-private-key-file", privatePath, "-public-key-file", filepath.Join(dir, "new.pub")},
		"an existing public key file":  {"-private-key-file", filepath.Join(dir, "new.key"), "-public-key-file", publicPath},
		"no public key file named":     {"-private-key-file", filepath.Join(dir, "other.key")},
	} {
		if err := keygen.Run(args, &out); err == nil {
			t.Errorf("%s: the subcommand succeeded", name)
		}
	}
	if got, err := os.ReadFile(privatePath); err != nil || !bytes.Equal(got, seed) {
		t.Errorf("the private key file changed")
	}
	for _, name := range []string{"new.key", "new.pub", "other.key"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s was left behind: %v", name, err)
		}
	}
}

// -version prints the release version and writes no key, even beside the key file flags.
func TestTheVersionFlagPrintsTheReleaseVersionAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "credential.key"), filepath.Join(dir, "credential.pub")
	for _, flag := range []string{"-version", "--version"} {
		var out bytes.Buffer
		if err := keygen.Run([]string{flag, "-private-key-file", privatePath, "-public-key-file", publicPath}, &out); err != nil {
			t.Fatalf("run %s: %v", flag, err)
		}
		if want := version.Number + "\n"; out.String() != want {
			t.Errorf("%s printed %q, want %q", flag, out.String(), want)
		}
	}
	for _, path := range []string{privatePath, publicPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s was written: %v", path, err)
		}
	}
}
