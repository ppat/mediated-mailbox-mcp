-- name: RecordChange :exec
-- Appends one change to one of the account's own rules to the policy history, in the transaction that
-- made it (ADR-0102).
INSERT INTO policy_changes (account_id, actor, action, rule_id, suffixes_before, suffixes_after)
VALUES (@account_id, @actor, @action, @rule_id, @suffixes_before, @suffixes_after);

-- name: ChangeFigures :one
-- The policy_changes dataset's figures, the changes the account's history holds under the dataset's
-- range and filters, those that added restriction and those that lifted it (docs/UI.md section 17.1).
-- An edit lifts when it removed a suffix and adds when it removed none. The account's history is its
-- own changes and the base policy's (ADR-0102). base_rows and own_rows say which scopes the scope
-- filter keeps. A null array is no filter.
SELECT
    count(*) AS changes,
    count(*) FILTER (
        WHERE c.action IN ('added', 'confirmed') OR (c.action = 'edited' AND c.suffixes_before <@ c.suffixes_after)
    ) AS added,
    count(*) FILTER (
        WHERE c.action = 'lifted' OR (c.action = 'edited' AND NOT c.suffixes_before <@ c.suffixes_after)
    ) AS lifted
FROM policy_changes AS c
WHERE
    (c.account_id = @account_id::text OR c.account_id IS NULL)
    AND (@base_rows::boolean OR c.account_id IS NOT NULL)
    AND (@own_rows::boolean OR c.account_id IS NULL)
    AND (@range_start::timestamptz IS NULL OR c.ts >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.ts < @range_end::timestamptz)
    AND (@action_in::text[] IS NULL OR c.action = any(@action_in::text[]))
    AND (@action_out::text[] IS NULL OR c.action != all(@action_out::text[]))
    AND (@actor_in::text[] IS NULL OR c.actor = any(@actor_in::text[]))
    AND (@actor_out::text[] IS NULL OR c.actor != all(@actor_out::text[]))
    AND (@rule_in::text[] IS NULL OR c.rule_id = any(@rule_in::text[]))
    AND (@rule_out::text[] IS NULL OR c.rule_id != all(@rule_out::text[]))
    AND (@day_in::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date != all(@day_out::date[]));

-- name: ChangeRows :many
-- One page of the policy_changes dataset, fifty rows, under the same filters, newest first by default,
-- the change's identifier ending the sort.
SELECT
    c.id,
    c.ts,
    c.account_id,
    c.actor,
    c.action,
    c.rule_id,
    c.suffixes_before,
    c.suffixes_after
