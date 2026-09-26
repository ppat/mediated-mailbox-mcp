package gmail

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Consent runs the one-time interactive consent for one account and returns its grant. A developer
// runs it outside every deployable for the contract suite's test account, since a deployable's
// accounts are connected through the UI (ADR-0080, ADR-0083). It
// prints the consent page's address to prompt, receives Google's redirect on a loopback address,
// and exchanges the code for the grant. A login hint, when given, is the address of the account the
// grant is meant for.
func Consent(ctx context.Context, client *http.Client, clientID, clientSecret, loginHint string, prompt io.Writer) (Grant, error) {
	state, verifier := rand.Text(), rand.Text()+rand.Text()
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return Grant{}, fmt.Errorf("gmail: listening for the consent redirect: %w", err)
	}
	redirectURI := "http://" + ln.Addr().String() + "/"

	results := make(chan consentResult, 1)
	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           consentHandler(state, results),
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	var code string
	if _, err = fmt.Fprintf(prompt, "Open this address and grant access:\n%s\n", AuthorizationURL(clientID, redirectURI, state, verifier, loginHint)); err == nil {
		select {
		case r := <-results:
			code, err = r.code, r.err
		case err = <-served:
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := errors.Join(err, srv.Shutdown(shutdown)); err != nil {
		return Grant{}, err
	}
	return ExchangeCode(ctx, client, clientID, clientSecret, code, verifier, redirectURI)
}

// consentHandler receives Google's redirect after the consent. Only a redirect carrying state
// counts, so a request forged by another page is refused. The first one that counts is sent on
// results, as the authorization code or as the reason there is none, and later ones are answered
// and dropped.
func consentHandler(state string, results chan<- consentResult) http.Handler {
	decide := func(r consentResult) {
		select {
		case results <- r:
		default:
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case q.Get("state") != state:
			http.Error(w, "state mismatch", http.StatusBadRequest)
		case q.Get("error") != "":
			http.Error(w, "consent refused", http.StatusBadRequest)
			decide(consentResult{err: errors.New("gmail: the consent was refused")})
		case q.Get("code") == "":
			http.Error(w, "no authorization code", http.StatusBadRequest)
			decide(consentResult{err: errors.New("gmail: the consent redirect carried no authorization code")})
		default:
			if _, err := io.WriteString(w, "Consent received. This window can be closed.\n"); err != nil {
				decide(consentResult{err: fmt.Errorf("gmail: answering the consent redirect: %w", err)})
				return
			}
			decide(consentResult{code: q.Get("code")})
		}
	})
}

// consentResult is what the consent redirect delivered, the authorization code or the reason
// there is none.
type consentResult struct {
	code string
	err  error
}

// Address returns the address of the account an access token belongs to, read from the account's
// profile. It is how the operator, and the contract suite's run against Gmail, confirm which account
// a grant is for.
func Address(ctx context.Context, client *http.Client, accessToken string) (string, error) {
	r := profileRequest()
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gmail: reading the profile: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxResponse))
	if err := errors.Join(readErr, res.Body.Close()); err != nil {
		return "", fmt.Errorf("gmail: reading the profile: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gmail: reading the profile answered %s", res.Status)
	}
	return parseProfileAddress(body)
}

// parseProfileAddress reads the account's address from a profile response.
func parseProfileAddress(body []byte) (string, error) {
	var raw struct {
		EmailAddress string `json:"emailAddress"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("gmail: reading the profile: %w", err)
	}
	if raw.EmailAddress == "" {
		return "", errors.New("gmail: the profile has no address")
	}
	return raw.EmailAddress, nil
}

// RequireAccount refuses a grant whose account, the address the account's profile gives, is not the
// account wanted. Addresses are compared ignoring case. The error names neither address, since the
// account a grant belongs to may be anyone's and the contract suite's run logs it.
func RequireAccount(granted, wanted string) error {
	if wanted == "" || !strings.EqualFold(granted, wanted) {
		return errors.New("gmail: the grant belongs to an account other than the one wanted")
	}
	return nil
}
