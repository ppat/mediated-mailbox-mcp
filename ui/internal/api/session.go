package api

import (
	"context"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
)

// sessionCookie is the anonymous session the request token is bound to (ADR-0061).
const sessionCookie = "ui_session"

// tokenHeader carries the request token on every state-changing request (docs/UI.md section 15).
const tokenHeader = "X-Request-Token"

// MinTokenKey is the fewest bytes the key behind the request token may hold.
const MinTokenKey = 32

// The labels the two keys are derived under, so the request token's key and the consent attempt's
// sealing key never coincide (ADR-0111).
const (
	tokenLabel   = "mediated-mailbox ui request token"
	attemptLabel = "mediated-mailbox ui consent attempt"
)

// keys are the request token's key and the consent attempt's cipher, both derived from the one key
// the composition root hands the server, generated at its start or read from token_key_file.
type keys struct {
	token   []byte
	attempt cipher.AEAD
}

func newKeys(material []byte) (keys, error) {
	if len(material) < MinTokenKey {
		return keys{}, errors.New("the request token's key holds fewer than 32 bytes")
	}
	token, err := hkdf.Key(sha256.New, material, nil, tokenLabel, 32)
	if err != nil {
		return keys{}, err
	}
	sealing, err := hkdf.Key(sha256.New, material, nil, attemptLabel, 32)
	if err != nil {
		return keys{}, err
	}
	aead, err := newAEAD(sealing)
	if err != nil {
		return keys{}, err
	}
	return keys{token: token, attempt: aead}, nil
}

// requestToken is the token for a session, an HMAC of its identifier under the token's key.
func (k keys) requestToken(session string) string {
	mac := hmac.New(sha256.New, k.token)
	mac.Write([]byte(session))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

type sessionKey struct{}

// session is the request's session, the identifier its cookie carried, or the one issued with its
// response when it carried none.
type session struct {
	id string
	// carried reports whether the request's cookie named it, so a request whose session was only
	// issued now carries no token for it.
	carried bool
}

// sessionOf returns the request's session. Every request passes through withSession first.
func sessionOf(ctx context.Context) session {
	if s, ok := ctx.Value(sessionKey{}).(session); ok {
		return s
	}
	return session{}
}

// withSession gives every request a session. A request without the cookie, or with one that is not a
// session identifier this server could have issued, is issued a new one with its response, HttpOnly,
// Secure and SameSite=Strict with browser-session lifetime (ADR-0061).
func withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := session{}
		if c, err := r.Cookie(sessionCookie); err == nil && validSession(c.Value) {
			s = session{id: c.Value, carried: true}
		} else {
			s = session{id: rand.Text() + rand.Text()}
			http.SetCookie(w, &http.Cookie{
				Name: sessionCookie, Value: s.id, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
			})
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, s)))
	})
}

// validSession reports whether v has the shape of an identifier withSession issues, 52 characters of
// rand.Text's alphabet.
func validSession(v string) bool {
	if len(v) != 52 {
		return false
	}
	for _, c := range v {
		if (c < 'A' || c > 'Z') && (c < '2' || c > '7') {
			return false
		}
	}
	return true
}

// withToken refuses every request whose method is not GET or HEAD before it is routed, unless it
// carries the request token of the session its cookie names (ADR-0061). The check sits in front of
// the whole mux, so a state-changing route added later cannot be mounted without it. A request with
// no token, a token of another session, or no session cookie is refused with stale_page, the
// answer a page older than the server's last restart gets (docs/UI.md sections 15 and 17.3).
func withToken(k keys, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			s := sessionOf(r.Context())
			got := r.Header.Get(tokenHeader)
			if !s.carried || got == "" || !hmac.Equal([]byte(got), []byte(k.requestToken(s.id))) {
				requestInfo(r.Context()).route = "token"
				writeFailure(w, r, clientFault(http.StatusForbidden, "stale_page",
					"the request token does not match this session, as on a page older than the UI server's last restart"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
