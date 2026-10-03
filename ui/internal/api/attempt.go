package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// attemptCookie carries the session's one consent attempt, sealed by the server (ADR-0111).
const attemptCookie = "ui_consent"

// attemptLifetime is how long an attempt lasts, the design's starting value (docs/UI.md section 8.12).
const attemptLifetime = 15 * time.Minute

// What an attempt does.
const (
	attemptConnect     = "connect"
	attemptReauthorize = "reauthorize"
	attemptMove        = "move"
)

// attempt is one consent attempt as its cookie carries it (ADR-0111). ClientID is the identifier the
// client held when the attempt started, so a client replaced since is told apart at the finish.
type attempt struct {
	Kind     string `json:"kind"`
	State    string `json:"state"`
	Verifier string `json:"verifier"`
	Account  string `json:"account"`
	Provider string `json:"provider"`
	Mailbox  string `json:"mailbox"`
	// Remembered is whether the account remembered Mailbox when the attempt started. Otherwise a finish
	// stores and answers the mailbox the provider confirmed, in the provider's spelling (ADR-0080).
	Remembered    bool     `json:"remembered"`
	Client        string   `json:"client"`
	ClientID      string   `json:"client_id"`
	LoweredTarget *float64 `json:"lowered_target"`
	// Expires is the instant the attempt expires, in milliseconds since the epoch.
	Expires int64 `json:"expires"`
	// Consent is the consent page's address, which a reload of the page shows again.
	Consent string `json:"consent"`
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealAttempt seals a to the session, its identifier the additional data, so it opens in no other
// session (ADR-0111).
func (k keys) sealAttempt(a attempt, sessionID string) (string, error) {
	plain, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, k.attempt.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(k.attempt.Seal(nonce, nonce, plain, []byte(sessionID))), nil
}

// errNoAttempt is a session holding no attempt this server can open.
var errNoAttempt = errors.New("the session holds no consent attempt")

// openAttempt opens the session's attempt from its cookie. A cookie that is absent, altered, sealed
// for another session or under another key is no attempt.
func (k keys) openAttempt(r *http.Request) (attempt, error) {
	c, err := r.Cookie(attemptCookie)
	if err != nil {
		return attempt{}, errNoAttempt
	}
	sealed, err := base64.RawURLEncoding.DecodeString(c.Value)
	n := k.attempt.NonceSize()
	if err != nil || len(sealed) < n {
		return attempt{}, errNoAttempt
	}
	plain, err := k.attempt.Open(nil, sealed[:n], sealed[n:], []byte(sessionOf(r.Context()).id))
	if err != nil {
		return attempt{}, errNoAttempt
	}
	var a attempt
	if err := json.Unmarshal(plain, &a); err != nil {
		return attempt{}, errNoAttempt
	}
	return a, nil
}

// keepAttempt sets the session's attempt, replacing any it held, so only the newest can finish.
func keepAttempt(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name: attemptCookie, Value: value, Path: "/api/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
}

// endAttempt clears the session's attempt once a finish has stored what it was for.
func endAttempt(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: attemptCookie, Value: "", Path: "/api/", MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
}
