package gmail

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Consent runs the one-time interactive consent for one account and returns its refresh token. An
// operator runs it outside every deployable, since no deployable obtains a credential itself
// (ADR-0038). It prints the consent page's address to prompt, receives Google's redirect on a
// loopback address, and exchanges the code for the refresh token.
func Consent(ctx context.Context, client *http.Client, clientID, clientSecret string, prompt io.Writer) (string, error) {
	state, verifier := rand.Text(), rand.Text()+rand.Text()
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("gmail: listening for the consent redirect: %w", err)
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
	if _, err = fmt.Fprintf(prompt, "Open this address and grant access:\n%s\n", AuthorizationURL(clientID, redirectURI, state, verifier)); err == nil {
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
		return "", err
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
