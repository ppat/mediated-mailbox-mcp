-- name: SearchPage :many
-- One page of the messages the query selects, in the order sort_by and descending name, after the
-- position a cursor names or from the start when it names none. Ties within one sort value are
-- broken by the message identifier, ascending, so the order is total.
SELECT
    m.message_id,
    m.thread_id,
    m.from_email,
    m.from_name,
    m.subject,
    m.sent_at,
    m.labels,
    m.flags,
    m.has_attachments,
    m.attachment_types,
    m.content_flags,
    m.scan_state
FROM messages AS m
WHERE
    m.account_id = @account_id
    AND (@after::timestamptz IS NULL OR m.sent_at >= @after::timestamptz)
    AND (@before::timestamptz IS NULL OR m.sent_at < @before::timestamptz)
    AND (@from_emails::citext[] IS NULL OR m.from_email = any(@from_emails::citext[]))
    AND (@from_domains::citext[] IS NULL OR m.from_domain = any(@from_domains::citext[]))
    AND (@labels::text[] IS NULL OR m.labels @> @labels::text[])
    AND (@excluded_labels::text[] IS NULL OR NOT m.labels && @excluded_labels::text[])
    AND (@labels_within::text[] IS NULL OR m.labels <@ @labels_within::text[])
    AND (@subject_patterns::text[] IS NULL OR m.subject ILIKE all(@subject_patterns::text[]))
    AND (
        @unread::boolean[] IS NULL
        OR (NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) = any(@unread::boolean[])
    )
    AND (
        @starred::boolean[] IS NULL
        OR coalesce((m.flags ->> 'starred')::boolean, FALSE) = any(@starred::boolean[])
    )
    AND (@has_attachments::boolean[] IS NULL OR m.has_attachments = any(@has_attachments::boolean[]))
    AND (
        @restricted::boolean[] IS NULL
        OR (
            NOT EXISTS (
                SELECT 1 FROM unnest(@normal_domains::text[]) AS n (d)
                WHERE n.d = m.from_domain::text
            )
        ) = any(@restricted::boolean[])
    )
    AND (
        @first_page::boolean
        OR (
            @sort_by::text = 'date' AND @descending::boolean AND (
                m.sent_at < @after_sent_at::timestamptz
                OR (m.sent_at = @after_sent_at::timestamptz AND m.message_id > @after_message_id::text)
            )
        )
        OR (
            @sort_by::text = 'date' AND NOT @descending::boolean AND (
                m.sent_at > @after_sent_at::timestamptz
                OR (m.sent_at = @after_sent_at::timestamptz AND m.message_id > @after_message_id::text)
            )
        )
        OR (
            @sort_by::text = 'sender' AND @descending::boolean AND (
                m.from_email < @after_sender::citext
                OR (m.from_email = @after_sender::citext AND m.message_id > @after_message_id::text)
            )
        )
        OR (
            @sort_by::text = 'sender' AND NOT @descending::boolean AND (
                m.from_email > @after_sender::citext
                OR (m.from_email = @after_sender::citext AND m.message_id > @after_message_id::text)
            )
        )
        OR (
            @sort_by::text = 'subject' AND @descending::boolean AND (
                coalesce(m.subject, '') < @after_subject::text
                OR (coalesce(m.subject, '') = @after_subject::text AND m.message_id > @after_message_id::text)
            )
        )
        OR (
            @sort_by::text = 'subject' AND NOT @descending::boolean AND (
                coalesce(m.subject, '') > @after_subject::text
                OR (coalesce(m.subject, '') = @after_subject::text AND m.message_id > @after_message_id::text)
            )
        )
    )
ORDER BY
    CASE WHEN @sort_by::text = 'date' AND @descending::boolean THEN m.sent_at END DESC,
    CASE WHEN @sort_by::text = 'date' AND NOT @descending::boolean THEN m.sent_at END ASC,
    CASE WHEN @sort_by::text = 'sender' AND @descending::boolean THEN m.from_email END DESC,
    CASE WHEN @sort_by::text = 'sender' AND NOT @descending::boolean THEN m.from_email END ASC,
    CASE WHEN @sort_by::text = 'subject' AND @descending::boolean THEN coalesce(m.subject, '') END DESC,
    CASE WHEN @sort_by::text = 'subject' AND NOT @descending::boolean THEN coalesce(m.subject, '') END ASC,
    m.message_id ASC
