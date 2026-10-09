-- +goose Up
-- An account identifier is one DNS label, lowercase letters, digits and hyphens, at most 63
-- characters, starting and ending with a letter or a digit. Every identifier the grammar accepts is a
-- path segment the mediator's API root can address (ADR-0087), so the grammar replaces the check that
-- refused only ., .. and /. An account stored outside the grammar stops this migration, and nothing
-- rewrites it, since an account is system-of-record data (ADR-0048).
ALTER TABLE accounts
DROP CONSTRAINT accounts_account_id_addressable,
ADD CONSTRAINT accounts_account_id_grammar CHECK (account_id ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$');
