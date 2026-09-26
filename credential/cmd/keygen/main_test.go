package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
)

// F6's part of VERIFICATIONS' row for the key-generation command. Its main function writes a key pair
// into a temporary directory, credential/seal seals a value to the public key file and
// credential/open opens it with a keyring loaded from both files (ADR-0088).
func TestTheCommandWritesAPairTheLibrarySealsToAndOpensWith(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "credential.key"), filepath.Join(dir, "credential.pub")
	args := os.Args
	t.Cleanup(func() { os.Args = args })
	os.Args = []string{"keygen", "-private-key-file", privatePath, "-public-key-file", publicPath}

	main()

	public, err := seal.LoadPublicKey(publicPath)
	if err != nil {
		t.Fatalf("loading the public key the command wrote: %v", err)
	}
	c := seal.AccountCredential("personal")
	sealed, err := public.Seal([]byte("refresh-token"), c)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := open.Load(publicPath, privatePath)
	if err != nil {
		t.Fatalf("loading the keyring from the files the command wrote: %v", err)
	}
	if got, err := ring.Open(sealed, c); err != nil || string(got) != "refresh-token" {
		t.Errorf("opened %q, %v, want the value sealed", got, err)
	}
}

// The command prints the identifier of the key it wrote, and refuses to overwrite a key file, so a
// key in use is never replaced by accident.
func TestTheCommandPrintsTheKeyAndOverwritesNothing(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "credential.key"), filepath.Join(dir, "credential.pub")
	var out bytes.Buffer
	if err := run([]string{"-private-key-file", privatePath, "-public-key-file", publicPath}, &out); err != nil {
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
		if err := run(args, &out); err == nil {
			t.Errorf("%s: the command succeeded", name)
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
