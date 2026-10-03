-- +goose Up
-- The UI's policy management (ADR-0084, ADR-0102, ADR-0110, ADR-0112).
--
-- A rule's identifier is unique within its scope, the base policy or one account's own rules, so the
-- base policy and an account, or two accounts, may each hold a rule of one identifier (ADR-0110). The
-- base policy is the scope whose account is null, and NULLS NOT DISTINCT makes it one scope. The
-- policy snapshot's validation in core/policy refuses a repeated identifier within one scope only,
-- and lands with this key.
ALTER TABLE policy_rules DROP CONSTRAINT policy_rules_pkey;
ALTER TABLE policy_rules ALTER COLUMN rule_id SET NOT NULL;
ALTER TABLE policy_rules ADD CONSTRAINT policy_rules_scope_rule_id_key UNIQUE NULLS NOT DISTINCT (account_id, rule_id);

-- Every change to policy_rules, appended in the same transaction (ADR-0102). A base rule's change has
-- a null account. No runtime role holds UPDATE or DELETE here, as on audit_log.
CREATE TABLE policy_changes (
    id bigserial PRIMARY KEY,
    -- NULL for a base rule's change. An overlay rule's names its account.
    account_id text REFERENCES accounts,
    ts timestamptz NOT NULL DEFAULT now(),
    -- The operator identity (ADR-0084).
    actor text NOT NULL,
    -- added | edited | lifted | confirmed
    action text NOT NULL,
    rule_id text NOT NULL,
    -- Empty for added and confirmed.
    suffixes_before text[] NOT NULL DEFAULT '{}',
    -- Empty for lifted.
    suffixes_after text[] NOT NULL DEFAULT '{}'
);
CREATE INDEX ON policy_changes (account_id, ts DESC);

-- Row-level security on the history mirrors the rules. An account reads and writes its own rows, and
-- every account reads the base policy's.
ALTER TABLE policy_changes ENABLE ROW LEVEL SECURITY;
CREATE POLICY policy_changes_account ON policy_changes
USING (account_id = current_setting('app.account'));
CREATE POLICY policy_changes_base ON policy_changes FOR SELECT
USING (account_id IS NULL);

-- The base policy is written only by the UI's role, and only in a base-policy transaction, which sets
-- app.base to on and the account to the empty string (ADR-0112). The transaction helper's second entry
-- point sets and reads back both. An account's transaction sets an account, so it writes no base row,
-- and a base-policy transaction's empty account matches no account's row, so it reads and writes none.
-- Neither setting is known on a connection that never set it, so both are read with missing_ok.
CREATE POLICY policy_rules_base_write ON policy_rules
TO mediated_mailbox_ui
USING (
    account_id IS NULL
    AND current_setting('app.base', TRUE) = 'on'
    AND coalesce(current_setting('app.account', TRUE), '') = ''
)
WITH CHECK (
    account_id IS NULL
    AND current_setting('app.base', TRUE) = 'on'
    AND coalesce(current_setting('app.account', TRUE), '') = ''
);
CREATE POLICY policy_changes_base_write ON policy_changes FOR INSERT
TO mediated_mailbox_ui
WITH CHECK (
    account_id IS NULL
    AND current_setting('app.base', TRUE) = 'on'
    AND coalesce(current_setting('app.account', TRUE), '') = ''
);

-- Policy management adds, edits and lifts rules, and an import does all three (ADR-0084, ADR-0110).
-- An edit changes a rule's domain suffixes alone, since its scope and identifier never change. Its
-- statements sit in db/policyrules/manage for an account's rules and db/policyrules/base for the base
-- policy's, which only the UI's list admits. The insert on policy_rules was granted with the schema.
GRANT UPDATE (domain_suffix), DELETE ON policy_rules TO mediated_mailbox_ui;

-- Each policy write appends its history row, and the policy screens read the history. Insert alone,
-- so evidence of who lifted a restriction survives the UI's compromise (ADR-0102).
GRANT SELECT, INSERT ON policy_changes TO mediated_mailbox_ui;
GRANT USAGE ON SEQUENCE policy_changes_id_seq TO mediated_mailbox_ui;
