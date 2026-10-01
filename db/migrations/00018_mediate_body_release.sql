-- +goose Up
-- The mediator's body operation (ADR-0002). The gate's read in db/messages names the content rules
-- that set a message's flags, which the request's audit row records. Each serve and each denial is
-- recorded through db/auditlog/record, which only the mediator's list admits, as an insert and nothing
-- else, since the audit log is append-only to every runtime role (ADR-0016). At the end of each body
-- request the mediator records the authentication attempt its adapter reports, only over an older one,
-- through db/accountstate/authentication (ADR-0097), whose predicate reads last_auth_at, which the
-- mediator already reads for the system status.
GRANT SELECT (rule_ids) ON messages TO mediated_mailbox_mediate;
GRANT INSERT (account_id, actor, action, message_id, sensitivity, rule_ids) ON audit_log TO mediated_mailbox_mediate;
GRANT USAGE ON SEQUENCE audit_log_id_seq TO mediated_mailbox_mediate;
GRANT UPDATE (last_auth_at, last_auth_outcome) ON account_state TO mediated_mailbox_mediate;
