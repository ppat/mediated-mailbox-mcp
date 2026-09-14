-- Statements over the test library's tables, each breaking the grouping check on purpose.

-- want grouping-case-parameter
-- name: CountByDimension :many
SELECT count(*) AS senders
FROM fixture_senders
WHERE account_id = @account_id
GROUP BY CASE @dimension::text WHEN 'domain' THEN domain::text END;

-- The same grouping reached through the output column's ordinal.
-- want grouping-case-parameter
-- name: CountByDimensionOrdinal :many
SELECT CASE sqlc.arg(dimension)::text WHEN 'domain' THEN domain::text END AS dimension_key, count(*) AS senders
FROM fixture_senders
WHERE account_id = @account_id
GROUP BY 1;

-- A case expression over a parameter in the sort is the sanctioned sort idiom, so it must not be
-- reported, and neither must grouping by a literal column.
-- name: CountByDomainSorted :many
SELECT domain, count(*) AS senders
FROM fixture_senders
WHERE account_id = @account_id
GROUP BY domain
ORDER BY CASE WHEN @sort::text = 'senders' THEN count(*) END DESC, domain;
