package open_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
)

// The known-answer files under testdata/knownanswer were written once by an earlier build and are
// the expectation, so they are never regenerated. A build that rewrote them would pass against
// itself and stop guarding anything. credential.key and credential.pub are a key pair the
// key-generation command wrote, in the files a deployment mounts (ADR-0088). The pair was generated
// for this test alone and seals nothing but the plaintext below, so the seed protects nothing.
// credential.sealed is knownAnswerPlaintext sealed to credential.pub under knownAnswerContext, by
// seal.LoadPublicKey and PublicKey.Seal of the build that wrote the pair, on Go 1.27.2.
const knownAnswerDir = "testdata/knownanswer"

const knownAnswerPlaintext = "a test plaintext sealed by an earlier build"

var knownAnswerContext = seal.AccountCredential("known-answer-account")

func knownAnswerFile(t *testing.T, name string) (path string, content []byte) {
	t.Helper()
	path = filepath.Join(knownAnswerDir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, content
}

// VERIFICATIONS' row for a build that cannot open a value an earlier build sealed. X-Wing's
// specification and its HPKE binding are drafts, so a Go release following a changed draft could
// stop opening the values already stored, and every value a deployment holds would be refused as
// altered (ADR-0088). This build opens the value an earlier build sealed, with the seed it wrote, to
// the plaintext it sealed.
func TestThisBuildOpensAValueAnEarlierBuildSealed(t *testing.T) {
	publicPath, _ := knownAnswerFile(t, "credential.pub")
	seedPath, _ := knownAnswerFile(t, "credential.key")
	_, sealed := knownAnswerFile(t, "credential.sealed")
	ring, err := open.Load(publicPath, seedPath)
	if err != nil {
		t.Fatalf("loading the earlier build's key pair: %v", err)
	}
	got, err := ring.Open(sealed, knownAnswerContext)
	if err != nil {
		t.Fatalf("this build does not open the value an earlier build sealed: %v", err)
	}
	if string(got) != knownAnswerPlaintext {
		t.Errorf("the value an earlier build sealed opens to %q, want %q", got, knownAnswerPlaintext)
	}
}

// VERIFICATIONS' row for a build that derives another public key, or another key identifier, from
// the same seed. An opener derives the public key from its seed at start and refuses a mounted
// public key matching none (ADR-0088), and finds a value's key by the identifier the value's header
// carries, so either change refuses every deployment's start or every stored value. The seed
// derives the public key an earlier build wrote beside it, and that key the identifier the earlier
// build's sealed value names.
func TestTheSeedDerivesThePublicKeyAnEarlierBuildWrote(t *testing.T) {
	publicPath, _ := knownAnswerFile(t, "credential.pub")
	seedPath, _ := knownAnswerFile(t, "credential.key")
	_, sealed := knownAnswerFile(t, "credential.sealed")
	// A keyring refuses a public key matching none of its seeds, so it loads only when this build
	// derives from the seed the public key an earlier build wrote beside it.
	ring, err := open.Load(publicPath, seedPath)
	if err != nil {
		t.Fatalf("the seed derives another public key than the one an earlier build wrote beside it: %v", err)
	}
	// A sealed value's identifier is the 16 bytes after its version byte (ADR-0088).
	id := ring.Current()
	if want := sealed[1:17]; !bytes.Equal(id[:], want) {
		t.Errorf("the key is named %x, and the value an earlier build sealed to it names %x", id[:], want)
	}
}
