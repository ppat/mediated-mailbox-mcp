// Package setup is the pure rules of the UI's OAuth client setup and account setup (docs/UI.md
// sections 8.11 to 8.13). The shell reads the request and the stored records, and these decide what
// is refused and why, as values (ADR-0040).
package setup

import (
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Refusal is a refused request's code in the error contract, empty for none (docs/UI.md section
// 17.4).
type Refusal string

// The refusals these rules decide.
const (
	None              Refusal = ""
	NameRefused       Refusal = "name_refused"
	ProjectRefused    Refusal = "project_refused"
	IdentifierRefused Refusal = "identifier_refused"
	TargetRefused     Refusal = "target_refused"
	MailboxRefused    Refusal = "mailbox_refused"
	ConsentDeclined   Refusal = "consent_declined"
	NoCode            Refusal = "no_code"
	WrongAttempt      Refusal = "wrong_attempt"
	AttemptExpired    Refusal = "attempt_expired"
)

// ProjectShaped reports whether s has the shape of a Google Cloud project ID, 6 to 30 lowercase
// letters, digits and hyphens starting with a letter (docs/UI.md section 8.11).
func ProjectShaped(s string) bool {
	if len(s) < 6 || len(s) > 30 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// ClientName refuses a name not shaped like a project ID, so a client's name is always one path
// segment its screens can be reached at (docs/UI.md sections 5 and 8.11). That refuses new, the path
// segment of a new client's setup, since it is too short to be a project ID. A name another client
// holds is refused by the store.
func ClientName(name string) Refusal {
	if !ProjectShaped(name) {
		return NameRefused
	}
	return None
}

// ProjectID refuses a project ID that is given and not shaped like one. A client may be stored with
// none.
func ProjectID(id *string) Refusal {
	if id != nil && !ProjectShaped(*id) {
		return ProjectRefused
	}
	return None
}

// Identifier refuses an account identifier outside the grammar of one lowercase DNS label, one to
// 63 lowercase ASCII letters, digits and hyphens starting and ending with a letter or a digit, which
// the database's check on accounts.account_id also holds, so every identifier is a path segment the
// API server's API root and the UI's pages can address (ADR-0087, ADR-0137) and legible wherever it
// spreads (ADR-0128). It reads id as it arrives, trimming and folding nothing, so it accepts nothing
// the database refuses. An identifier another account holds is refused by the store.
func Identifier(id string) Refusal {
	if !identifierShaped(id) {
		return IdentifierRefused
	}
	return None
}

// identifierShaped reports whether id is inside the account identifier grammar,
// ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$.
func identifierShaped(id string) bool {
	if len(id) < 1 || len(id) > 63 || id[0] == '-' || id[len(id)-1] == '-' {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// Mailbox refuses a mailbox address with no local part or no domain.
func Mailbox(address string) Refusal {
	at := strings.LastIndex(address, "@")
	if at <= 0 || at == len(address)-1 || strings.TrimSpace(address) != address {
		return MailboxRefused
	}
	return None
}

// Target refuses a lowered target outside the range ADR-0024 allows, above 5% and at most 50% of the
// provider's declared ceiling. None clears the target.
func Target(fraction *float64) Refusal {
	if fraction != nil && (!(*fraction > 0.05) || *fraction > 0.5) {
		return TargetRefused
	}
	return None
}

// Expired refuses an attempt that expires at expires, an instant in milliseconds, at now. A finish
// decides it before it reads the pasted address, so an attempt past its expiry answers as expired
// whatever was pasted (ADR-0111).
func Expired(expires, now int64) Refusal {
	if now >= expires {
		return AttemptExpired
	}
	return None
}

// Redirect decides a pasted address's redirect against the attempt it should finish, whose state is
// state (docs/UI.md section 8.12). A redirect carrying a code or an error under another state belongs
// to another attempt. Then a redirect carrying an error was declined at the provider, and one carrying
// neither has no code. The address was already read as the consent's redirect address, and the attempt
// was already found unexpired.
func Redirect(r mail.Redirect, state string) Refusal {
	switch {
	case r.State != state && (r.Code != "" || r.Error != ""):
		return WrongAttempt
	case r.Error != "":
		return ConsentDeclined
	case r.Code == "":
		return NoCode
	}
	return None
}
