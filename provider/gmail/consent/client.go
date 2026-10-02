package consent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// checkCode is the authorization code a client check sends, which names no consent.
const checkCode = "mediated-mailbox-client-check"

// CheckClient asks Google's token endpoint whether it recognises a client's identifier and secret,
// before the UI stores them (ADR-0107). It exchanges a code no consent issued. Google checks the
// client before the code, so an answer that refuses the code, invalid_grant, means the client was
// accepted, and one that refuses the client means it was not (RFC 6749 section 5.2). It returns nil
// for an accepted client, mail.ErrClientRefused for a refused one, and any other error for an answer
// it cannot read as either. Whether Google answers a Web application client the same way is one of
// the behaviours the live check of the client setup settles (ROADMAP.md, M7).
func CheckClient(ctx context.Context, client *http.Client, clientID, clientSecret, redirectURI string) error {
	_, err := PostForm(ctx, client, checkForm(clientID, clientSecret, redirectURI))
	return checkAnswer(err)
}

// checkForm is the client check's request, a code exchange for checkCode with a verifier of the
// length PKCE requires.
func checkForm(clientID, clientSecret, redirectURI string) url.Values {
	return exchangeForm(clientID, clientSecret, checkCode, "mediated-mailbox-client-check-verifier-0000000", redirectURI)
}

// checkAnswer reads the token endpoint's answer to a client check.
func checkAnswer(err error) error {
	var answered *answerError
	switch {
	case err == nil:
		return errors.New("gmail: the token endpoint exchanged a code no consent issued")
	case !errors.As(err, &answered):
		return err
	case answered.reason == "invalid_grant":
		return nil
	case answered.code == http.StatusBadRequest || answered.code == http.StatusUnauthorized:
		return fmt.Errorf("%w: %w", mail.ErrClientRefused, err)
	default:
		return err
	}
}
