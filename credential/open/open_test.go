package open_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// A refresh token of the length Google issues, which seals to 1180 bytes (ADR-0088).
var token = []byte("1//0g-refresh-token-27bytes")

// pair generates a key pair the way the key-generation command does and returns the private key's
// seed with the public key's bytes.
func pair(t *testing.T) (seed, public []byte) {
	t.Helper()
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err = key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return seed, key.PublicKey().Bytes()
}

func privateKey(t *testing.T, seed []byte) open.PrivateKey {
	t.Helper()
	k, err := open.ParsePrivateKey(seed)
	if err != nil {
		t.Fatalf("ParsePrivateKey: %v", err)
	}
	return k
}

func publicKey(t *testing.T, public []byte) seal.PublicKey {
	t.Helper()
	p, err := seal.ParsePublicKey(public)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}
	return p
}

// keyring returns a keyring holding the seeds and sealing to the public key.
func keyring(t *testing.T, public []byte, seeds ...[]byte) *open.Keyring {
	t.Helper()
	keys := make([]open.PrivateKey, 0, len(seeds))
	for _, s := range seeds {
		keys = append(keys, privateKey(t, s))
	}
	r, err := open.NewKeyring(publicKey(t, public), keys...)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	return r
}

func sealTo(t *testing.T, public []byte, plaintext []byte, c seal.Context) []byte {
	t.Helper()
	sealed, err := publicKey(t, public).Seal(plaintext, c)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	return sealed
}

// The part of VERIFICATIONS' row for opening a sealed credential that code can reach. The public key
// in the opening subsection's place is the must-not-compile case below. A value opens with the key it
// was sealed to, and a keyring holding only another pair's key, or a value altered by one byte, is
// refused (ADR-0081).
func TestOnlyThePrivateKeyItWasSealedToOpensAValue(t *testing.T) {
	seed, public := pair(t)
	otherSeed, otherPublic := pair(t)
	c := seal.AccountCredential("personal")
	sealed := sealTo(t, public, token, c)

	got, err := keyring(t, public, seed).Open(sealed, c)
	if err != nil || !bytes.Equal(got, token) {
		t.Fatalf("opening with the key it was sealed to: %q, %v, want the token", got, err)
	}
	if got, err := keyring(t, otherPublic, otherSeed).Open(sealed, c); err == nil {
		t.Errorf("another key pair's private key opened the value as %q", got)
	}
	altered := bytes.Clone(sealed)
	altered[len(altered)-1] ^= 0x01
	if got, err := keyring(t, public, seed).Open(altered, c); !errors.Is(err, open.ErrRefused) {
		t.Errorf("a value altered by one byte: %q, %v, want ErrRefused", got, err)
	}
}

// A public key passed where the opening subsection takes a private key does not compile (ADR-0088).
func TestAPublicKeyIsNotAPrivateKey(t *testing.T) {
	mustnotcompile.Require(t, "./testdata/mustnotcompile/publicasprivate",
		"cannot use public (variable of struct type seal.PublicKey) as open.PrivateKey value in argument to open.NewKeyring")
}

// A 27-byte token seals to 1180 bytes, a version byte, the key identifier, the encapsulated key and
// the ciphertext with its tag, and the header names the key sealed to (ADR-0088).
func TestASealedValuesBytes(t *testing.T) {
	_, public := pair(t)
	sealed := sealTo(t, public, token, seal.AccountCredential("personal"))
	if len(sealed) != 1180 {
		t.Errorf("a 27-byte value sealed to %d bytes, want 1180", len(sealed))
	}
	if sealed[0] != 1 {
		t.Errorf("version byte %d, want 1", sealed[0])
	}
	if id := seal.IDOf(public); !bytes.Equal(sealed[1:17], id[:]) {
		t.Errorf("the header names key %x, want %x", sealed[1:17], id[:])
	}
}

