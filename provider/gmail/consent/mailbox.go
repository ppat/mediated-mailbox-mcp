package consent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// profileURL is Gmail's profile of the account an access token belongs to, which names the mailbox.
const profileURL = "https://gmail.googleapis.com/gmail/v1/users/me/profile"

// maxProfileResponse bounds how much of a profile response is read.
const maxProfileResponse = 1 << 20

// Address returns the address of the account an access token belongs to, read from the account's
// profile. It is how the UI, the operator running the consent command and the contract suite's run
// against Gmail confirm which mailbox a grant is for. It returns mail.ErrAPIDisabled when Google
// answers that the Gmail API is not enabled for the client's project.
func Address(ctx context.Context, client *http.Client, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gmail: reading the profile: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxProfileResponse))
	if err := errors.Join(readErr, res.Body.Close()); err != nil {
		return "", fmt.Errorf("gmail: reading the profile: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return "", profileRefusal(res.StatusCode, res.Status, body)
	}
	return parseProfileAddress(body)
}

// profileRefusal is the error for a profile read Google did not answer with the profile. A 403 whose
// reason says the API is not enabled for the project is mail.ErrAPIDisabled. Google names that reason
// as accessNotConfigured in the error's list and as SERVICE_DISABLED in its details, as its error
// documents for a disabled API show, and the live check of the client setup confirms the shape.
func profileRefusal(code int, status string, body []byte) error {
	var raw struct {
		Error struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
			Details []struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	if code == http.StatusForbidden && json.Unmarshal(body, &raw) == nil {
		var reasons []string
		for _, e := range raw.Error.Errors {
			reasons = append(reasons, e.Reason)
		}
		for _, d := range raw.Error.Details {
			reasons = append(reasons, d.Reason)
		}
		if slices.Contains(reasons, "accessNotConfigured") || slices.Contains(reasons, "SERVICE_DISABLED") {
			return fmt.Errorf("gmail: reading the profile answered %s: %w", status, mail.ErrAPIDisabled)
		}
	}
	return fmt.Errorf("gmail: reading the profile answered %s", status)
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

// SameMailbox reports whether two addresses name one Gmail mailbox, as Google identifies one. Case is
// ignored everywhere, and for gmail.com and googlemail.com the dots of the local part are ignored and
// the two domains are one, since Google delivers each such spelling to the same mailbox (docs/UI.md
// section 8.12). An empty address, or one with no local part or no domain, names no mailbox.
func SameMailbox(a, b string) bool {
	x, okA := canonical(a)
	y, okB := canonical(b)
	return okA && okB && x == y
}

// canonical is an address in the one spelling SameMailbox compares.
func canonical(address string) (string, bool) {
	at := strings.LastIndex(address, "@")
	if at <= 0 || at == len(address)-1 {
		return "", false
	}
	local, domain := strings.ToLower(address[:at]), strings.ToLower(address[at+1:])
	if domain == "googlemail.com" {
		domain = "gmail.com"
	}
	if domain == "gmail.com" {
		local = strings.ReplaceAll(local, ".", "")
	}
	return local + "@" + domain, true
}

// RequireAccount refuses a grant whose account, the address the account's profile gives, is not the
// account wanted, compared as SameMailbox compares. The error names neither address, since the
// account a grant belongs to may be anyone's and the contract suite's run logs it.
func RequireAccount(granted, wanted string) error {
	if !SameMailbox(granted, wanted) {
		return errors.New("gmail: the grant belongs to an account other than the one wanted")
	}
	return nil
}
