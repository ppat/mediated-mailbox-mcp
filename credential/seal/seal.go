// Package seal is the sealing half of the credential library. It seals an account's provider
// credential or the OAuth client's secret to the current public key, and it defines the bytes a
// sealed value carries (ADR-0081, ADR-0088). It holds nothing that opens a value, so an import list
// can admit it without the opening half, and the UI links no code that opens a credential. The
// opening half imports this one, so the compiler refuses an import of it from here as a cycle.
//
// A sealed value is a version byte naming the suite, the 16-byte identifier of the key it was
// sealed to, the encapsulated key, and the ciphertext with its tag. The additional data binds that
// header and a Context naming the value's purpose and its row, so a value moved to another row or
// used as the other kind of secret fails to open.
package seal

import (
	"bytes"
	"crypto/hpke"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// Version is the version byte of a value sealed with HPKE's X-Wing suite, HKDF-SHA256 and
// ChaCha20-Poly1305 in base mode (ADR-0088). Another suite takes another version byte.
const Version byte = 1

// Sizes of the parts of a sealed value and of a public key file.
const (
	// KeyIDSize is the length of a key identifier.
	KeyIDSize = 16
	// HeaderSize is the length of the version byte and the key identifier.
	HeaderSize = 1 + KeyIDSize
	// EncapsulatedKeySize is the length of an X-Wing encapsulated key.
	EncapsulatedKeySize = 1120
	// PublicKeySize is the length of an X-Wing public key, which is what a public key file holds.
	PublicKeySize = 1216
	// tagSize is the length of ChaCha20-Poly1305's tag.
	tagSize = 16
	// MinSealedSize is the length of a value sealed from an empty plaintext.
	MinSealedSize = HeaderSize + EncapsulatedKeySize + tagSize
)

// keyIDDomain is the fixed string a key identifier's hash starts with, so the identifier is never
// the same hash of the public key another protocol computes.
const keyIDDomain = "mediated-mailbox credential key identifier\x00"

// info is HPKE's info parameter. It is a constant, since the context sits in the additional data.
const info = "mediated-mailbox credential"

// KEM returns the suite's key encapsulation mechanism, X-Wing. The opening half and the
// key-generation command take the suite from these three functions, so no two places can disagree
// on it.
func KEM() hpke.KEM { return hpke.MLKEM768X25519() }

// KDF returns the suite's key derivation function.
func KDF() hpke.KDF { return hpke.HKDFSHA256() }

// AEAD returns the suite's authenticated encryption.
func AEAD() hpke.AEAD { return hpke.ChaCha20Poly1305() }

// Info returns HPKE's info parameter.
func Info() []byte { return []byte(info) }

// KeyID names the public key a value was sealed to. It is derived from the key, so it cannot
// disagree with the key file.
type KeyID [KeyIDSize]byte

// String returns the identifier in hexadecimal, the form a log line carries.
func (id KeyID) String() string { return hex.EncodeToString(id[:]) }

// IDOf returns the identifier of a public key, the first 16 bytes of SHA-256 over a fixed domain
// string and the key.
func IDOf(publicKey []byte) KeyID {
	sum := sha256.Sum256(append([]byte(keyIDDomain), publicKey...))
	var id KeyID
	copy(id[:], sum[:KeyIDSize])
	return id
}

// PublicKey is the key every sealer seals to. Its zero value holds no key and seals nothing.
type PublicKey struct {
	key hpke.PublicKey
	id  KeyID
}

// ParsePublicKey reads a public key from the bytes of a public key file, the X-Wing public key
// itself.
func ParsePublicKey(b []byte) (PublicKey, error) {
	if len(b) != PublicKeySize {
		return PublicKey{}, fmt.Errorf("credential: a public key is %d bytes, and this one is %d", PublicKeySize, len(b))
	}
	key, err := KEM().NewPublicKey(b)
	if err != nil {
		return PublicKey{}, fmt.Errorf("credential: reading a public key: %w", err)
	}
	return PublicKey{key: key, id: IDOf(b)}, nil
}

// LoadPublicKey reads the public key from its mounted file (ADR-0079).
func LoadPublicKey(path string) (PublicKey, error) {
	b, err := os.ReadFile(path) //nolint:gosec // the path is the mounted key file the composition root names
	if err != nil {
		return PublicKey{}, fmt.Errorf("credential: reading the public key file: %w", err)
	}
	return ParsePublicKey(b)
}

// KeyID returns the identifier every value sealed to the key carries.
func (p PublicKey) KeyID() KeyID { return p.id }

// Bytes returns the key as its file holds it, or nil for the zero value.
func (p PublicKey) Bytes() []byte {
	if p.key == nil {
		return nil
	}
	return p.key.Bytes()
}

// Equal reports whether two public keys are the same key.
func (p PublicKey) Equal(other PublicKey) bool {
	return p.key != nil && other.key != nil && bytes.Equal(p.key.Bytes(), other.key.Bytes())
}

// Seal seals plaintext to the key, bound to c. Every call draws a fresh encapsulated key, so two
// seals of the same plaintext differ.
func (p PublicKey) Seal(plaintext []byte, c Context) ([]byte, error) {
	if p.key == nil {
		return nil, errors.New("credential: sealing with no public key")
	}
	if err := c.Valid(); err != nil {
		return nil, err
	}
	header := make([]byte, 0, HeaderSize)
	header = append(header, Version)
	header = append(header, p.id[:]...)
	enc, sender, err := hpke.NewSender(p.key, KDF(), AEAD(), Info())
	if err != nil {
		return nil, fmt.Errorf("credential: sealing: %w", err)
	}
	ciphertext, err := sender.Seal(AdditionalData(Version, p.id, c), plaintext)
	if err != nil {
		return nil, fmt.Errorf("credential: sealing: %w", err)
	}
	return bytes.Join([][]byte{header, enc, ciphertext}, nil), nil
}

// ErrMalformed is returned for a value too short to hold its header, its encapsulated key and its
// tag, or whose version byte names a suite this library does not seal with.
var ErrMalformed = errors.New("credential: the sealed value is truncated or of an unknown version")

// Parts is a sealed value split at its header.
type Parts struct {
	// KeyID names the key the value says it was sealed to.
	KeyID KeyID
	// EncapsulatedKey is the KEM's output.
	EncapsulatedKey []byte
	// Ciphertext is the AEAD's output, the tag included.
	Ciphertext []byte
}

// Split splits a sealed value into its parts. It refuses a value too short to be one and a version
// byte other than Version. Nothing it returns has been authenticated.
func Split(sealed []byte) (Parts, error) {
	if len(sealed) < MinSealedSize || sealed[0] != Version {
		return Parts{}, ErrMalformed
	}
	var p Parts
	copy(p.KeyID[:], sealed[1:HeaderSize])
	p.EncapsulatedKey = sealed[HeaderSize : HeaderSize+EncapsulatedKeySize]
	p.Ciphertext = sealed[HeaderSize+EncapsulatedKeySize:]
	return p, nil
}

// purpose is the kind of secret a sealed value holds. The zero value is none, and a Context holding
// it seals and opens nothing.
type purpose uint8

const (
	purposeNone purpose = iota
	// purposeAccountCredential is an account's provider credential, stored in its state row.
	purposeAccountCredential
	// purposeClientSecret is the installation's OAuth client's secret.
	purposeClientSecret
)

// String returns the purpose as the additional data spells it, empty for none.
func (p purpose) String() string {
	switch p {
	case purposeAccountCredential:
		return "account credential"
	case purposeClientSecret:
		return "oauth client secret"
	case purposeNone:
		return ""
	default:
		return ""
	}
}

// Context names a sealed value's purpose and the row it is stored in. The same context opens a
// value that sealed it, and no other does. Its zero value names nothing and seals nothing.
type Context struct {
	purpose purpose
	row     string
}

// AccountCredential is the context of the credential stored in the account's state row.
func AccountCredential(accountID string) Context {
	return Context{purpose: purposeAccountCredential, row: accountID}
}

// ClientSecret is the context of the OAuth client's secret, stored in the row its argument names.
func ClientSecret(row string) Context {
	return Context{purpose: purposeClientSecret, row: row}
}

// Valid refuses a context naming no purpose or no row, since it would bind the value to nothing.
func (c Context) Valid() error {
	if c.purpose.String() == "" {
		return errors.New("credential: the context names no purpose")
	}
	if c.row == "" {
		return errors.New("credential: the context names no row")
	}
	return nil
}

// AdditionalData is the AEAD's additional data for a value with this version byte and key
// identifier, bound to c. It is the version byte and the key identifier, a zero byte, then the
// purpose and the row, each prefixed with its length as four big-endian bytes, so no two contexts
// encode alike.
func AdditionalData(version byte, id KeyID, c Context) []byte {
	name := c.purpose.String()
	out := make([]byte, 0, HeaderSize+1+4+len(name)+4+len(c.row))
	out = append(out, version)
	out = append(out, id[:]...)
	out = append(out, 0)
	out = binary.BigEndian.AppendUint32(out, uint32(len(name))) //nolint:gosec // a purpose is one of two short constants
	out = append(out, name...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(c.row))) //nolint:gosec // a row is an identifier far shorter than 4 GiB
	out = append(out, c.row...)
	return out
}