LIMIT @page_size;

-- name: SearchSummary :one
-- The number of messages the query selects, of their threads, of the unread among them and of those
-- with attachments, and the dates of the oldest and the newest. A message whose flags do not say it
-- was read counts as unread.
SELECT
    count(*) AS messages,
    count(DISTINCT m.thread_id) AS threads,
    count(*) FILTER (WHERE NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) AS unread,
    count(*) FILTER (WHERE m.has_attachments) AS with_attachments,
    min(m.sent_at)::timestamptz AS oldest_at,
    max(m.sent_at)::timestamptz AS newest_at
FROM messages AS m
WHERE
    m.account_id = @account_id
    AND (@after::timestamptz IS NULL OR m.sent_at >= @after::timestamptz)
    AND (@before::timestamptz IS NULL OR m.sent_at < @before::timestamptz)
    AND (@from_emails::citext[] IS NULL OR m.from_email = any(@from_emails::citext[]))
    AND (@from_domains::citext[] IS NULL OR m.from_domain = any(@from_domains::citext[]))
    AND (@labels::text[] IS NULL OR m.labels @> @labels::text[])
    AND (@excluded_labels::text[] IS NULL OR NOT m.labels && @excluded_labels::text[])
    AND (@labels_within::text[] IS NULL OR m.labels <@ @labels_within::text[])
    AND (@subject_patterns::text[] IS NULL OR m.subject ILIKE all(@subject_patterns::text[]))
    AND (
        @unread::boolean[] IS NULL
        OR (NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) = any(@unread::boolean[])
    )
    AND (
        @starred::boolean[] IS NULL
        OR coalesce((m.flags ->> 'starred')::boolean, FALSE) = any(@starred::boolean[])
    )
    AND (@has_attachments::boolean[] IS NULL OR m.has_attachments = any(@has_attachments::boolean[]))
    AND (
        @restricted::boolean[] IS NULL
        OR (
            NOT EXISTS (
                SELECT 1 FROM unnest(@normal_domains::text[]) AS n (d)
                WHERE n.d = m.from_domain::text
            )
        ) = any(@restricted::boolean[])
    );

-- name: CountBySender :many
-- One page of the groups the query's messages form by sender address, the group with the most
-- messages first and ties by address, after the position a cursor names or from the start.
SELECT
    m.from_email,
    count(*) AS messages,
    count(DISTINCT m.thread_id) AS threads,
    count(*) FILTER (WHERE NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) AS unread,
    count(*) FILTER (WHERE m.has_attachments) AS with_attachments,
    min(m.sent_at)::timestamptz AS oldest_at,
    max(m.sent_at)::timestamptz AS newest_at
FROM messages AS m
WHERE
    m.account_id = @account_id
    AND (@after::timestamptz IS NULL OR m.sent_at >= @after::timestamptz)
    AND (@before::timestamptz IS NULL OR m.sent_at < @before::timestamptz)
    AND (@from_emails::citext[] IS NULL OR m.from_email = any(@from_emails::citext[]))
    AND (@from_domains::citext[] IS NULL OR m.from_domain = any(@from_domains::citext[]))
    AND (@labels::text[] IS NULL OR m.labels @> @labels::text[])
    AND (@excluded_labels::text[] IS NULL OR NOT m.labels && @excluded_labels::text[])
    AND (@labels_within::text[] IS NULL OR m.labels <@ @labels_within::text[])
    AND (@subject_patterns::text[] IS NULL OR m.subject ILIKE all(@subject_patterns::text[]))
    AND (
        @unread::boolean[] IS NULL
        OR (NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) = any(@unread::boolean[])
    )
    AND (
        @starred::boolean[] IS NULL
        OR coalesce((m.flags ->> 'starred')::boolean, FALSE) = any(@starred::boolean[])
    )
    AND (@has_attachments::boolean[] IS NULL OR m.has_attachments = any(@has_attachments::boolean[]))
    AND (
        @restricted::boolean[] IS NULL
        OR (
            NOT EXISTS (
                SELECT 1 FROM unnest(@normal_domains::text[]) AS n (d)
                WHERE n.d = m.from_domain::text
            )
        ) = any(@restricted::boolean[])
    )
