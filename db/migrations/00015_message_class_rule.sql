-- +goose Up
-- A message names the policy rule that set its sender class apart from the content rules that set
-- its flags (ADR-0016, ADR-0001). NULL is a class no rule set. Rows stored before this column hold
-- NULL, and nothing fills them in.
ALTER TABLE messages ADD COLUMN class_rule_id text;

-- Backfill's first pass writes the rule with the class it stores, and the delisting transition clears
-- it with the class it resets (ADR-0037). The UI reads it through its read of every column of the
-- table. The mediator's role gets nothing on it, as it holds nothing on a message's stored sender
-- class, which the column explains and which a record forbids it to act on (ADR-0002).
GRANT INSERT (class_rule_id), UPDATE (class_rule_id) ON messages TO mediated_mailbox_backfill;
