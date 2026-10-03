package consent

import (
	"errors"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// A client check is a code exchange for a code no consent issued, with the client, a verifier and the
// redirect address, so Google checks the client before refusing the code.
func TestTheClientCheckRequest(t *testing.T) {
	want := url.Values{
		"client_id":     {"client-id"},
		"client_secret": {"client-secret"},
		"code":          {"mediated-mailbox-client-check"},
		"code_verifier": {"mediated-mailbox-client-check-verifier-0000000"},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {"http://127.0.0.1:47823/"},
	}
	if diff := cmp.Diff(want, checkForm("client-id", "client-secret", "http://127.0.0.1:47823/"), compare.Options); diff != "" {
		t.Errorf("client check request (-want +got):\n%s", diff)
	}
	if n := len(checkForm("", "", "").Get("code_verifier")); n < 43 || n > 128 {
		t.Errorf("the verifier is %d characters, outside PKCE's 43 to 128", n)
	}
}

// Google's answer to a client check, from bodies shaped as RFC 6749 section 5.2 and Google document
// them. A refused code means the client was accepted, a refused client means it was not, and anything
// else is no answer the check can read.
func TestTheClientChecksAnswer(t *testing.T) {
	answered := func(code int, status, body string) error {
		_, err := Answer(code, status, []byte(body))
		return err
	}
	if err := checkAnswer(answered(400, "400 Bad Request", `{"error": "invalid_grant", "error_description": "Malformed auth code."}`)); err != nil {
		t.Errorf("a refused code refused the client: %v", err)
	}
	for name, err := range map[string]error{
		"unknown client":   answered(401, "401 Unauthorized", `{"error": "invalid_client", "error_description": "The OAuth client was not found."}`),
		"wrong secret":     answered(401, "401 Unauthorized", `{"error": "invalid_client", "error_description": "Unauthorized"}`),
		"deleted client":   answered(401, "401 Unauthorized", `{"error": "deleted_client"}`),
		"refused request":  answered(400, "400 Bad Request", `{"error": "invalid_request", "error_description": "client_secret is missing."}`),
		"refusal, no body": answered(400, "400 Bad Request", `<html></html>`),
	} {
		if got := checkAnswer(err); !errors.Is(got, mail.ErrClientRefused) {
			t.Errorf("%s: %v, want %v", name, got, mail.ErrClientRefused)
		}
	}
	for name, err := range map[string]error{
		"server error":      answered(500, "500 Internal Server Error", `{"error": "internal_failure"}`),
		"no answer":         errors.New("dial tcp: connection refused"),
		"a code exchanged":  nil,
		"too many requests": answered(429, "429 Too Many Requests", ``),
	} {
		got := checkAnswer(err)
		if got == nil || errors.Is(got, mail.ErrClientRefused) {
			t.Errorf("%s: %v, want an error that is not %v", name, got, mail.ErrClientRefused)
		}
	}
}

// A profile read Google refuses because the Gmail API is not enabled for the client's project is
// told apart from any other refusal, from bodies in the shape Google's errors take.
func TestAProfileReadOfADisabledAPI(t *testing.T) {
	disabled := map[string]string{
		"the list's reason":   `{"error": {"code": 403, "message": "Gmail API has not been used in project 1 before or it is disabled.", "errors": [{"reason": "accessNotConfigured"}], "status": "PERMISSION_DENIED"}}`,
		"the details' reason": `{"error": {"code": 403, "status": "PERMISSION_DENIED", "details": [{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "SERVICE_DISABLED"}]}}`,
	}
	for name, body := range disabled {
		if err := profileRefusal(403, "403 Forbidden", []byte(body)); !errors.Is(err, mail.ErrAPIDisabled) {
			t.Errorf("%s: %v, want %v", name, err, mail.ErrAPIDisabled)
		}
	}
	for name, c := range map[string]struct {
		code int
		body string
	}{
		"another 403":          {403, `{"error": {"code": 403, "errors": [{"reason": "insufficientPermissions"}]}}`},
		"the reason on a 401":  {401, `{"error": {"code": 401, "errors": [{"reason": "accessNotConfigured"}]}}`},
		"a 403 without a body": {403, ``},
	} {
		if err := profileRefusal(c.code, "status", []byte(c.body)); err == nil || errors.Is(err, mail.ErrAPIDisabled) {
			t.Errorf("%s: %v, want an error that is not %v", name, err, mail.ErrAPIDisabled)
		}
	}
}

// A grant without the modify scope lacks it, and one holding the modify scope with any other holds a
// scope beside it, so the UI can say which (docs/UI.md section 8.12).
func TestAGrantsScopeIsToldApart(t *testing.T) {
	for name, scope := range map[string]string{
		"no scope":                     "",
		"the full mail scope":          "https://mail.google.com/",
		"read-only in place of modify": "https://www.googleapis.com/auth/gmail.readonly",
	} {
		if err := onlyModify(scope); !errors.Is(err, mail.ErrScopeMissing) {
			t.Errorf("%s: %v, want %v", name, err, mail.ErrScopeMissing)
		}
	}
	for name, scope := range map[string]string{
		"modify and the full scope": "https://www.googleapis.com/auth/gmail.modify https://mail.google.com/",
		"modify twice":              "https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/gmail.modify",
	} {
		if err := onlyModify(scope); !errors.Is(err, mail.ErrScopeRefused) {
			t.Errorf("%s: %v, want %v", name, err, mail.ErrScopeRefused)
		}
	}
}
