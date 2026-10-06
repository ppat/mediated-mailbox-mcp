// Import a scope's rules from a file (docs/UI.md section 8.7, ADR-0110). The operator drops the file or
// chooses it, and the server reads and checks it before anything is shown. A refused file names each
// problem with its rule and line. A file it takes is previewed as the difference between the scope's
// stored rules and the file, lifts and removed suffixes first, and imported with one confirmation of
// every restriction it lifts: none when it lifts nothing, one dialog for an account's scope, and the
// typed lift {k} for the base policy, whose lifts every account loses. The request always carries
// lift {k} when it lifts, so the server applies only an import whose confirmation covers its count.
import type { ComponentChildren } from "preact";
import { useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import { policyApi, send, type Failure, type ImportPreview, type Imported } from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { counted } from "../app/format.ts";
import { outcomeKey, type Outcome, type PutBack, type Scope } from "../app/outcomes.ts";
import {
  countLine,
  LiftDialog,
  policyHome,
  refreshPolicy,
  Refusal,
  releaseSentences,
} from "./policywrites.tsx";

// confirmation is the typed confirmation of an import that lifts k restrictions.
export function confirmation(k: number): string {
  return `lift ${k}`;
}

// importedOutcome is what the scope's policy screen reads after an import, each count taking the
// singular for one, with Put back when anything was lifted (section 8.7).
export function importedOutcome(scope: Scope, done: Imported): Outcome {
  const putBack: PutBack[] = [
    ...done.lifted.map((r) => ({ scope, rule: r.rule_id, suffixes: r.suffixes, onto: undefined })),
    ...done.edited
      .filter((e) => e.removed.length > 0)
      .map((e) => ({
        scope,
        rule: e.rule_id,
        suffixes: e.removed.map((r) => r.suffix),
        onto: e.after,
      })),
  ];
  return {
    text: `Imported: ${counted(done.added.length, "rule")} added, ${counted(done.edited.length, "rule")} edited, ${counted(done.lifts, "restriction")} lifted.`,
    putBack,
    putBackLabel: `Put back the ${counted(done.lifts, "lifted restriction")}`,
  };
}

// Preview is a file read and checked, or refused.
type Read = { file: string; preview: ImportPreview } | { file: string; failure: Failure };

export function ImportScreen(props: { account: string | undefined }) {
  const deps = useDeps();
  const { route } = useLocation();
  const { account } = props;
  const scope: Scope = account === undefined ? "base" : "account";
  const [read, setRead] = useState<Read | undefined>();
  const [dialog, setDialog] = useState(false);
  const [failure, setFailure] = useState<Failure | undefined>();
  const take = async (file: File | undefined) => {
    if (file === undefined) {
      return;
    }
    setFailure(undefined);
    setDialog(false);
    const text = await file.text();
    const result = await send<ImportPreview>(deps.post, `${policyApi(account)}/import/preview`, {
      file: text,
    });
    setRead(
      result.ok ? { file: text, preview: result.value } : { file: text, failure: result.failure },
    );
  };
  const apply = async (file: string, preview: ImportPreview) => {
    setFailure(undefined);
    const result = await send<Imported>(deps.post, `${policyApi(account)}/import`, {
      file,
      computed_against: preview.computed_against,
      confirmation: preview.lifts > 0 ? confirmation(preview.lifts) : null,
    });
    if (!result.ok) {
      setFailure(result.failure);
      return;
    }
    deps.outcomes.set(outcomeKey(account), importedOutcome(scope, result.value));
    refreshPolicy(deps);
    route(policyHome(account));
  };
  return (
    <section aria-labelledby="import-title" class="policy-import">
      <h1 id="import-title">
        {account === undefined ? "Import the base policy" : `Import ${account}'s rules`}
      </h1>
      <p>
        {account === undefined
          ? "The file replaces the base rules, which every account inherits."
          : `The file replaces ${account}'s own rules. The base rules are not touched.`}
      </p>
      <div
        class="drop-zone"
        onDragOver={(event) => event.preventDefault()}
        onDrop={(event) => {
          event.preventDefault();
          void take(event.dataTransfer?.files[0]);
        }}
      >
        <p>Drop a policy file here, or</p>
        <label>
          Choose a policy file{" "}
          <input
            type="file"
            accept=".yaml,.yml,application/yaml,text/yaml"
            onChange={(event) => void take(event.currentTarget.files?.[0])}
          />
        </label>
      </div>
      {read === undefined ? null : "failure" in read ? (
        <Refusal failure={read.failure} rule="" scopeName={account} />
      ) : (
        <PreviewOf
          account={account}
          preview={read.preview}
          dialog={dialog}
          failure={failure}
          onImport={() =>
            read.preview.lifts > 0 ? setDialog(true) : void apply(read.file, read.preview)
          }
          onConfirm={() => void apply(read.file, read.preview)}
          onCancel={() => setDialog(false)}
        />
      )}
      <p>
        <a href={policyHome(account)}>Back to the policy</a>
      </p>
    </section>
  );
}

// Group is one group of the preview, a heading with its count over its items.
function Group(props: { title: string; n: number; lifts?: boolean; children: ComponentChildren }) {
  if (props.n === 0) {
    return null;
  }
  return (
    <section class={props.lifts === true ? "preview-group lifts" : "preview-group"}>
      <h3>
        {props.title} ({props.n})
      </h3>
      <ul>{props.children}</ul>
    </section>
  );
}

// Releases is what one lift releases in the account, as the lift dialog states it, or nothing for the
// base policy, whose preview carries no account's numbers.
function Releases(props: {
  account: string | undefined;
  released: { senders: number; messages: number } | null;
  what: string;
}) {
  if (props.account === undefined || props.released === null) {
    return null;
  }
  return (
    <p class="muted">
      In {props.account}, {countLine(props.released.senders, props.released.messages)} are
      restricted by this {props.what} and by no other rule.
    </p>
  );
}

function PreviewOf(props: {
  account: string | undefined;
  preview: ImportPreview;
  dialog: boolean;
  failure: Failure | undefined;
  onImport: () => void;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const { account, preview } = props;
  const removed = preview.edited.flatMap((e) => e.removed.map((r) => ({ rule: e.rule_id, ...r })));
  const added = preview.edited.flatMap((e) =>
    e.added.map((suffix) => ({ rule: e.rule_id, suffix })),
  );
  const changes = preview.added.length + preview.edited.length;
  if (changes === 0 && preview.lifted.length === 0) {
    return <p>The file matches the stored rules. Nothing to import.</p>;
  }
  const k = preview.lifts;
  const refusal =
    props.failure === undefined ? undefined : (
      <Refusal failure={props.failure} rule="" scopeName={account} />
    );
  return (
    <section class="preview" aria-label="What the import changes">
      <Group title="Rules lifted" n={preview.lifted.length} lifts>
        {preview.lifted.map((r) => (
          <li key={r.rule_id}>
            <span class="mono">{r.rule_id}</span>, {r.suffixes.join(", ")}
            <Releases account={account} released={r.released} what="rule" />
          </li>
        ))}
      </Group>
      <Group title="Suffixes removed from a rule" n={removed.length} lifts>
        {removed.map((r) => (
          <li key={`${r.rule}/${r.suffix}`}>
            <span class="mono">{r.suffix}</span> from <span class="mono">{r.rule}</span>
            <Releases account={account} released={r.released} what="suffix" />
          </li>
        ))}
      </Group>
      <Group title="Rules added" n={preview.added.length}>
        {preview.added.map((r) => (
          <li key={r.rule_id}>
            <span class="mono">{r.rule_id}</span>, {r.suffixes.join(", ")}
          </li>
        ))}
      </Group>
      <Group title="Suffixes added to a rule" n={added.length}>
        {added.map((r) => (
          <li key={`${r.rule}/${r.suffix}`}>
            <span class="mono">{r.suffix}</span> to <span class="mono">{r.rule}</span>
          </li>
        ))}
      </Group>
      <p class="muted">Unchanged rules: {preview.unchanged}</p>
      <button type="button" class={k > 0 ? "lift" : "primary"} onClick={props.onImport}>
        {k > 0
          ? `Import, lifting ${counted(k, "restriction")}`
          : `Import ${counted(changes, "change")}`}
      </button>
      {props.dialog ? null : refusal}
      {props.dialog ? (
        <LiftDialog
          title={`This import lifts ${counted(k, "restriction")}`}
          typed={
            account === undefined
              ? {
                  expected: confirmation(k),
                  label: `Type ${confirmation(k)} to confirm`,
                  until: `Type ${confirmation(k)} to enable`,
                }
              : undefined
          }
          action={`Import and lift ${k}`}
          refusal={refusal}
          onConfirm={props.onConfirm}
          onCancel={props.onCancel}
        >
          <ul>
            {preview.lifted.map((r) => (
              <li key={r.rule_id}>
                <span class="mono">{r.rule_id}</span>
              </li>
            ))}
            {removed.map((r) => (
              <li key={`${r.rule}/${r.suffix}`}>
                <span class="mono">{r.suffix}</span> from <span class="mono">{r.rule}</span>
              </li>
            ))}
          </ul>
          {account === undefined ? (
            <p>
              {preview.accounts.length === 0
                ? "No account is connected, so nothing is released now."
                : `Every account loses these restrictions: ${preview.accounts.join(", ")}.`}
            </p>
          ) : (
            <p>
              In {account},{" "}
              {countLine(preview.released?.senders ?? 0, preview.released?.messages ?? 0)} are
              restricted by what this import lifts and by no other rule.
            </p>
          )}
          <p>{releaseSentences}</p>
        </LiftDialog>
      ) : null}
    </section>
  );
}
