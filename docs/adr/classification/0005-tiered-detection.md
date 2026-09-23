# 0005. Secret detection is tiered and structural — and no LLM ever sits in the scanning path

**Status:** Accepted ·
**Pillar:** [Redaction is enforced by code, not by the provider's token](../../../DESIGN.md#redaction-is-enforced-by-code-not-by-the-providers-token) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

No list can enumerate MFA formats, so content-based detection must be heuristic. But full machine
learning is also the wrong first tool: one-time-code detection is a high-precision *pattern*
problem, and patterns handle the bulk at negligible cost.

## Decision

Detection is tiered, cost-ascending, each tier seeing only what the previous could not settle.
The scanner reads a body as the Markdown the sanitizing converter produces
([ADR-0036](../redaction/0036-released-bodies-are-clean-markdown.md)), which the shell passes in, so
the scanner calls nothing outside itself.

**Tier 1 — structural patterns (~85–90% of cases, microseconds).** One-time-code mail is
structurally distinctive, not merely lexically:

- A digit run of 4–8 within a token window of a trigger word (`code`, `OTP`, `verification`,
  `PIN`, `passcode`, `2FA`, `one-time`, `security code`, plus localized forms). A run may be split
  into groups of at least 3 digits by a space, a hyphen, or the no-break, narrow no-break and thin
  spaces mail uses to keep a code from wrapping.
- A short line whose entire content is a 4–8 digit run — extremely high precision; almost nothing
  else formats that way.
- A digit run in a first- or second-level heading, or as a table cell's whole content, the visual
  one-time-code idiom of HTML mail as it reads after conversion to Markdown. Letter-spacing styling
  does not survive the conversion.
- A URL with a high-entropy path segment plus `token`/`confirm`/`verify`/`reset`/`magic`/`auth`
  anywhere in its path or query, never in its host.
- A URL query parameter named `token`, `code`, `key`, `auth`, `t`, or `otp` carrying ≥16
  entropy-dense characters.

**Tier 2 — entropy and position scoring.** Candidate spans scored on Shannon entropy,
character-class mix, length, distance to the nearest trigger, and structural position (own line,
heading, bold), with the threshold tuned for recall. Catches alphanumeric and unusual formats Tier
1's fixed patterns miss.

**Tier 3 — a small local model**, in scope but deferred:
[ADR-0006](./0006-tier-3-local-model-deferred.md).

Implementation constraints that keep the tiers fast:

- **Set-matching, not sequential alternation** — the keyword layer is an Aho-Corasick automaton
  compiled once at startup; sequential regex alternation over dozens of patterns is the classic way
  this gets slow. The automaton is written in the scanner's own pure core and built once by the
  scanner's constructor.
- **Cost-ordered evaluation** — within detection, Tier 1 before Tier 2 before Tier 3, each stage
  eliminating most of what reaches it, so the expensive tiers should be rare; the full
  predicate-to-tier chain lives with the scan gate
  ([ADR-0007](../redaction/0007-composite-scan-gate.md)).
- **Scanning stays linear in a body's length**, so a large body cannot stall the batch path.

**The vocabulary and the tuning are configuration.** The trigger words by language, the link words
and query parameter names, the token window, the density values, and Tier 2's weights and
thresholds arrive as configuration the shell passes in. The words named in Tier 1 above are the
core of the defaults the application ships, which are English. A deployment adds languages and
retunes without a release. Each default rests on the evidence below, and every one of them is a
starting value, retuned against the real mailbox.

| Setting | Default | Why |
| --- | --- | --- |
| Trigger words | The Tier 1 words, plus `verify`, `password`, `two-factor` and its spellings, `one time`, `single-use`, `login`, `log in`, `log-in`, `sign-in`, `sign in`, `auth`, `authenticate`, `authentication`, `secret`, `access`, `validate`, `validation`, `TAN` and `confirmation` | [Android's notification one-time-code detector][android-otp], the one production detector with published source, matches these words, and the default mail templates of common authentication libraries use the rest, among them [Ory Kratos][kratos-templates], [Discourse][discourse-locales], [Supabase][supabase-templates] and [Amazon Cognito][cognito-templates]. [A study of 386,000 real SMS messages][reaves-sms] found such words overloaded, "password" often appearing where no code is, so `password`, `access` and `secret` trade precision for recall |
| Token window | 8 words | [Android's detector][android-otp] and [Amazon Macie][macie-proximity] both look 50 characters from the keyword, about 8 words of English. The templates put the trigger 0 to 5 words from the code, before or after it, as in [Ory Kratos's][kratos-login-subject] "Use code {code} to log in", [Discourse's][discourse-login-subject] "{code} is your login code", [Supabase's][supabase-templates] "{code} is your verification code" and [Amazon Cognito's][cognito-cdk-default] "The verification code to your new account is {code}" |
| Link words | The Tier 1 words, plus `password`, `login` and `unlock` | [Rails][rails-passwords-route] puts a reset token under `/passwords/`, [Discourse][discourse-email-login] a login token under `/session/email-login/`, and [Devise][devise-unlock-route] has an `/unlock` path, none holding a Tier 1 word |
| Parameter names | The Tier 1 names, plus `reset_password_token`, `confirmation_token`, `unlock_token`, `oobCode`, `verification_code`, `confirmation_code`, `ticket` and `signature` | The names [Devise][devise-mailer], [Firebase][firebase-oobcode], Auth0 ([`verification_code`][auth0-passwordless], [`ticket`][auth0-ticket]), [Amazon Cognito][cognito-templates] and [Laravel][laravel-signed-route] give the token in a login or verification link. A name matches whole and in any letter case |
| Density | At least 16 characters and 3.0 bits of Shannon entropy per character | [GitGuardian's generic high-entropy detector][gitguardian-generic] uses the same pair. The tokens the libraries generate for a link are 20 characters or longer and pass, as in [Devise][devise-token], [Django][django-tokens], [Laravel][laravel-reset] and [Auth.js][authjs-token]. The short numeric codes some of them carry in a link do not, a gap stated below. At this length the entropy test screens out repeated values and most all-digit values, not English words or slugs, so the link word or parameter name carries the precision |
| Tier 2 candidates | 4 to 10 letters and digits with at least one digit | [Android's detector][android-otp], [2FHey][twofhey-parser] and [Google's SMS consent interface][google-sms-consent] each take at least 4 characters with a digit among them and at most 10 |
| Tier 2 weights | Equal across the five features | No published detector combines these features by weight. [Presidio][presidio-context] adds a fixed boost for nearby context, [Google's Sensitive Data Protection][google-dlp-likelihood] moves a finding along fixed likelihood levels, and the one-time-code detectors apply rules that pass or fail. On the evaluation set equal weights separate every code from every reference |
| Tier 2 thresholds | 0.6 for a body and 0.6 for a subject | Set against the evaluation set, written as the default templates of the libraries above write codes and links, beside order, flight and product references in bodies and subjects. Every code there that only Tier 2 can catch scores at least 0.73, having a trigger word near it or standing alone or in bold, and the lowest such code in the scanner's other tests, nine digits after a trigger, scores 0.65. A reference scores at most 0.53. A subject threshold at or below 0.53 would mask alphanumeric order and flight numbers in a subject, and one at or below 0.45 numeric order numbers too, while catching no code the set holds, because every template that puts a code in a subject puts a trigger word beside it, as in [Supabase's][supabase-templates], [Discourse's][discourse-locales] and [Ory Kratos's][kratos-templates] |

[android-otp]: https://android.googlesource.com/platform/packages/modules/ExtServices/+/refs/heads/main/java/src/android/ext/services/notification/NotificationOtpDetectionHelper.java
[auth0-passwordless]: https://github.com/auth0/auth0-react/issues/102
[auth0-ticket]: https://community.auth0.com/t/brand-style-the-email-verification-screen/78148
[authjs-token]: https://github.com/nextauthjs/next-auth/blob/main/packages/core/src/lib/actions/signin/send-token.ts
[cognito-cdk-default]: https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.aws_cognito.UserVerificationConfig.html
[cognito-templates]: https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-user-pool-settings-message-customizations.html
[devise-mailer]: https://github.com/heartcombo/devise/tree/main/app/views/devise/mailer
[devise-token]: https://github.com/heartcombo/devise/blob/main/lib/devise.rb
[devise-unlock-route]: https://github.com/heartcombo/devise/blob/46c2c3913eac6acbb13c9916f011595d0d82691e/lib/devise/rails/routes.rb#L394-L397
[discourse-email-login]: https://github.com/discourse/discourse/blob/c8e3857107f8fa5317558b281cc8eb229083ed12/config/locales/server.en.yml#L5159
[discourse-locales]: https://github.com/discourse/discourse/blob/main/config/locales/server.en.yml
[discourse-login-subject]: https://github.com/discourse/discourse/blob/c8e3857107f8fa5317558b281cc8eb229083ed12/config/locales/server.en.yml#L3714
[django-tokens]: https://github.com/django/django/blob/main/django/contrib/auth/tokens.py
[firebase-oobcode]: https://firebase.google.com/docs/auth/custom-email-handler
[gitguardian-generic]: https://docs.gitguardian.com/secrets-detection/secrets-detection-engine/detectors/generics/generic_high_entropy_secret
[google-dlp-likelihood]: https://docs.cloud.google.com/sensitive-data-protection/docs/creating-custom-infotypes-likelihood
[google-sms-consent]: https://developers.google.com/identity/sms-retriever/user-consent/request
[kratos-login-subject]: https://github.com/ory/kratos/blob/f4a876dc1204f4a2edd29af09d45214260b510ab/courier/template/courier/builtin/templates/login_code/valid/email.subject.gotmpl#L1
[kratos-templates]: https://github.com/ory/kratos/tree/master/courier/template/courier/builtin/templates
[laravel-reset]: https://github.com/laravel/framework/blob/12.x/src/Illuminate/Auth/Passwords/DatabaseTokenRepository.php
[laravel-signed-route]: https://github.com/laravel/framework/blob/3b5d2d2865ed801ea0b8c49377e72619b34c88a4/src/Illuminate/Routing/UrlGenerator.php#L366-L386
[macie-proximity]: https://docs.aws.amazon.com/macie/latest/user/cdis-options.html
[presidio-context]: https://github.com/microsoft/presidio/blob/e9895a51f5af6a00ed859a1ee38d627868355f8a/presidio-analyzer/presidio_analyzer/context_aware_enhancers/lemma_context_aware_enhancer.py
[rails-passwords-route]: https://github.com/rails/rails/blob/567e6802f53683a411c9044526437e5a59f1aba7/railties/lib/rails/generators/rails/authentication/authentication_generator.rb#L43
[reaves-sms]: https://www.cise.ufl.edu/~butler/pubs/oakland16.pdf
[supabase-templates]: https://supabase.com/docs/guides/auth/auth-email-templates
[twofhey-parser]: https://github.com/SoFriendly/2fhey/blob/76a3c02df52ea98bba5263233ec337823310df07/TwoFHey/OTPParser/OTPParser.swift

**Explicitly not: an LLM in the scanning path.** Slow, expensive, non-deterministic, and —
decisively — it means sending body content to an inference endpoint, which is the exact exposure
this system exists to prevent. LLM help writing *rules* is fine: done offline on a curated sample,
shipping the rules, never the model.

## Alternatives considered

- **A single ML classifier for all content detection.** Rejected: it spends the hard cases' tool
  on the easy cases, adds nondeterminism where patterns are near-perfect, and requires labeled data
  that does not exist on day one.
- **Patterns only, no scoring tier.** Rejected: fixed patterns miss alphanumeric and novel formats;
  the scoring tier is what covers the tail without a model.
- **An LLM scanner (local or hosted).** Rejected as above; the hosted variant is
  self-contradictory for this system, and even a local LLM is slow and non-deterministic where the
  tiers are fast and inspectable.
- **Scanning the HTML, parsed by the shell or tokenized in the core.** The case for the first is
  that an HTML parser in the shell hands the core a plain document model and keeps the
  letter-spacing idiom visible. The case for the second is that it needs no shell step. A tokenizer
  written for the scanner would be a second HTML parser to maintain. Rejected by the operator on
  2026-09-23 in favour of the Markdown the converter already produces.
- **An Aho-Corasick library.** The case for it is that the automaton is already written. One
  candidate returns which words matched and not where, which the token window needs, and the other
  is some three thousand lines of outside code inside the safeguard core, to be checked against the
  conditions for a pure core's imports
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)) on every version. Rejected by
  the operator on 2026-09-23.
- **The vocabulary and tuning as code.** The case for it is that every improvement is a reviewed
  change with a version bump. Rejected by the operator on 2026-09-23. Configuration lets a
  deployment add languages and retune without a release, and a configuration revision stamped on
  every verdict keeps re-scanning tractable.

## Consequences

- Detection quality is inspectable per tier: every hit records which tier and rule fired, so
  precision problems localize to a rule rather than a model.
- The tier boundary gives Tier 3 a natural insertion point later without touching Tiers 1–2.
- Patterns are code, so improving them is a reviewed change with a version bump. The vocabulary
  and the tuning are configuration, whose revision is stamped on every verdict beside the version.
  A change to either marks the verdicts made before it stale, which is what makes re-scanning after
  improvements tractable ([ADR-0009](../redaction/0009-scanner-verdicts-carry-no-content.md)).
- A trigger word and a digit run are found only where a word boundary separates them, so in a
  language written without spaces between words neither is found, subject masking included, and
  that language needs matching of another kind. Digits outside ASCII, full-width digits among them,
  are not read as digits, and case folding covers ASCII letters only.
- A long slug holding a link word, such as `/how-to-reset-your-sleep-schedule`, is as dense as a
  token and is flagged as a login link. Two gaps are known. A short numeric code carried in a link,
  such as [Cognito's][cognito-templates] `confirmation_code=123456`, is too short for the density
  test. A link rewritten by a mail provider's click tracking, such as
  [SendGrid's](https://support.sendgrid.com/hc/en-us/articles/44375837088795-How-to-Know-if-my-Links-Are-Shortened-by-SendGrid),
  hides both the parameter name and the link word.
- Assumptions about other components. The shell converts a body to Markdown with the sanitizing
  converter before scanning it, so every path that scans runs the converter too. The shell that
  first runs the scanner decides how configuration is loaded.