FROM policy_changes AS c
WHERE
    (c.account_id = @account_id::text OR c.account_id IS NULL)
    AND (@base_rows::boolean OR c.account_id IS NOT NULL)
    AND (@own_rows::boolean OR c.account_id IS NULL)
    AND (@range_start::timestamptz IS NULL OR c.ts >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.ts < @range_end::timestamptz)
    AND (@action_in::text[] IS NULL OR c.action = any(@action_in::text[]))
    AND (@action_out::text[] IS NULL OR c.action != all(@action_out::text[]))
    AND (@actor_in::text[] IS NULL OR c.actor = any(@actor_in::text[]))
    AND (@actor_out::text[] IS NULL OR c.actor != all(@actor_out::text[]))
    AND (@rule_in::text[] IS NULL OR c.rule_id = any(@rule_in::text[]))
    AND (@rule_out::text[] IS NULL OR c.rule_id != all(@rule_out::text[]))
    AND (@day_in::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
ORDER BY
    CASE WHEN @descending::boolean THEN c.ts END DESC,
    CASE WHEN NOT @descending::boolean THEN c.ts END ASC,
    c.id ASC
LIMIT 50 OFFSET @row_offset;

-- name: ChangesByAction :many
-- Every group of the policy_changes dataset by action, under the same filters, ordered by count and
-- then by the group's key, the order the bars draw (docs/UI.md section 17.1).
SELECT
    c.action,
    count(*) AS changes
FROM policy_changes AS c
WHERE
    (c.account_id = @account_id::text OR c.account_id IS NULL)
    AND (@base_rows::boolean OR c.account_id IS NOT NULL)
    AND (@own_rows::boolean OR c.account_id IS NULL)
    AND (@range_start::timestamptz IS NULL OR c.ts >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.ts < @range_end::timestamptz)
    AND (@action_in::text[] IS NULL OR c.action = any(@action_in::text[]))
    AND (@action_out::text[] IS NULL OR c.action != all(@action_out::text[]))
    AND (@actor_in::text[] IS NULL OR c.actor = any(@actor_in::text[]))
    AND (@actor_out::text[] IS NULL OR c.actor != all(@actor_out::text[]))
    AND (@rule_in::text[] IS NULL OR c.rule_id = any(@rule_in::text[]))
    AND (@rule_out::text[] IS NULL OR c.rule_id != all(@rule_out::text[]))
    AND (@day_in::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
GROUP BY c.action
ORDER BY changes DESC, c.action ASC;

-- name: ChangesByScope :many
-- Every group of the policy_changes dataset by scope, as ChangesByAction. The base policy's changes
-- are the null group, last.
SELECT
    c.account_id,
    count(*) AS changes
FROM policy_changes AS c
WHERE
    (c.account_id = @account_id::text OR c.account_id IS NULL)
    AND (@base_rows::boolean OR c.account_id IS NOT NULL)
    AND (@own_rows::boolean OR c.account_id IS NULL)
    AND (@range_start::timestamptz IS NULL OR c.ts >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.ts < @range_end::timestamptz)
    AND (@action_in::text[] IS NULL OR c.action = any(@action_in::text[]))
    AND (@action_out::text[] IS NULL OR c.action != all(@action_out::text[]))
    AND (@actor_in::text[] IS NULL OR c.actor = any(@actor_in::text[]))
    AND (@actor_out::text[] IS NULL OR c.actor != all(@actor_out::text[]))
    AND (@rule_in::text[] IS NULL OR c.rule_id = any(@rule_in::text[]))
    AND (@rule_out::text[] IS NULL OR c.rule_id != all(@rule_out::text[]))
    AND (@day_in::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
GROUP BY c.account_id
ORDER BY changes DESC, c.account_id ASC NULLS LAST;

-- name: ChangesByActor :many
-- Every group of the policy_changes dataset by the identity that made the change, as ChangesByAction.
SELECT
    c.actor,
    count(*) AS changes
FROM policy_changes AS c
WHERE
    (c.account_id = @account_id::text OR c.account_id IS NULL)
    AND (@base_rows::boolean OR c.account_id IS NOT NULL)
    AND (@own_rows::boolean OR c.account_id IS NULL)
    AND (@range_start::timestamptz IS NULL OR c.ts >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.ts < @range_end::timestamptz)
    AND (@action_in::text[] IS NULL OR c.action = any(@action_in::text[]))
    AND (@action_out::text[] IS NULL OR c.action != all(@action_out::text[]))
    AND (@actor_in::text[] IS NULL OR c.actor = any(@actor_in::text[]))
    AND (@actor_out::text[] IS NULL OR c.actor != all(@actor_out::text[]))
    AND (@rule_in::text[] IS NULL OR c.rule_id = any(@rule_in::text[]))
    AND (@rule_out::text[] IS NULL OR c.rule_id != all(@rule_out::text[]))
    AND (@day_in::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
GROUP BY c.actor
ORDER BY changes DESC, c.actor ASC;

-- name: ChangesByDay :many
-- Every group of the policy_changes dataset by the UTC day of the change, as ChangesByAction.
SELECT
    (c.ts AT TIME ZONE 'UTC')::date AS day,
    count(*) AS changes
FROM policy_changes AS c
WHERE
    (c.account_id = @account_id::text OR c.account_id IS NULL)
    AND (@base_rows::boolean OR c.account_id IS NOT NULL)
    AND (@own_rows::boolean OR c.account_id IS NULL)
    AND (@range_start::timestamptz IS NULL OR c.ts >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.ts < @range_end::timestamptz)
    AND (@action_in::text[] IS NULL OR c.action = any(@action_in::text[]))
    AND (@action_out::text[] IS NULL OR c.action != all(@action_out::text[]))
    AND (@actor_in::text[] IS NULL OR c.actor = any(@actor_in::text[]))
    AND (@actor_out::text[] IS NULL OR c.actor != all(@actor_out::text[]))
    AND (@rule_in::text[] IS NULL OR c.rule_id = any(@rule_in::text[]))
    AND (@rule_out::text[] IS NULL OR c.rule_id != all(@rule_out::text[]))
    AND (@day_in::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (c.ts AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
GROUP BY day
ORDER BY changes DESC, day ASC;

-- name: RuleHistory :many
-- One rule's changes, newest first, the changes recorded under its identifier and its scope, the
-- account's own when base is false and the base policy's when it is true (docs/UI.md section 8.7).
SELECT
    c.id,
    c.ts,
    c.account_id,
    c.actor,
    c.action,
    c.rule_id,
    c.suffixes_before,
    c.suffixes_after
FROM policy_changes AS c
WHERE
    (c.account_id = @account_id::text OR c.account_id IS NULL)
    AND c.rule_id = @rule_id
    AND @base::boolean = (c.account_id IS NULL)
ORDER BY c.ts DESC, c.id DESC;

-- name: LatestChange :one
-- When the account's policy last changed, its own rules or the base policy's, null before its first
-- change, for the policy screen's L0 strip.
SELECT max(c.ts)::timestamptz AS latest
FROM policy_changes AS c
WHERE c.account_id = @account_id::text OR c.account_id IS NULL;
