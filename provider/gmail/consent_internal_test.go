package gmail

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// redirect sends one request to the consent handler over a real loopback connection and returns
// the status and whatever the handler handed over.
func redirect(t *testing.T, query url.Values) (int, *consentResult) {
	t.Helper()
	results := make(chan consentResult, 1)
	srv := httptest.NewServer(consentHandler("the-state", results))
	defer srv.Close()
	res, err := srv.Client().Get(srv.URL + "/?" + query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Body.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-results:
		return res.StatusCode, &r
	default:
		return res.StatusCode, nil
	}
}

// Only a redirect carrying the consent's state counts, so a request another page forges is
// refused and hands nothing over.
func TestTheConsentRedirectRefusesAWrongOrMissingState(t *testing.T) {
	for name, query := range map[string]url.Values{
		"wrong state":            {"state": {"another-state"}, "code": {"the-code"}},
		"missing state":          {"code": {"the-code"}},
		"error with wrong state": {"state": {"another-state"}, "error": {"access_denied"}},
	} {
		t.Run(name, func(t *testing.T) {
			status, got := redirect(t, query)
			if status != http.StatusBadRequest {
				t.Errorf("status %d, want %d", status, http.StatusBadRequest)
			}
			if got != nil {
				t.Errorf("the handler handed over %+v", *got)
			}
		})
	}
}

// A redirect with the right state that carries an error, or no code at all, ends the consent
// without a code.
func TestTheConsentRedirectRefusesAnErrorOrNoCode(t *testing.T) {
	for name, query := range map[string]url.Values{
		"error":   {"state": {"the-state"}, "error": {"access_denied"}},
		"no code": {"state": {"the-state"}},
	} {
		t.Run(name, func(t *testing.T) {
			status, got := redirect(t, query)
			if status != http.StatusBadRequest {
				t.Errorf("status %d, want %d", status, http.StatusBadRequest)
			}
			if got == nil || got.err == nil || got.code != "" {
				t.Errorf("the handler handed over %+v, want an error and no code", got)
			}
		})
	}
}

// A redirect with the right state hands over its code.
func TestTheConsentRedirectHandsOverTheCode(t *testing.T) {
	status, got := redirect(t, url.Values{"state": {"the-state"}, "code": {"the-code"}})
	if status != http.StatusOK {
		t.Errorf("status %d, want %d", status, http.StatusOK)
	}
	if got == nil || got.err != nil || got.code != "the-code" {
		t.Errorf("the handler handed over %+v, want the code", got)
	}
}

// A grant is accepted only for the account wanted, whatever the case of either address, and the
// refusal names neither address.
func TestRequireAccount(t *testing.T) {
	for _, granted := range []string{"test@example.com", "Test@Example.COM"} {
		if err := RequireAccount(granted, "test@example.com"); err != nil {
			t.Errorf("RequireAccount(%q) refused the wanted account: %v", granted, err)
		}
	}
	for name, c := range map[string]struct{ granted, wanted string }{
		"another account":             {"someone@example.com", "test@example.com"},
		"an account sharing a prefix": {"test@example.com.evil", "test@example.com"},
		"no account wanted":           {"test@example.com", ""},
		"no account granted":          {"", "test@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			err := RequireAccount(c.granted, c.wanted)
			if err == nil {
				t.Fatalf("RequireAccount accepted %s", name)
			}
			for _, address := range []string{c.granted, c.wanted} {
				if address != "" && strings.Contains(err.Error(), address) {
					t.Errorf("the refusal %q names the address %q", err, address)
				}
			}
		})
	}
}