// VERIFICATIONS' row for altering a sealed value. Every single-bit flip, every truncation and every
// other version byte is refused, and two seals of the same plaintext differ (ADR-0088).
func TestEveryAlterationIsRefused(t *testing.T) {
	seed, public := pair(t)
	ring := keyring(t, public, seed)
	c := seal.AccountCredential("personal")
	sealed := sealTo(t, public, token, c)

	t.Run("every single-bit flip", func(t *testing.T) {
		// A flip in the version byte or past the identifier is refused as altered. A flip in the
		// identifier names a key the keyring does not hold, since the identifier is authenticated
		// only once its key is found.
		for i := range len(sealed) * 8 {
			flipped := bytes.Clone(sealed)
			flipped[i/8] ^= 1 << (i % 8)
			want := open.ErrRefused
			if byteAt := i / 8; byteAt >= 1 && byteAt <= 16 {
				want = open.ErrUnknownKey
			}
			if got, err := ring.Open(flipped, c); !errors.Is(err, want) {
				t.Fatalf("flipping bit %d of byte %d: %q, %v, want %v", i%8, i/8, got, err, want)
			}
		}
	})
	t.Run("every truncation", func(t *testing.T) {
		for n := range len(sealed) {
			if got, err := ring.Open(sealed[:n], c); !errors.Is(err, open.ErrRefused) {
				t.Fatalf("the value cut to %d bytes: %q, %v, want ErrRefused", n, got, err)
			}
		}
	})
	t.Run("every other version byte", func(t *testing.T) {
		for v := range 256 {
			if v == 1 {
				continue
			}
			versioned := bytes.Clone(sealed)
			versioned[0] = byte(v)
			if got, err := ring.Open(versioned, c); !errors.Is(err, open.ErrRefused) {
				t.Fatalf("version byte %d: %q, %v, want ErrRefused", v, got, err)
			}
		}
	})
	t.Run("two seals of the same plaintext", func(t *testing.T) {
		first, err := seal.Split(sealed)
		if err != nil {
			t.Fatal(err)
		}
		second, err := seal.Split(sealTo(t, public, token, c))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(first.EncapsulatedKey, second.EncapsulatedKey) || bytes.Equal(first.Ciphertext, second.Ciphertext) {
			t.Error("two seals of the same plaintext share an encapsulated key or a ciphertext")
		}
	})
}

// VERIFICATIONS' row for copying a sealed value. A value opens only under the context it was sealed
// with, so a value copied to another account's row or used as the OAuth client's secret is refused
// (ADR-0088).
func TestAValueOpensOnlyInItsOwnRowAndPurpose(t *testing.T) {
	seed, public := pair(t)
	ring := keyring(t, public, seed)
	sealed := sealTo(t, public, token, seal.AccountCredential("personal"))
	for name, c := range map[string]seal.Context{
		"another account's credential":            seal.AccountCredential("work"),
		"the client's secret under the same name": seal.ClientSecret("personal"),
		"a row whose name extends the account":    seal.AccountCredential("personal2"),
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := ring.Open(sealed, c); !errors.Is(err, open.ErrRefused) {
				t.Errorf("opened as %q, %v, want ErrRefused", got, err)
			}
		})
	}
	t.Run("a context naming nothing", func(t *testing.T) {
		if got, err := ring.Open(sealed, seal.Context{}); err == nil {
			t.Errorf("opened as %q under the zero context", got)
		}
	})
}

// VERIFICATIONS' row for a missing key and tampering. A value sealed to a key the keyring does not
// hold is refused as naming an unknown key and never as altered, and a value naming a held key and
// altered by one byte is refused as altered. A value whose identifier bytes were altered names no
// held key, so it too reads as an unknown key (ADR-0088).
func TestAMissingKeyIsToldApartFromTampering(t *testing.T) {
	seed, public := pair(t)
	_, otherPublic := pair(t)
	ring := keyring(t, public, seed)
	c := seal.AccountCredential("personal")

	unknown := sealTo(t, otherPublic, token, c)
	_, err := ring.Open(unknown, c)
	if !errors.Is(err, open.ErrUnknownKey) || errors.Is(err, open.ErrRefused) {
		t.Errorf("a value sealed to a key the keyring does not hold: %v, want ErrUnknownKey alone", err)
	}

	altered := sealTo(t, public, token, c)
	altered[len(altered)/2] ^= 0x80
	_, err = ring.Open(altered, c)
	if !errors.Is(err, open.ErrRefused) || errors.Is(err, open.ErrUnknownKey) {
		t.Errorf("a value altered by one byte: %v, want ErrRefused alone", err)
	}

	reidentified := sealTo(t, public, token, c)
	reidentified[5] ^= 0x01
	_, err = ring.Open(reidentified, c)
	if !errors.Is(err, open.ErrUnknownKey) || errors.Is(err, open.ErrRefused) {
		t.Errorf("a value whose identifier bytes were altered: %v, want ErrUnknownKey alone", err)
	}
}

// F6's part of VERIFICATIONS' row for a public key matching none of the private keys. The keyring an
// opener starts with refuses such a public key, because it would seal rotated credentials no opener
// can read (ADR-0088).
func TestAKeyringRefusesAPublicKeyMatchingNoPrivateKey(t *testing.T) {
	seed, public := pair(t)
	oldSeed, oldPublic := pair(t)
	_, strayPublic := pair(t)

	if _, err := open.NewKeyring(publicKey(t, strayPublic), privateKey(t, seed), privateKey(t, oldSeed)); err == nil {
		t.Error("a keyring started with a public key matching none of its private keys")
	}
	if _, err := open.NewKeyring(publicKey(t, public)); err == nil {
		t.Error("a keyring started with no private key")
	}
	var empty open.PrivateKey
	if _, err := open.NewKeyring(publicKey(t, public), empty); err == nil {
		t.Error("a keyring started with an empty private key")
	}
	for name, current := range map[string][]byte{"the first": public, "the second": oldPublic} {
		if _, err := open.NewKeyring(publicKey(t, current), privateKey(t, seed), privateKey(t, oldSeed)); err != nil {
			t.Errorf("a public key matching %s private key: %v", name, err)
		}
	}
}

