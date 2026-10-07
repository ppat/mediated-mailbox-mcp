package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
)

// releasedDir names the directory holding a key pair the key-generation binary attached to a release
// wrote. Only the chainsaw workflow's check of a release's binary sets it
// (.github/scripts/chainsaw/keygen.sh), which reads this test's verdict from its verbose output, so
// any other run skips the test.
const releasedDir = "MEDIATED_MAILBOX_RELEASED_KEY_DIR"

// R1's part of VERIFICATIONS' row for the key-generation command. The pair the released binary wrote
// is one credential/seal seals to and credential/open opens with, so a deployment set up from the
// published binary alone comes up (ADR-0088).
func TestAReleasedPairSealsAndOpens(t *testing.T) {
	dir := os.Getenv(releasedDir)
	if dir == "" {
		t.Skipf("%s is unset. The chainsaw workflow's check of a release's key-generation binary sets it", releasedDir)
	}
	privatePath, publicPath := filepath.Join(dir, "credential.key"), filepath.Join(dir, "credential.pub")
	public, err := seal.LoadPublicKey(publicPath)
	if err != nil {
		t.Fatalf("loading the public key the released binary wrote: %v", err)
	}
	c := seal.AccountCredential("released")
	sealed, err := public.Seal([]byte("refresh-token"), c)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := open.Load(publicPath, privatePath)
	if err != nil {
		t.Fatalf("loading the keyring from the files the released binary wrote: %v", err)
	}
	if got, err := ring.Open(sealed, c); err != nil || string(got) != "refresh-token" {
		t.Errorf("opened %q, %v, want the value sealed", got, err)
	}
}
