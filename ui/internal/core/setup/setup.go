// Package setup is the pure rules of the UI's OAuth client setup and account setup (docs/UI.md
// sections 8.11 to 8.13). The shell reads the request and the stored records, and these decide what
// is refused and why, as values, each refusal nil for none (ADR-0040, docs/UI.md section 17.4).
package setup

import (
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
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
func ClientName(name string) *refusal.Refusal {
	if !ProjectShaped(name) {
		return refusal.Of(refusal.NameRefused, "a client's name is 6 to 30 lowercase letters, digits and hyphens starting with a letter, and never new")
	}
	return nil
}

// ProjectID refuses a project ID that is given and not shaped like one. A client may be stored with
// none.
func ProjectID(id *string) *refusal.Refusal {
	if id != nil && !ProjectShaped(*id) {
		return refusal.Of(refusal.ProjectRefused, "a project ID is 6 to 30 lowercase letters, digits and hyphens starting with a letter")
	}
	return nil
}

// Identifier refuses an account identifier outside the grammar of one lowercase DNS label, one to
// 63 lowercase ASCII letters, digits and hyphens starting and ending with a letter or a digit, which
// the database's check on accounts.account_id also holds, so every identifier is a path segment the
// API server's API root and the UI's pages can address (ADR-0087, ADR-0137) and legible wherever it
// spreads (ADR-0128). It reads id as it arrives, trimming and folding nothing, so it accepts nothing
// the database refuses. An identifier another account holds is refused by the store.
func Identifier(id string) *refusal.Refusal {
	if !identifierShaped(id) {
		return IdentifierRefusal()
	}
	return nil
}

// IdentifierRefusal is the refusal of an identifier outside the grammar, which names the grammar.
// The database's check on the identifier refuses one the same way (docs/UI.md section 8.12).
func IdentifierRefusal() *refusal.Refusal {
	return refusal.Of(refusal.IdentifierRefused, "an account identifier is 1 to 63 lowercase letters, digits and hyphens, starting and ending with a letter or a digit")
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
func Mailbox(address string) *refusal.Refusal {
	at := strings.LastIndex(address, "@")
	if at <= 0 || at == len(address)-1 || strings.TrimSpace(address) != address {
		return refusal.Of(refusal.MailboxRefused, "the mailbox is an address with a local part and a domain")
	}
	return nil
}

// Target refuses a lowered target outside the range ADR-0024 allows, above 5% and at most 50% of the
// provider's declared ceiling. None clears the target.
func Target(fraction *float64) *refusal.Refusal {
	if fraction != nil && (!(*fraction > 0.05) || *fraction > 0.5) {
		return refusal.Of(refusal.TargetRefused, "a lowered target is above 5% and at most 50% of the declared ceiling")
	}
	return nil
}

// Expired refuses an attempt that expires at expires, an instant in milliseconds, at now. A finish
// decides it before it reads the pasted address, so an attempt past its expiry answers as expired
// whatever was pasted (ADR-0111).
func Expired(expires, now int64) *refusal.Refusal {
	if now >= expires {
		return refusal.Of(refusal.AttemptExpired, "the attempt expired")
	}
	return nil
}

// Redirect decides a pasted address's redirect against the attempt it should finish, whose state is
// state (docs/UI.md section 8.12). A redirect carrying a code or an error under another state belongs
// to another attempt. Then a redirect carrying an error was declined at the provider, and one carrying
// neither has no code. The address was already read as the consent's redirect address, and the attempt
// was already found unexpired.
func Redirect(r mail.Redirect, state string) *refusal.Refusal {
	const message = "the pasted address cannot finish this attempt"
	switch {
	case r.State != state && (r.Code != "" || r.Error != ""):
		return refusal.Of(refusal.WrongAttempt, message)
	case r.Error != "":
		return refusal.Of(refusal.ConsentDeclined, message)
	case r.Code == "":
		return refusal.Of(refusal.NoCode, message)
	}
	return nil
}

// Client is one stored OAuth client's identity, its name, its provider and its identifier at the
// provider.
type Client struct {
	Name, Provider, ID string
}

// NewClient refuses a client's identifier or secret left empty, and a project ID not shaped like one,
// for a client set up or replaced.
func NewClient(id, secret string, project *string) *refusal.Refusal {
	if id == "" || secret == "" {
		return refusal.Of(refusal.ClientRefused, "the client identifier and the client secret are both required")
	}
	return ProjectID(project)
}

// Added refuses a client set up under a name a stored client holds, or with an identifier a stored
// client of its provider holds, reading stored in order, so the first stored client either refusal
// names decides it.
func Added(stored []Client, c Client) *refusal.Refusal {
	for _, s := range stored {
		if s.Name == c.Name {
			return refusal.Of(refusal.NameTaken, "another client holds this name")
		}
		if s.Provider == c.Provider && s.ID == c.ID {
			return refusal.Of(refusal.ClientExists, "this client is already set up as "+s.Name)
		}
	}
	return nil
}

// Replaced refuses a replacement of a client no stored client of its provider holds the name of, and
// one to an identifier another stored client of the provider holds.
func Replaced(stored []Client, c Client) *refusal.Refusal {
	if !slices.ContainsFunc(stored, func(s Client) bool { return s.Name == c.Name && s.Provider == c.Provider }) {
		return UnknownClient()
	}
	for _, s := range stored {
		if s.Name != c.Name && s.Provider == c.Provider && s.ID == c.ID {
			return refusal.Of(refusal.ClientExists, "this client is already set up as "+s.Name)
		}
	}
	return nil
}

// UnknownClient is the refusal of a client no stored client of the provider holds the name of.
func UnknownClient() *refusal.Refusal {
	return refusal.Of(refusal.UnknownClient, "no client of this provider holds this name")
}

// The constraints whose refusals name a client mistake (ADR-0016, ADR-0106).
const (
	ClientNameKey = "oauth_clients_pkey"
	ClientIDKey   = "oauth_clients_provider_client_id_key"
	AccountKey    = "accounts_pkey"
)

// ClientConflict decides a client the table's keys refused when another setup stored one meanwhile,
// by the key that refused it, and is nil for a key that names no client mistake.
func ClientConflict(key string) *refusal.Refusal {
	switch key {
	case ClientNameKey:
		return refusal.Of(refusal.NameTaken, "another client holds this name")
	case ClientIDKey:
		return refusal.Of(refusal.ClientExists, "this client is already set up under another name")
	}
	return nil
}

// Named returns the stored client holding name whose provider is one of providers, those a consent
// is held for, and refuses a name no such client holds.
func Named(stored []Client, name string, providers []string) (Client, *refusal.Refusal) {
	for _, c := range stored {
		if c.Name == name && slices.Contains(providers, c.Provider) {
			return c, nil
		}
	}
	return Client{}, refusal.Of(refusal.UnknownClient, "no client holds this name")
}

// What an attempt does, connect an account, re-authorize one through its own client, or move one to
// another client of its provider.
const (
	Connect     = "connect"
	Reauthorize = "reauthorize"
	Move        = "move"
)

// Unclaimed refuses a connection under an identifier one of listed, the accounts listing, holds.
func Unclaimed(listed []string, account string) *refusal.Refusal {
	if slices.Contains(listed, account) {
		return refusal.Of(refusal.IdentifierTaken, "an account already holds this identifier")
	}
	return nil
}

// Held is what account setup reads of an account a re-authorization starts for, its provider, the
// name of the client it connects through, empty for none, and the mailbox it remembers, nil for none.
type Held struct {
	Provider string
	Client   string
	Mailbox  *string
}

// ClientOf returns the name of the client a re-authorization goes through, requested when the request
// names one and the account's own client otherwise, and refuses an account that connects through
// none when the request names none.
func ClientOf(held Held, requested *string) (string, *refusal.Refusal) {
	name := held.Client
	if requested != nil {
		name = *requested
	}
	if name == "" {
		return "", refusal.Of(refusal.UnknownClient, "the account connects through no client")
	}
	return name, nil
}

// Reauthorization decides a re-authorization of held through client, and returns its kind, a move
// when the client is not the account's own, and the mailbox the grant is checked against. A client
// of another provider is refused. The mailbox is the one the account remembers, and a request naming
// one while the account remembers its own is refused, as is one naming none while it remembers none,
// whose named mailbox is checked as a connection's is (ADR-0080, ADR-0106).
func Reauthorization(held Held, client Client, mailbox *string) (kind, address string, r *refusal.Refusal) {
	if client.Provider != held.Provider {
		return "", "", refusal.Of(refusal.ClientWrongProvider, "the client belongs to another provider than the account's")
	}
	switch {
	case held.Mailbox != nil && mailbox != nil:
		return "", "", refusal.Of(refusal.MailboxRemembered, "the account remembers its mailbox, so the request names none")
	case held.Mailbox == nil && mailbox == nil:
		return "", "", refusal.Of(refusal.MailboxRequired, "the account remembers no mailbox, so the request names it")
	case held.Mailbox == nil:
		if r := Mailbox(*mailbox); r != nil {
			return "", "", r
		}
		address = *mailbox
	default:
		address = *held.Mailbox
	}
	kind = Reauthorize
	if client.Name != held.Client {
		kind = Move
	}
	return kind, address, nil
}

// StillFree refuses, before a finish exchanges its code, a connection's identifier one of listed, the
// accounts listing, holds, since an account took it after the attempt started, and an attempt whose
// client no stored client holds as it was when the attempt started, since it was replaced or removed.
// listed is read for a connection alone.
func StillFree(kind, account string, listed []string, stored []Client, attempt Client) *refusal.Refusal {
	if kind == Connect && slices.Contains(listed, account) {
		return IdentifierTaken()
	}
	if !slices.Contains(stored, attempt) {
		return refusal.Of(refusal.ClientChanged, "the client "+attempt.Name+" changed while this was in progress")
	}
	return nil
}

// IdentifierTaken is the refusal of a connection whose identifier an account took while it was in
// progress.
func IdentifierTaken() *refusal.Refusal {
	return refusal.Of(refusal.IdentifierTaken, "an account was connected under this identifier while this one was in progress")
}

// ClientChanged reports whether the client a finish writes through changed since its attempt
// started, read again in the transaction that writes, so a client replaced or removed meanwhile
// writes nothing. identity is the stored client of the attempt's name, none when it was removed.
func ClientChanged(identity []Client, attempt Client) bool {
	return len(identity) != 1 || identity[0] != attempt
}

// Violation is the database's refusal of a write by a constraint, the constraint's name and whether
// it was a unique key's or a check's.
type Violation struct {
	Constraint    string
	Unique, Check bool
}

// Stored decides a finish's transaction that stored nothing. A client changed meanwhile, an
// identifier an account took meanwhile, refused by the accounts key, and an identifier the
// database's check refuses are each the client's, and anything else is the database's, nil.
func Stored(clientChanged bool, v Violation) *refusal.Refusal {
	switch {
	case clientChanged:
		return refusal.Of(refusal.ClientChanged, "the attempt's client changed while this was in progress")
	case v.Unique && v.Constraint == AccountKey:
		return IdentifierTaken()
	case v.Check:
		return IdentifierRefusal()
	}
	return nil
}
