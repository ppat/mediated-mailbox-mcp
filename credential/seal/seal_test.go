package seal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

func public(t *testing.T) []byte {
	t.Helper()
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key.PublicKey().Bytes()
}

// A key identifier is the first 16 bytes of SHA-256 over the fixed domain string and the public key,
// computed here from ADR-0088's rule rather than read from the code.
func TestAKeyIdentifierIsDerivedFromTheKey(t *testing.T) {
	b := public(t)
	sum := sha256.Sum256(append([]byte("mediated-mailbox credential key identifier\x00"), b...))
	got := seal.IDOf(b)
	if !bytes.Equal(got[:], sum[:16]) {
		t.Errorf("IDOf = %x, want %x", got[:], sum[:16])
	}
	key, err := seal.ParsePublicKey(b)
	if err != nil {
		t.Fatal(err)
	}
	if key.KeyID() != got {
		t.Errorf("the parsed key names %s, want %s", key.KeyID(), got)
	}
	if got.String() != hex.EncodeToString(sum[:16]) {
		t.Errorf("the identifier prints as %s", got)
	}
}

// The additional data is the version byte and the key identifier, a zero byte, then the purpose and
// the row, each prefixed with its length as four big-endian bytes (ADR-0088). The lengths keep a
// purpose and a row from running into each other, so no two contexts encode alike.
func TestTheAdditionalDataBindsTheHeaderAndTheContext(t *testing.T) {
	var id seal.KeyID
	for i := range id {
		id[i] = byte(i + 1)
	}
	header := "\x01\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x00"
	for name, c := range map[string]struct {
		context seal.Context
		want    string
	}{
		"an account's credential": {seal.AccountCredential("personal"), header + "\x00\x00\x00\x12account credential\x00\x00\x00\x08personal"},
		"the client's secret":     {seal.ClientSecret("gmail"), header + "\x00\x00\x00\x13oauth client secret\x00\x00\x00\x05gmail"},
	} {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff([]byte(c.want), seal.AdditionalData(1, id, c.context), compare.Options); diff != "" {
				t.Errorf("additional data (-want +got):\n%s", diff)
			}
		})
	}
}

// A value is sealed only to a key and only under a context naming its purpose and its row, since a
// context naming nothing would bind the value to nothing.
func TestSealingRefusesAMissingKeyOrContext(t *testing.T) {
	key, err := seal.ParsePublicKey(public(t))
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		key     seal.PublicKey
		context seal.Context
	}{
		"no key":                        {seal.PublicKey{}, seal.AccountCredential("personal")},
		"the zero context":              {key, seal.Context{}},
		"an account with no identifier": {key, seal.AccountCredential("")},
		"a client with no row":          {key, seal.ClientSecret("")},
	} {
		t.Run(name, func(t *testing.T) {
			if sealed, err := c.key.Seal([]byte("token"), c.context); err == nil {
				t.Errorf("sealed %d bytes", len(sealed))
			}
		})
	}
}

// A public key file holds the 1216-byte X-Wing public key and nothing else.
func TestAPublicKeyIsItsExactBytes(t *testing.T) {
	b := public(t)
	for name, file := range map[string][]byte{
		"a trailing newline": append(bytes.Clone(b), '\n'),
		"one byte short":     b[:len(b)-1],
		"empty":              nil,
	} {
		if _, err := seal.ParsePublicKey(file); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

// A value too short to hold a header, an encapsulated key and a tag, or of another version, is not
// split.
func TestSplitRefusesATruncatedOrReversionedValue(t *testing.T) {
	key, err := seal.ParsePublicKey(public(t))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := key.Seal(nil, seal.AccountCredential("personal"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed) != 1+16+1120+16 {
		t.Fatalf("an empty plaintext sealed to %d bytes", len(sealed))
	}
	if _, err := seal.Split(sealed); err != nil {
		t.Errorf("Split: %v", err)
	}
	reversioned := bytes.Clone(sealed)
	reversioned[0] = 2
	for name, v := range map[string][]byte{"one byte short": sealed[:len(sealed)-1], "version 2": reversioned} {
		if _, err := seal.Split(v); !errors.Is(err, seal.ErrMalformed) {
			t.Errorf("%s: %v, want ErrMalformed", name, err)
		}
	}
}
