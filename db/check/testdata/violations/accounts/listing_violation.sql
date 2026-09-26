-- Statements in a directory named accounts, the subsection that holds the accounts listing. The
-- listing is exempt from the account predicate there, and a write beside it is not, so an exception
-- keyed by the subsection alone is refused.

-- name: Accounts :many
SELECT account_id FROM accounts;

-- want account-predicate
-- name: RenameEveryProvider :execrows
UPDATE accounts SET provider = @provider;
