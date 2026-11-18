// Package refusal is the client's half of the UI's error contract, a refused request as a value
// (docs/UI.md section 17.3, ADR-0040). A refusal carries the code that says why the request was
// refused, the message that words it, and, for a refused policy write or file, the problems that
// say what is wrong with each rule or with the file. The code and a problem's kind are two levels of
// the contract, so each is its own type. The pure cores return a refusal and the shell answers it.
package refusal

// Code is why a request was refused, the error contract's code. The browser branches on some codes,
// so a code's string never changes.
type Code string

// The codes a refused request carries, every one the UI answers with.
const (
	AllAccounts          Code = "all_accounts"
	APIDisabled          Code = "api_disabled"
	AttemptExpired       Code = "attempt_expired"
	ClientChanged        Code = "client_changed"
	ClientExists         Code = "client_exists"
	ClientInUse          Code = "client_in_use"
	ClientRefused        Code = "client_refused"
	ClientWrongProvider  Code = "client_wrong_provider"
	CodeRefused          Code = "code_refused"
	ConfirmationRequired Code = "confirmation_required"
	ConsentDeclined      Code = "consent_declined"
	FileRefused          Code = "file_refused"
	IdentifierRefused    Code = "identifier_refused"
	IdentifierTaken      Code = "identifier_taken"
	IdentityMissing      Code = "identity_missing"
	InvalidFilter        Code = "invalid_filter"
	InvalidLevel         Code = "invalid_level"
	InvalidPage          Code = "invalid_page"
	InvalidRange         Code = "invalid_range"
	InvalidRow           Code = "invalid_row"
	InvalidSort          Code = "invalid_sort"
	MailboxRefused       Code = "mailbox_refused"
	MailboxRemembered    Code = "mailbox_remembered"
	MailboxRequired      Code = "mailbox_required"
	MalformedBody        Code = "malformed_body"
	MissingDataset       Code = "missing_dataset"
	MissingFilter        Code = "missing_filter"
	MissingGroup         Code = "missing_group"
	MissingParent        Code = "missing_parent"
	NameRefused          Code = "name_refused"
	NameTaken            Code = "name_taken"
	NoAttempt            Code = "no_attempt"
	NoCode               Code = "no_code"
	NotConnected         Code = "not_connected"
	NotFound             Code = "not_found"
	ProjectRefused       Code = "project_refused"
	RepeatedParameter    Code = "repeated_parameter"
	RuleRefused          Code = "rule_refused"
	ScopeMissing         Code = "scope_missing"
	ScopeRefused         Code = "scope_refused"
	StalePage            Code = "stale_page"
	StalePreview         Code = "stale_preview"
	StaleRule            Code = "stale_rule"
	TargetRefused        Code = "target_refused"
	UnknownAccount       Code = "unknown_account"
	UnknownClient        Code = "unknown_client"
	UnknownDataset       Code = "unknown_dataset"
	UnknownDimension     Code = "unknown_dimension"
	UnknownGroup         Code = "unknown_group"
	UnknownParameter     Code = "unknown_parameter"
	UnknownProvider      Code = "unknown_provider"
	UnknownRow           Code = "unknown_row"
	UnknownRule          Code = "unknown_rule"
	UnknownRun           Code = "unknown_run"
	UnknownSort          Code = "unknown_sort"
	WrongAddress         Code = "wrong_address"
	WrongAttempt         Code = "wrong_attempt"
	WrongMailbox         Code = "wrong_mailbox"
)

// Codes returns every code, in the order declared.
func Codes() []Code {
	return []Code{
		AllAccounts,
		APIDisabled,
		AttemptExpired,
		ClientChanged,
		ClientExists,
		ClientInUse,
		ClientRefused,
		ClientWrongProvider,
		CodeRefused,
		ConfirmationRequired,
		ConsentDeclined,
		FileRefused,
		IdentifierRefused,
		IdentifierTaken,
		IdentityMissing,
		InvalidFilter,
		InvalidLevel,
		InvalidPage,
		InvalidRange,
		InvalidRow,
		InvalidSort,
		MailboxRefused,
		MailboxRemembered,
		MailboxRequired,
		MalformedBody,
		MissingDataset,
		MissingFilter,
		MissingGroup,
		MissingParent,
		NameRefused,
		NameTaken,
		NoAttempt,
		NoCode,
		NotConnected,
		NotFound,
		ProjectRefused,
		RepeatedParameter,
		RuleRefused,
		ScopeMissing,
		ScopeRefused,
		StalePage,
		StalePreview,
		StaleRule,
		TargetRefused,
		UnknownAccount,
		UnknownClient,
		UnknownDataset,
		UnknownDimension,
		UnknownGroup,
		UnknownParameter,
		UnknownProvider,
		UnknownRow,
		UnknownRule,
		UnknownRun,
		UnknownSort,
		WrongAddress,
		WrongAttempt,
		WrongMailbox,
	}
}

// Kind is what is wrong with one rule of a write or a file, or with a file as a whole, inside a
// refusal whose code is RuleRefused or FileRefused.
type Kind string

// The kinds of problem. The first six are a rule's, the policy snapshot's validation's and the UI's
// own, and the last two a file's as a whole.
const (
	BlankIdentifier    Kind = "blank_identifier"
	RepeatedIdentifier Kind = "repeated_identifier"
	NoSuffix           Kind = "no_suffix"
	InvalidSuffix      Kind = "invalid_suffix"
	OtherClass         Kind = "other_class"
	ReservedIdentifier Kind = "reserved_identifier"
	NotTheForm         Kind = "not_the_form"
	TooLarge           Kind = "too_large"
)

// Kinds returns every kind, in the order declared.
func Kinds() []Kind {
	return []Kind{BlankIdentifier, RepeatedIdentifier, NoSuffix, InvalidSuffix, OtherClass, ReservedIdentifier, NotTheForm, TooLarge}
}

// Problem is one problem inside a refusal. Rule is the identifier of the rule it is about, Line the
// line of a file it is at and Suffix the suffix an invalid suffix names, each empty or 0 for none.
type Problem struct {
	Kind   Kind
	Rule   string
	Line   int
	Suffix string
}

// Refusal is a refused request.
type Refusal struct {
	Code    Code
	Message string
	// Problems are what is wrong with each rule of a refused write or file, empty for any other
	// refusal.
	Problems []Problem
}

func (r *Refusal) Error() string { return string(r.Code) + ": " + r.Message }

// Of returns the refusal of code worded by message.
func Of(code Code, message string) *Refusal {
	return &Refusal{Code: code, Message: message}
}
