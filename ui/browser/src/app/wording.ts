// The on-screen wording of the stored vocabularies of docs/UI.md section 11. Each table is keyed by the
// closed value set the contract generates, so a value added to the registry fails the type check here
// until it is worded. A value the table does not know is still shown, as itself and marked unknown,
// never hidden (section 7.1).
import type { components } from "../generated/contract.ts";

type Schemas = components["schemas"];

export const planStatus = {
  DRAFT: "Awaiting approval",
  APPROVED: "Approved, waiting to apply",
  APPLYING: "Applying",
  APPLIED: "Applied",
  ROLLED_BACK: "Rolled back",
  REJECTED: "Rejected",
  APPLY_REFUSED: "Apply refused",
} as const satisfies Record<Schemas["PlanRow"]["status"], string>;

export const candidateStatus = {
  pending: "Awaiting review",
  confirmed: "Confirmed, not yet in effect",
  dismissed: "Dismissed",
} as const satisfies Record<Schemas["CandidateRow"]["status"], string>;

export const runState = {
  running: "running",
  succeeded: "succeeded",
  failed: "failed",
} as const satisfies Record<Schemas["Run"]["state"], string>;

// A workload's state is derived by the server, never stored (section 11).
export const workloadState = {
  not_started: "not started",
  running: "running",
  idle: "idle",
} as const satisfies Record<Schemas["BackfillBlock"]["state"], string>;

// The message row's vocabularies (docs/UI.md section 11). The contract carries a failure's message
// fields as open strings, so a value outside these tables arrives and is shown marked unknown.
export const senderClass: Readonly<Record<string, string>> = {
  normal: "normal",
  restricted: "restricted",
};

export const contentFlag: Readonly<Record<string, string>> = {
  mfa_code: "mfa",
  login_link: "link",
};

// scanState is the short form a cell shows, and scanStateLong the wording on hover and in a detail.
export const scanState: Readonly<Record<string, string>> = {
  scanned: "scanned",
  skipped_restricted: "restricted",
  skipped_gate: "gate skip",
  pending: "pending",
};

export const scanStateLong: Readonly<Record<string, string>> = {
  scanned: "scanned",
  skipped_restricted: "not scanned, restricted sender",
  skipped_gate: "released unscanned, gate skip",
  pending: "pending content scan",
};

export const errorClass: Readonly<Record<string, string>> = {
  throttled: "provider throttled",
  provider_error: "provider error",
  gone: "not found at provider",
  scanner_timeout: "scanner timeout",
  validation: "validation",
  authentication: "authentication",
};

export const disposition: Readonly<Record<string, string>> = {
  recovered: "recovered",
  pending: "pending",
  gone: "not found",
  abandoned: "abandoned",
};

export const auditAction: Readonly<Record<string, string>> = {
  READ_BODY: "body served",
  DENY_BODY: "body denied",
  MUTATE: "mutation applied",
  DENY_MUTATE: "mutation refused",
};

export const itemKind: Readonly<Record<string, string>> = {
  page: "page",
  message: "message",
  op: "operation",
};

// The policy history's actions (ADR-0102).
export const changeAction = {
  added: "added",
  edited: "edited",
  lifted: "lifted",
  confirmed: "confirmed from a candidate",
} as const satisfies Record<Schemas["ChangeRow"]["action"], string>;

// The problems a refused policy write or file names (docs/UI.md sections 8.7 and 17.4), each worded as
// what is wrong with the rule. The suffix an invalid suffix names is added where the problem is shown.
export const problemKind = {
  blank_identifier: "its identifier is empty or carries surrounding space",
  repeated_identifier: "another rule holds the same identifier",
  no_suffix: "it has no domain suffix",
  invalid_suffix: "a suffix is not shaped like a domain name",
  other_class: "its class is not restricted, the one class",
  reserved_identifier:
    "its identifier is a word the policy screens' addresses use, exactly . or .., or holds a /",
  not_the_form: "the file is not the policy file's form",
  too_large: "the file is over 1 MB",
} as const satisfies Record<Schemas["Problem"]["kind"], string>;

// Wording is a stored value's text, with known false for a value its table does not hold.
export type Wording = { text: string; known: boolean };

// word looks a stored value up in a vocabulary's table.
export function word(table: Readonly<Record<string, string>>, value: string): Wording {
  const text = Object.hasOwn(table, value) ? table[value] : undefined;
  return text === undefined ? { text: value, known: false } : { text, known: true };
}

// vocabularies are the tables a dataset's dimension is worded by, keyed by dataset and dimension.
const vocabularies: Readonly<
  Record<string, Readonly<Record<string, Readonly<Record<string, string>>>>>
> = {
  plans: { status: planStatus },
  candidates: { status: candidateStatus },
  runs: { state: runState },
  failures: { error_class: errorClass, disposition },
  policy_changes: { action: changeAction },
};

// dimensionWording words a value of a dataset's dimension, or returns the value as known text when the
// dimension has no vocabulary, since a free value such as a sender has no wording of its own.
export function dimensionWording(dataset: string, dimension: string, value: string): Wording {
  const table = vocabularies[dataset]?.[dimension];
  return table === undefined ? { text: value, known: true } : word(table, value);
}
