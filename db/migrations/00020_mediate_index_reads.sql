-- +goose Up
-- The mediator's index reads (ADR-0108, ADR-0109). The sender domain term, the grouping by sender
-- domain and the read of the distinct senders the service layer classifies read messages.from_domain.
-- The sender listing reads the senders statistics through db/senders, every column but the stored
-- sender class, which a record forbids the mediator to act on (ADR-0002, ADR-0075), the prior scan
-- hits and the embedding, which no read serves. The rebuild of the statistics and the counts of prior
-- scan hits sit in db/senders/statistics, which backfill and delta sync admit for the writes earlier
-- migrations and 00019 grant them on senders. Delta sync's removal of a sender with no stored message
-- sits in db/messages/change. The mediator's list admits neither.
GRANT SELECT (from_domain) ON messages TO mediated_mailbox_mediate;
GRANT SELECT (
    account_id,
    domain,
    local_part_sample,
    display_names,
    message_count,
    first_seen,
    last_seen,
    has_list_id_ratio,
    label_distribution
) ON senders TO mediated_mailbox_mediate;
