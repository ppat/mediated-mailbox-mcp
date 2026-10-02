# 0107. Gmail connects through an installed-app OAuth client in the installation owner's own Cloud project, set up through a guided flow in the UI, with `gmail.modify`

**Status:** Accepted (supersedes [ADR-0083](./0083-gmail-through-an-installation-oauth-client.md),
jointly with [ADR-0106](./0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)) ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [A2](../../../USE_CASES.md#a2--no-destructive-action-on-sensitive-mail), [P3](../../../USE_CASES.md#p3--multi-account), [O6](../../../USE_CASES.md#o6--deployable)

## Context

The deployables that call Gmail need durable, headless access to a mailbox with label-mutation
rights, and accounts may span organizations with no shared administrator. An account is connected
and re-authorized through a guided flow in the UI
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)). Every Gmail API grant
goes through an OAuth client, and every OAuth client belongs to a Google Cloud project. Setting one
up by hand is the friction that loses users along the way, so the setup has to be as painless as
the provider allows.

Two facts from Google bound the choice. `gmail.modify` is a restricted scope. An app used only by
its developer, or by a few people the developer knows personally, may stay unverified, and anything
wider needs Google's restricted-scope verification and a yearly security assessment
([restricted scope verification](https://developers.google.com/identity/protocols/oauth2/production-readiness/restricted-scope-verification)).
And no public API configures a consent screen for an external audience or creates an installed-app
or web OAuth client. Only creating the project and enabling the Gmail API can be scripted
([gcloud projects create](https://docs.cloud.google.com/sdk/gcloud/reference/projects/create),
[gcloud services enable](https://docs.cloud.google.com/sdk/gcloud/reference/services/enable)).

## Decision

| Route | Verdict |
| --- | --- |
| **Installed-app OAuth, offline refresh token** | **Chosen.** Consent once per account, revocable per account |
| Service account with domain-wide delegation | **Rejected.** A Workspace-wide skeleton key, and incoherent with accounts spanning organizations |
| Service account without delegation | Non-viable. It cannot access user mailboxes at all |

- **Each Gmail client is an installed-app OAuth client in a Cloud project of the person running
  the installation.** A client serves only its owner's own installation, so it stays inside
  Google's personal-use exception and needs no verification. Each account's grant is its own
  refresh token, revocable alone. How many clients an installation holds, and which accounts
  connect through each, is
  [ADR-0106](./0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)'s.
- **Setting up the client is its own step, apart from connecting an account.** The UI guides it
  once per client, entirely in Google Cloud console. It links straight to each console page
  it needs, gives each step's instructions in the UI, and states the exact value to enter. It
  offers no command-line route, because the two steps a script can do are trivial in the console
  the person must open anyway for the rest. The person brings the client's identifier and secret
  into the UI, which checks them against Google at once and stores the secret sealed
  ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)).
- **The client's project is published "In production", never left in "Testing".** A project in
  "Testing" issues refresh tokens that expire after 7 days
  ([Google's OAuth 2.0 overview](https://developers.google.com/identity/protocols/oauth2)), so the
  saved token itself stops working and every account would need consent again each week. In
  production and unverified, the person clicks through Google's unverified-app warning once per
  consent ([unverified apps](https://support.google.com/cloud/answer/7454865)).
- **Connecting an account runs the consent from the UI.** The UI sends the person to Google's
  consent page with a loopback redirect to `127.0.0.1`, where nothing listens. The browser lands on
  a page that fails to load, and the person pastes that page's address, which carries the
  authorization code, back into the UI. The UI exchanges the code with PKCE, checks the state it
  issued, and refuses a grant for a mailbox other than the one the person named. Loopback redirects
  remain Google's recommended method for desktop clients
  ([OAuth 2.0 for installed apps](https://developers.google.com/identity/protocols/oauth2/native-app)).
- **On re-authorization, the mailbox named is the one the account remembers.** The consent's
  mailbox check compares the grant with the mailbox the account remembers
  ([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)).
- **`gmail.modify` is the scope.** Label mutation is the point, and `gmail.readonly` with
  `gmail.labels` does not permit applying labels to messages. `gmail.settings.*` and
  `https://mail.google.com/` are never requested. The latter grants IMAP access and permanent
  delete. The absence of permanent delete from the token is one of the two structural halves of
  [ADR-0019](../mutation/0019-asymmetric-mutation.md)'s "never permanently delete".

## Alternatives considered

- **One client the project ships with the system.** For it, nobody running the system creates a
  Cloud project. Against it, an open-source project's client serves people its developer does not
  know, which puts it outside the personal-use exception and into restricted-scope verification and
  a yearly paid security assessment, which an open-source project cannot carry.
- **A script that creates the whole setup with the Cloud CLI**, as some projects advertise. For
  it, the person types one command. Against it, only the project and the Gmail API can be created
  from a script. The consent screen and the OAuth client have no public API, the Identity-Aware
  Proxy commands once used for clients were shut down
  ([IAP deprecations](https://docs.cloud.google.com/iap/docs/deprecations)) and never made a client
  a personal account could use, and GAM's own project command prints a console walkthrough for the
  client and waits for its identifier and secret to be pasted back
  ([GAM](https://github.com/GAM-team/GAM)).
- **Offering the scriptable part as commands beside the guided flow.** For it, a person at ease in
  a terminal creates the project and enables the Gmail API without the console. Against it, the
  operator ruled on 2026-10-01 that installing, configuring and authenticating the Cloud CLI is
  busy work for the two most trivial steps, done more easily in the console the person must be in
  anyway for the steps that matter.
- **A web-application client redirecting to the UI.** For it, no address is pasted. Against it,
  a web client's redirect must be an HTTPS address registered on the client, and a bare IP address
  is refused
  ([OAuth 2.0 for web server apps](https://developers.google.com/identity/protocols/oauth2/web-server)),
  and nothing about a deployment guarantees the UI such an address
  ([ADR-0051](../engineering/0051-environment-contract.md)).
- **IMAP with an app password.** For it, no Cloud project exists anywhere. Rejected by the
  operator. An app password grants the whole mailbox including permanent delete, which removes the
  token's half of [ADR-0019](../mutation/0019-asymmetric-mutation.md)'s guarantee.
- **The device authorization flow.** Non-viable. Its fixed list of allowed scopes holds no Gmail
  scope ([limited-input devices](https://developers.google.com/identity/protocols/oauth2/limited-input-device)).
- **Broader scopes "to be safe".** Rejected, because every scope the token lacks is a capability
  no compromise of a provider-calling deployable can exercise.

## Consequences

- Each account is an independent grant. Revoking one revokes one, and no organization-level
  administration is assumed anywhere. Accounts that connect through one client share that
  client, never a grant.
- The token can never permanently delete mail, regardless of any code bug. The client surface's
  half of that guarantee is [ADR-0019](../mutation/0019-asymmetric-mutation.md).
- Google discontinued its manual copy-and-paste redirect
  ([OAuth 2.0 for installed apps](https://developers.google.com/identity/protocols/oauth2/native-app)).
  Pasting a loopback address back is not that method, but it rests on Google continuing to allow
  a loopback redirect nobody listens on.
- The consent command stays as a developer tool that mints the refresh token the contract suite's
  run against the test account uses. It shares the consent code the UI runs and ships in no image.
- Refresh-token durability stays a load-bearing operational concern. Its handling is
  [ADR-0082](../operability/0082-rotation-writeback-to-the-database.md), and a lost grant is
  repaired through the UI.