GROUP BY m.from_email
HAVING
    @first_page::boolean
    OR count(*) < @after_count::bigint
    OR (count(*) = @after_count::bigint AND m.from_email > @after_key::citext)
ORDER BY messages DESC, m.from_email ASC
LIMIT @page_size;

-- name: CountBySenderDomain :many
-- One page of the groups the query's messages form by sender domain, ordered and continued as the
-- groups by sender address are.
SELECT
    m.from_domain,
    count(*) AS messages,
    count(DISTINCT m.thread_id) AS threads,
    count(*) FILTER (WHERE NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) AS unread,
    count(*) FILTER (WHERE m.has_attachments) AS with_attachments,
    min(m.sent_at)::timestamptz AS oldest_at,
    max(m.sent_at)::timestamptz AS newest_at
FROM messages AS m
WHERE
    m.account_id = @account_id
    AND (@after::timestamptz IS NULL OR m.sent_at >= @after::timestamptz)
    AND (@before::timestamptz IS NULL OR m.sent_at < @before::timestamptz)
    AND (@from_emails::citext[] IS NULL OR m.from_email = any(@from_emails::citext[]))
    AND (@from_domains::citext[] IS NULL OR m.from_domain = any(@from_domains::citext[]))
    AND (@labels::text[] IS NULL OR m.labels @> @labels::text[])
    AND (@excluded_labels::text[] IS NULL OR NOT m.labels && @excluded_labels::text[])
    AND (@labels_within::text[] IS NULL OR m.labels <@ @labels_within::text[])
    AND (@subject_patterns::text[] IS NULL OR m.subject ILIKE all(@subject_patterns::text[]))
    AND (
        @unread::boolean[] IS NULL
        OR (NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) = any(@unread::boolean[])
    )
    AND (
        @starred::boolean[] IS NULL
        OR coalesce((m.flags ->> 'starred')::boolean, FALSE) = any(@starred::boolean[])
    )
    AND (@has_attachments::boolean[] IS NULL OR m.has_attachments = any(@has_attachments::boolean[]))
    AND (
        @restricted::boolean[] IS NULL
        OR (
            NOT EXISTS (
                SELECT 1 FROM unnest(@normal_domains::text[]) AS n (d)
                WHERE n.d = m.from_domain::text
            )
        ) = any(@restricted::boolean[])
    )
GROUP BY m.from_domain
HAVING
    @first_page::boolean
    OR count(*) < @after_count::bigint
    OR (count(*) = @after_count::bigint AND m.from_domain > @after_key::citext)
ORDER BY messages DESC, m.from_domain ASC
LIMIT @page_size;

-- name: CountByLabel :many
-- Every group the query's labelled messages form by label, the group with the most messages first and
-- ties by label. A message counts in the group of each label it carries. The messages with no label
-- are counted by SearchSummary with labels_within set to the empty list. The groups are as many as the
-- account's labels, so the service layer pages them rather than the statement.
SELECT
    l.label::text AS label,
    count(*) AS messages,
    count(DISTINCT m.thread_id) AS threads,
    count(*) FILTER (WHERE NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) AS unread,
    count(*) FILTER (WHERE m.has_attachments) AS with_attachments,
    min(m.sent_at)::timestamptz AS oldest_at,
    max(m.sent_at)::timestamptz AS newest_at
