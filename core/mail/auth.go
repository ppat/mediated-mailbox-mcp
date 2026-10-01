package mail

// AuthOutcome is how a provider authentication attempt ended, one of a closed set of three whose
// values are the spellings account_state.last_auth_outcome stores (ADR-0097, ADR-0016).
type AuthOutcome string

const (
	// AuthSucceeded is a credential the provider accepted.
	AuthSucceeded AuthOutcome = "succeeded"
	// AuthRefused is an answer from the provider refusing the request. It needs the operator to
	// act, re-authorizing the account or, for a refused client, replacing the installation's OAuth
	// client.
	AuthRefused AuthOutcome = "refused"
	// AuthFailed is an attempt that got no answer the adapter could read as either, a passed
	// deadline included, which is transient and needs no operator action.
	AuthFailed AuthOutcome = "failed"
)

// AuthAttempt is an adapter's latest request to its provider's authentication endpoint, the instant
// it started and its outcome. The zero value is no attempt. The adapter reports it and writes
// nothing, and the deployable records it at the end of each unit of work (ADR-0097).
type AuthAttempt struct {
	At      UnixMilli
	Outcome AuthOutcome
}
