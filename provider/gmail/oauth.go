package gmail

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Scope is the one OAuth scope the grant requests (ADR-0011). It permits applying labels, which
// gmail.readonly with gmail.labels does not, and it leaves out permanent delete, which only the
// full https://mail.google.com/ scope grants. That absence is one of the two structural halves of
// ADR-0019's rule that mail is never permanently deleted.
const Scope = "https://www.googleapis.com/auth/gmail.modify"

// Google's endpoints for installed-app OAuth.
const (
	authEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenEndpoint = "https://oauth2.googleapis.com/token" //nolint:gosec // an endpoint URL, not a credential
)

// maxTokenResponse bounds how much of a token endpoint response is read.
const maxTokenResponse = 1 << 20

// AuthorizationURL is the consent page the operator opens once per account. It requests Scope and
// nothing else, and it leaves out include_granted_scopes, which would fold every scope the account
// ever granted this client into the new token. access_type=offline and prompt=consent make Google
// return a refresh token even when the account consented before. The code challenge is the S256
// PKCE challenge of verifier. A login hint, when given, is the address of the account the grant is
// meant for, so Google offers that account first.
func AuthorizationURL(clientID, redirectURI, state, verifier, loginHint string) string {
	challenge := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"access_type":           {"offline"},
		"client_id":             {clientID},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
		"prompt":                {"consent"},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {Scope},
		"state":                 {state},
	}
	if loginHint != "" {
		q.Set("login_hint", loginHint)
	}
	return authEndpoint + "?" + q.Encode()
}

// Grant is what the consent returns, the account's refresh token and a first access token.
type Grant struct {
	RefreshToken string
	AccessToken  string
}

// ExchangeCode trades the authorization code the consent page returned for the account's grant. It
// refuses a grant holding any scope but Scope, so the token is known to lack permanent delete
// rather than assumed to (ADR-0011). The operator stores the refresh token in the secret store,
// from which it reaches the deployables as a mounted file (ADR-0038).
func ExchangeCode(ctx context.Context, client *http.Client, clientID, clientSecret, code, verifier, redirectURI string) (Grant, error) {
	body, err := postForm(ctx, client, exchangeForm(clientID, clientSecret, code, verifier, redirectURI))
	if err != nil {
		return Grant{}, err
	}
	return grantFrom(body)
}

// grantFrom reads the code exchange's response body as a grant. It refuses a response holding any
// scope but Scope, or no refresh token.
func grantFrom(body []byte) (Grant, error) {
	resp, err := parseTokenResponse(body)
	if err != nil {
		return Grant{}, err
	}
	if err := onlyModify(resp.Scope); err != nil {
		return Grant{}, err
	}
	if resp.RefreshToken == "" {
		return Grant{}, errors.New("gmail: the consent returned no refresh token")
	}
	return Grant{RefreshToken: resp.RefreshToken, AccessToken: resp.AccessToken}, nil
}

// onlyModify refuses a granted scope list that is anything but Scope alone. Google lists the
// granted scopes separated by spaces.
func onlyModify(scope string) error {
	if granted := strings.Fields(scope); len(granted) != 1 || granted[0] != Scope {
		return fmt.Errorf("gmail: the consent granted the scopes %q, and only %s is accepted", granted, Scope)
	}
	return nil
}

// exchangeForm is the token request that trades an authorization code. The verifier binds it to
// the consent page's PKCE challenge, and the redirect address must be the one the consent used.
func exchangeForm(clientID, clientSecret, code, verifier, redirectURI string) url.Values {
	return url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"code_verifier": {verifier},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}
}

// tokenResponse is the part of Google's token endpoint response this package uses. A refresh
// response carries a refresh token only when Google rotated it. Scope lists the granted scopes.
type tokenResponse struct {
	AccessToken  string
	ExpiresIn    time.Duration
	RefreshToken string
	Scope        string
}

// parseTokenResponse reads a token endpoint response body.
func parseTokenResponse(body []byte) (tokenResponse, error) {
	var raw struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return tokenResponse{}, fmt.Errorf("gmail: reading the token response: %w", err)
	}
	if raw.AccessToken == "" {
		return tokenResponse{}, errors.New("gmail: the token response holds no access token")
	}
	if raw.ExpiresIn <= 0 {
		return tokenResponse{}, errors.New("gmail: the token response holds no lifetime")
	}
	return tokenResponse{
		AccessToken:  raw.AccessToken,
		ExpiresIn:    time.Duration(raw.ExpiresIn) * time.Second,
		RefreshToken: raw.RefreshToken,
		Scope:        raw.Scope,
	}, nil
}

// postForm posts a form to the token endpoint and returns the body of a successful response. An
// error names Google's error code and description, which carry no credential.
func postForm(ctx context.Context, client *http.Client, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("gmail: building the token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gmail: requesting a token: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxTokenResponse))
	if err := errors.Join(readErr, res.Body.Close()); err != nil {
		return nil, fmt.Errorf("gmail: reading the token response: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		var e struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		if json.Unmarshal(body, &e) != nil {
			return nil, fmt.Errorf("gmail: the token endpoint answered %s", res.Status)
		}
		return nil, fmt.Errorf("gmail: the token endpoint answered %s: %s %s", res.Status, e.Error, e.Description)
	}
	return body, nil
}