FROM messages AS m
CROSS JOIN LATERAL unnest(m.labels) AS l (label)
WHERE
    m.account_id = @account_id
    AND (@after::timestamptz IS NULL OR m.sent_at >= @after::timestamptz)
    AND (@before::timestamptz IS NULL OR m.sent_at < @before::timestamptz)
    AND (@from_emails::citext[] IS NULL OR m.from_email = any(@from_emails::citext[]))
    AND (@from_domains::citext[] IS NULL OR m.from_domain = any(@from_domains::citext[]))
    AND (@labels::text[] IS NULL OR m.labels @> @labels::text[])
    AND (@excluded_labels::text[] IS NULL OR NOT m.labels && @excluded_labels::text[])
    AND (@labels_within::text[] IS NULL OR m.labels <@ @labels_within::text[])
    AND (@subject_patterns::text[] IS NULL OR m.subject ILIKE all(@subject_patterns::text[]))
    AND (
        @unread::boolean[] IS NULL
        OR (NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) = any(@unread::boolean[])
    )
    AND (
        @starred::boolean[] IS NULL
        OR coalesce((m.flags ->> 'starred')::boolean, FALSE) = any(@starred::boolean[])
    )
    AND (@has_attachments::boolean[] IS NULL OR m.has_attachments = any(@has_attachments::boolean[]))
    AND (
        @restricted::boolean[] IS NULL
        OR (
            NOT EXISTS (
                SELECT 1 FROM unnest(@normal_domains::text[]) AS n (d)
                WHERE n.d = m.from_domain::text
            )
        ) = any(@restricted::boolean[])
    )
GROUP BY l.label
ORDER BY messages DESC, l.label ASC;

-- name: CountByMonth :many
-- Every group the query's messages form by the UTC calendar month they were sent in, keyed by the
-- instant the month starts, the group with the most messages first and ties by month. The groups are as
-- many as the months the account's mail spans, so the service layer pages them rather than the
-- statement.
SELECT
    g.month::timestamptz AS month,
    count(*) AS messages,
    count(DISTINCT g.thread_id) AS threads,
    count(*) FILTER (WHERE NOT coalesce((g.flags ->> 'read')::boolean, FALSE)) AS unread,
    count(*) FILTER (WHERE g.has_attachments) AS with_attachments,
    min(g.sent_at)::timestamptz AS oldest_at,
    max(g.sent_at)::timestamptz AS newest_at
FROM (
    SELECT
        m.thread_id,
        m.flags,
        m.has_attachments,
        m.sent_at,
        date_trunc('month', m.sent_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS month
    FROM messages AS m
    WHERE
        m.account_id = @account_id
        AND (@after::timestamptz IS NULL OR m.sent_at >= @after::timestamptz)
        AND (@before::timestamptz IS NULL OR m.sent_at < @before::timestamptz)
        AND (@from_emails::citext[] IS NULL OR m.from_email = any(@from_emails::citext[]))
        AND (@from_domains::citext[] IS NULL OR m.from_domain = any(@from_domains::citext[]))
        AND (@labels::text[] IS NULL OR m.labels @> @labels::text[])
        AND (@excluded_labels::text[] IS NULL OR NOT m.labels && @excluded_labels::text[])
        AND (@labels_within::text[] IS NULL OR m.labels <@ @labels_within::text[])
        AND (@subject_patterns::text[] IS NULL OR m.subject ILIKE all(@subject_patterns::text[]))
        AND (
            @unread::boolean[] IS NULL
            OR (NOT coalesce((m.flags ->> 'read')::boolean, FALSE)) = any(@unread::boolean[])
        )
        AND (
            @starred::boolean[] IS NULL
            OR coalesce((m.flags ->> 'starred')::boolean, FALSE) = any(@starred::boolean[])
        )
        AND (@has_attachments::boolean[] IS NULL OR m.has_attachments = any(@has_attachments::boolean[]))
        AND (
            @restricted::boolean[] IS NULL
            OR (
                NOT EXISTS (
                    SELECT 1 FROM unnest(@normal_domains::text[]) AS n (d)
                    WHERE n.d = m.from_domain::text
                )
            ) = any(@restricted::boolean[])
        )
) AS g
GROUP BY g.month
ORDER BY messages DESC, g.month ASC;
