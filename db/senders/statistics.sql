-- name: RebuildSender :exec
-- Rebuilds the statistics of the account's sender at domain from the messages the index holds for
-- it, its volume, first and last message, List-Id share, label distribution, the first twenty of its
-- local parts and display names in sorted order, and its class (ADR-0016, ADR-0017). A sender is
-- restricted when any of its messages was classified restricted. The statistics are read from the
-- stored messages rather than added to, so a page ingested twice leaves them as one ingestion does.
INSERT INTO senders AS s (
    account_id,
    domain,
    local_part_sample,
    display_names,
    message_count,
    first_seen,
    last_seen,
    has_list_id_ratio,
    label_distribution,
    sender_class
)
SELECT
    m.account_id,
    m.from_domain,
    (array_agg(DISTINCT split_part(m.from_email::text, '@', 1)))[1:20] AS local_part_sample,
    coalesce((array_agg(DISTINCT m.from_name) FILTER (WHERE m.from_name <> ''))[1:20], '{}') AS display_names,
    count(*) AS message_count,
    min(m.sent_at) AS first_seen,
    max(m.sent_at) AS last_seen,
    avg((m.list_id IS NOT NULL)::integer)::real AS has_list_id_ratio,
    (
        SELECT coalesce(jsonb_object_agg(l.label, l.messages), '{}'::jsonb)
        FROM (
            SELECT
                x.label,
                count(*) AS messages
            FROM messages AS n
            CROSS JOIN LATERAL unnest(n.labels) AS x (label)
            WHERE n.account_id = m.account_id AND n.from_domain = m.from_domain
            GROUP BY x.label
        ) AS l
    ) AS label_distribution,
    CASE WHEN bool_or(m.sender_class = 'restricted') THEN 'restricted' ELSE 'normal' END AS sender_class
FROM messages AS m
WHERE m.account_id = @account_id AND m.from_domain = @domain
GROUP BY m.account_id, m.from_domain
ON CONFLICT (account_id, domain) DO UPDATE
    SET
        local_part_sample = excluded.local_part_sample,
        display_names = excluded.display_names,
        message_count = excluded.message_count,
        first_seen = excluded.first_seen,
        last_seen = excluded.last_seen,
        has_list_id_ratio = excluded.has_list_id_ratio,
        label_distribution = excluded.label_distribution,
        sender_class = excluded.sender_class;

-- name: AddScanHits :execrows
-- Adds the messages whose scan verdict carried a content flag to the prior hits of the account's
-- sender at domain, which the scan gate reads (ADR-0093).
UPDATE senders
SET scan_hit_count = scan_hit_count + @hits
WHERE account_id = @account_id AND domain = @domain;
