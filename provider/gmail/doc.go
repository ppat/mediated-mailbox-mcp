// Package gmail is the Gmail adapter. It implements the Provider Port over Gmail's REST API with the
// standard library's HTTP client, and it declares its rate profile.
//
// It also holds the account's installed-app OAuth grant through the installation's OAuth client
// (ADR-0083). That is the one-time consent, which requests only the modify scope, and the token
// source, built from the client and the refresh token the deployable opened from the database. The
// source holds a rotated refresh token and hands it to the deployable, which writes it back to the
// account's state row (ADR-0080, ADR-0082).
//
// # How the adapter is built and what proves it
//
// The adapter is three layers kept apart. Builders turn canonical inputs into one Gmail request
// each, a method, a path, query parameters and a JSON body. Parsers turn a Gmail response body into
// the canonical model or into one of the port's errors. The Adapter's methods are the shell that
// sends the requests and hands each response to its parser, and they hold no mapping of their own.
// The builders and parsers are tested on literal JSON shaped as Google's reference pages document
// it. What the shell does before a response arrives, holding a call to its cost and counting each
// request, is tested with requests a real HTTP client fails before they reach the network. What it
// does with Gmail's responses has no test here, because a stand-in for Google's API would be a mock
// (ADR-0043). Only the contract suite's run against real Gmail proves that part, and with it every
// assumption below marked as unverified.
//
// # What a call may cost
//
// Every port call is held to the worst case the rate profile declares for it (ADR-0023), which the
// rate limiter's lease covers. The call adds up what Gmail charges for each request it sends and
// refuses, without sending it, a request that would take it past that figure. Pages are sized from
// the same profile, one thread a page for ListThreads and three messages a page for EnumerateAll,
// and a caller asking GetMessageMetadata for more than three identifiers or Mutate for more than
// three ops is refused, so it splits its work into calls the rate limiter can issue.
//
// Every request is also counted where it is sent, at what Gmail charges for it, on
// mediated_mailbox_provider_request_cost_total with the account and provider="gmail", the series
// the runaway rule reads (ADR-0077). Beside the count the adapter sets
// mediated_mailbox_provider_hard_cap for the account, the hard-cap fraction of the ceiling the rate
// profile declares, which the rule compares the count against. A request is counted once it is
// built and before it leaves, so one that fails is counted as well, and one never sent because no
// access token was had is not. The count knows nothing of leases, so a request sent without one is
// counted like any other.
//
// # Operations
//
//   - ListThreads lists one thread through threads.list and reads it through threads.get, every
//     message through the metadata mask. GetThreadMetadata reads its thread the same way.
//   - EnumerateAll pages messages.list and reads each message through messages.get with the
//     metadata mask. GetMessageMetadata reads each identifier the same way.
//   - The metadata mask is a fields mask sent with the full format, naming no body field, so Google
//     returns the headers and the parts tree without any body content (ADR-0010). Every header of
//     the message comes back, since a mask cannot pick headers by name, and the model reads only
//     the ones it maps. Each part's type and file name come back to six levels of nesting.
//   - GetMessageBody reads the message in the full format and takes the first plain text part and
//     the first HTML part that are not attachments.
//   - Mutate sends one messages.modify per op. Only a label an op adds must exist (ADR-0010), so a
//     label it removes that the account lacks is left out of the modify. An op left with nothing to
//     change reads its message and sends nothing else, so an op on an unknown message is still not
//     found.
//     An op that the provider throttled, or whose credential it refused, fails every op after it
//     without a call.
//   - CurrentCursor reads the mailbox's history identifier from the profile, and ChangesSince reads
//     one page of history.list after it.
//   - Every listing sets includeSpamTrash, so the trash and spam are listed like any other label.
//
// # The canonical query
//
// A label is matched through the labelIds parameter with the identifier labels.list gives its
// path, never through the search text's label operator, which writes a slash, a hyphen and a space
// the same way. A label the account lacks selects nothing, and no listing call is made.
//
// A sender address and a date go into the search text as a superset of what the node selects,
// and every thread Gmail returns is kept only when one of its messages matches the query exactly
// on the mapped metadata. An After or Before instant is widened by a second on each side, since
// the search compares whole seconds and it is unverified whether its bounds are inclusive. It is
// also unverified whether the search compares the internal date or the Date header. The mapped
// date is the internal date, so a search comparing the header could leave out a message whose
// two dates differ. An address goes in quoted, and only when every character is one that cannot
// end the quote or start an operator. Any other address stays out of the search text, and the
// exact match on the parsed sender does all the selecting.
//
// A page token the adapter issues names the listing and carries Gmail's own page token, so a token
// from elsewhere is refused before any call. A cursor carries the history identifier the same way,
// and a cursor the adapter cannot read is a cursor gap.
//
// # The model
//
//   - Labels show user labels by their name and the system labels INBOX, TRASH, SPAM, SENT and
//     DRAFT by their identifier. IMPORTANT, CHAT, the CATEGORY_ labels and any other system label
//     are left out, because Gmail assigns them itself and they would show up as changes no one
//     made. That choice rests on the contract comparing labels exactly, and the run against real
//     Gmail may overturn it. A label change in the history counts as a modification only when it
//     touches a label or flag the model shows.
//   - UNREAD and STARRED map to the read and starred flags.
//   - The date is the internal date, the size is the size estimate, the list identifier is the
//     List-Id header without its angle brackets, and the snippet has its HTML character
//     references decoded.
//   - The authentication results are read from the first Authentication-Results header whose
//     authentication service is mx.google.com, so a header the sender wrote is not read.
//   - Attachment names are the file names of the parts the metadata mask returns. An attachment
//     nested deeper than the mask's six levels is missing from its metadata.
//   - A label identifier a message carries that labels.list no longer lists belongs to a label
//     deleted after the message was read, since the labels are listed after the messages, and it is
//     left out.
//
// # Errors
//
// A 404 is ErrNotFound, apart from history.list, where it is a cursor gap. A 400 whose message
// is Gmail's "Invalid id value" is ErrNotFound too, and any other 400 is ErrInvalid. An identifier
// holding a character outside Gmail's identifier alphabet names no message, and no call is made.
// A 429 and a 403 for exceeding the user's rate are per-user throttles, and a 403 for exceeding the
// project's daily limit is a per-project throttle, carrying any Retry-After delay. The scopes follow
// Google's error guide for the Gmail API, which says rateLimitExceeded means "the user has reached
// the maximum request rate for the Gmail API", userRateLimitExceeded "occurs when a request reaches
// the per-user limit", dailyLimitExceeded "occurs when your project reaches its API limit", and a
// 429 applies to "daily per-user limits, bandwidth limits, and concurrent request limits"
// (https://developers.google.com/workspace/gmail/api/guides/handle-errors). Any other 401 or 403
// is ErrAuthentication, and anything else is ErrProvider.
package gmail
