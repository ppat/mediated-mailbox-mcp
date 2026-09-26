// Package open is the opening half of the credential library. It holds the keyring of private
// keys a deployable that calls a provider is given, opens a sealed value with the key its header
// names, and seals to the current public key, so it can write back a rotated credential and re-seal
// a value opened with a key that is not the current one (ADR-0081, ADR-0082, ADR-0088, ADR-0092).
//
// It refuses to build a keyring whose public key matches none of its private keys, because such a
// deployable would seal rotated credentials no opener can read. A value naming a key the keyring
// holds that fails authentication is refused as altered, and a value naming no key it holds is
// refused as naming an unknown key, so a missing key file is never reported as tampering. The key
// identifier is authenticated only once its key is found, so a value whose identifier bytes were
// altered also reads as naming an unknown key.
package open

import (
	"crypto/hpke"
	"errors"
	"fmt"
	"os"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
)

// SeedSize is the length of an X-Wing seed, which is what a private key file holds.
const SeedSize = 32

// ErrUnknownKey is returned for a value whose header names a key the keyring does not hold. The
// value may be intact and the key file missing, or its identifier bytes may have been altered.
var ErrUnknownKey = errors.New("credential: the sealed value names a key this keyring does not hold")

// ErrRefused is returned for a value that is truncated or of an unknown version, and for a value
// naming a key the keyring holds that fails authentication, because it was altered or sealed under
// another context. Nothing of it is returned.
var ErrRefused = errors.New("credential: the sealed value was refused, as altered, truncated, of an unknown version or bound to another row or purpose")

// PrivateKey is one key of a keyring. Its zero value holds no key, and a keyring refuses it.
type PrivateKey struct {
	key    hpke.PrivateKey
	public seal.PublicKey
}

// ParsePrivateKey reads a private key from the bytes of a private key file, the X-Wing seed.
func ParsePrivateKey(b []byte) (PrivateKey, error) {
	if len(b) != SeedSize {
		return PrivateKey{}, fmt.Errorf("credential: a private key is a %d-byte seed, and this one is %d bytes", SeedSize, len(b))
	}
	key, err := seal.KEM().NewPrivateKey(b)
	if err != nil {
		return PrivateKey{}, fmt.Errorf("credential: reading a private key: %w", err)
	}
	public, err := seal.ParsePublicKey(key.PublicKey().Bytes())
	if err != nil {
		return PrivateKey{}, err
	}
	return PrivateKey{key: key, public: public}, nil
}

// LoadPrivateKey reads a private key from its mounted file (ADR-0079).
func LoadPrivateKey(path string) (PrivateKey, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the path is the mounted key file the composition root names
	if err != nil {
		return PrivateKey{}, fmt.Errorf("credential: reading a private key file: %w", err)
	}
	return ParsePrivateKey(b)
}

// PublicKey returns the public key the private key derives.
func (k PrivateKey) PublicKey() seal.PublicKey { return k.public }

// Keyring is every private key a deployable holds and the one public key it seals to. It is
// immutable, so one keyring serves every goroutine.
type Keyring struct {
	keys    map[seal.KeyID]hpke.PrivateKey
	current seal.PublicKey
}

// NewKeyring returns the keyring holding keys and sealing to current. It refuses no keys, a zero
// key, and a current public key that none of the keys derives.
func NewKeyring(current seal.PublicKey, keys ...PrivateKey) (*Keyring, error) {
	if len(keys) == 0 {
		return nil, errors.New("credential: a keyring holds no private key")
	}
	ring := &Keyring{keys: make(map[seal.KeyID]hpke.PrivateKey, len(keys)), current: current}
	matched := false
	for _, k := range keys {
		if k.key == nil {
			return nil, errors.New("credential: a keyring was given an empty private key")
		}
		ring.keys[k.public.KeyID()] = k.key
		matched = matched || k.public.Equal(current)
	}
	if !matched {
		return nil, fmt.Errorf("credential: the public key %s matches none of the private keys, so values sealed to it could not be opened", current.KeyID())
	}
	return ring, nil
}

// Load reads the public key and every private key from their mounted files and returns the
// keyring they make, refusing one whose public key matches none of the private keys.
func Load(publicKeyPath string, privateKeyPaths ...string) (*Keyring, error) {
	current, err := seal.LoadPublicKey(publicKeyPath)
	if err != nil {
		return nil, err
	}
	keys := make([]PrivateKey, 0, len(privateKeyPaths))
	for _, path := range privateKeyPaths {
		k, err := LoadPrivateKey(path)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return NewKeyring(current, keys...)
}

// Current returns the identifier of the key the keyring seals to.
func (r *Keyring) Current() seal.KeyID { return r.current.KeyID() }

// Open opens a sealed value bound to c with the key its header names. It returns ErrUnknownKey
// when the keyring holds no such key and ErrRefused for any other failure.
func (r *Keyring) Open(sealed []byte, c seal.Context) ([]byte, error) {
	plaintext, _, err := r.open(sealed, c)
	return plaintext, err
}

// open opens a sealed value and also returns the key it names.
func (r *Keyring) open(sealed []byte, c seal.Context) ([]byte, seal.KeyID, error) {
	if err := c.Valid(); err != nil {
		return nil, seal.KeyID{}, err
	}
	parts, err := seal.Split(sealed)
	if err != nil {
		return nil, seal.KeyID{}, ErrRefused
	}
	key, ok := r.keys[parts.KeyID]
	if !ok {
		return nil, parts.KeyID, fmt.Errorf("%w: %s", ErrUnknownKey, parts.KeyID)
	}
	recipient, err := hpke.NewRecipient(parts.EncapsulatedKey, key, seal.KDF(), seal.AEAD(), seal.Info())
	if err != nil {
		return nil, parts.KeyID, ErrRefused
	}
	plaintext, err := recipient.Open(seal.AdditionalData(seal.Version, parts.KeyID, c), parts.Ciphertext)
	if err != nil {
		return nil, parts.KeyID, ErrRefused
	}
	return plaintext, parts.KeyID, nil
}

// Seal seals plaintext to the current public key, bound to c, as a rotated credential is sealed
// before it is written back.
func (r *Keyring) Seal(plaintext []byte, c seal.Context) ([]byte, error) {
	return r.current.Seal(plaintext, c)
}

// Reseal opens a sealed value bound to c and, when it names a key other than the current one,
// seals its plaintext again to the current key. It returns the plaintext, the value to store and
// whether that value is new. A value already sealed to the current key comes back as it was.
func (r *Keyring) Reseal(sealed []byte, c seal.Context) (plaintext, stored []byte, resealed bool, err error) {
	plaintext, id, err := r.open(sealed, c)
	if err != nil {
		return nil, nil, false, err
	}
	if id == r.current.KeyID() {
		return plaintext, sealed, false, nil
	}
	stored, err = r.current.Seal(plaintext, c)
	if err != nil {
		return nil, nil, false, err
	}
	return plaintext, stored, true, nil
}
