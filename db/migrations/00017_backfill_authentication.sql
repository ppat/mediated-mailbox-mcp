-- +goose Up
-- Backfill records the authentication attempt its adapter reports at the end of each unit of work,
-- only over an older one (ADR-0097). The statement sits in db/accountstate/authentication, which
-- only backfill's list admits. It reads last_auth_at in its predicate, which backfill already
-- reads through the account's progress.
GRANT UPDATE (last_auth_at, last_auth_outcome) ON account_state TO mediated_mailbox_backfill;