// F6's part of VERIFICATIONS' row for re-sealing to the current key. A value sealed to an old key the
// keyring holds is sealed again to the current key and still opens, and a value already sealed to the
// current key is left as it is (ADR-0092).
func TestReSealingMovesAValueToTheCurrentKey(t *testing.T) {
	seed, public := pair(t)
	oldSeed, oldPublic := pair(t)
	ring := keyring(t, public, oldSeed, seed)
	c := seal.AccountCredential("personal")

	old := sealTo(t, oldPublic, token, c)
	plaintext, stored, resealed, err := ring.Reseal(old, c)
	if err != nil {
		t.Fatalf("Reseal: %v", err)
	}
	if !resealed || !bytes.Equal(plaintext, token) {
		t.Errorf("re-sealing a value on the old key: resealed %v, plaintext %q, want true and the token", resealed, plaintext)
	}
	if id := seal.IDOf(public); !bytes.Equal(stored[1:17], id[:]) {
		t.Errorf("the re-sealed value names key %x, want the current key %x", stored[1:17], id[:])
	}
	if got, err := keyring(t, public, seed).Open(stored, c); err != nil || !bytes.Equal(got, token) {
		t.Errorf("the re-sealed value opened with the current key alone: %q, %v, want the token", got, err)
	}

	current := sealTo(t, public, token, c)
	_, stored, resealed, err = ring.Reseal(current, c)
	if err != nil || resealed || !bytes.Equal(stored, current) {
		t.Errorf("re-sealing a value on the current key: resealed %v, unchanged %v, %v, want it left as it is", resealed, bytes.Equal(stored, current), err)
	}

	if _, _, _, err := ring.Reseal(old, seal.AccountCredential("work")); !errors.Is(err, open.ErrRefused) {
		t.Errorf("re-sealing under another account's context: %v, want ErrRefused", err)
	}
}

// F6's part of VERIFICATIONS' row for every sealer sealing to one current key. The public key a sealer
// holds, a keyring's seal of a rotated credential and a keyring's re-seal all name the current key,
// and a keyring reports that key (ADR-0092).
func TestEverySealerNamesTheCurrentKey(t *testing.T) {
	seed, public := pair(t)
	oldSeed, oldPublic := pair(t)
	ring := keyring(t, public, oldSeed, seed)
	c := seal.AccountCredential("personal")
	want := seal.IDOf(public)

	if got := ring.Current(); got != want {
		t.Errorf("the keyring reports key %s, want %s", got, want)
	}
	fromRing, err := ring.Seal(token, c)
	if err != nil {
		t.Fatal(err)
	}
	_, resealed, _, err := ring.Reseal(sealTo(t, oldPublic, token, c), c)
	if err != nil {
		t.Fatal(err)
	}
	for name, sealed := range map[string][]byte{
		"the public key":       sealTo(t, public, token, c),
		"the keyring's seal":   fromRing,
		"the keyring's reseal": resealed,
	} {
		parts, err := seal.Split(sealed)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if parts.KeyID != want {
			t.Errorf("%s sealed to key %s, want %s", name, parts.KeyID, want)
		}
	}
}

// A keyring loads its keys from the mounted files, and refuses a file of the wrong length.
func TestAKeyringLoadsItsKeysFromFiles(t *testing.T) {
	seed, public := pair(t)
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	privatePath, publicPath := write("private", seed), write("public", public)
	ring, err := open.Load(publicPath, privatePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := seal.AccountCredential("personal")
	if got, err := ring.Open(sealTo(t, public, token, c), c); err != nil || !bytes.Equal(got, token) {
		t.Errorf("a loaded keyring opened %q, %v, want the token", got, err)
	}
	for name, paths := range map[string][2]string{
		"a seed with a trailing newline":       {publicPath, write("newline", append(bytes.Clone(seed), '\n'))},
		"a public key with a trailing byte":    {write("long", append(bytes.Clone(public), 0)), privatePath},
		"a missing private key file":           {publicPath, filepath.Join(dir, "absent")},
		"the public key where the seed should": {publicPath, publicPath},
	} {
		if _, err := open.Load(paths[0], paths[1]); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}
