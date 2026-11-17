// Package provider is the calling side of the provider library, published as
// mediated-mailbox-provider, a narrow, named exception to the rule that shared code is pure. Its case
// is argued in provider/README.md.
//
// It holds the list of shipped adapters, the token an account's calls authenticate with, the facts
// about each provider, the shared provider series, and the caller, one job kind's or one operation's
// own equipment for calling a provider. A caller returns, for one account's unit of work, a mailbox
// whose every operation takes a lease sized by the operation's cost at the account's target, waits
// for it no later than the unit's deadline, runs under the caller's timeout, tells the rate limiter
// how it went, and tries once more over a credential read again when the provider refuses the one it
// had (ADR-0010, ADR-0023, ADR-0025, ADR-0090, ADR-0106, ADR-0117). One type does this for all ten
// operations of the Provider Port.
//
// The adapters are its subpackages under mail and calendar, Google's grant is google, each
// provider's consent by name is consent, and the rate limiter is ratelimit.
package provider
