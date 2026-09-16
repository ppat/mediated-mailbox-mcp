-- +goose Up
-- A migration creating code inside the database on purpose, one statement per kind the lint refuses.

-- want database-code
-- +goose StatementBegin
CREATE FUNCTION fixture_touch() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RETURN NEW;
END
$$;
-- +goose StatementEnd

-- want database-code
CREATE TRIGGER fixture_touch BEFORE UPDATE ON fixture_senders FOR EACH ROW EXECUTE FUNCTION fixture_touch();

-- want database-code
CREATE PROCEDURE fixture_reset() LANGUAGE sql AS 'UPDATE fixture_senders SET message_count = 0';

-- want database-code
CREATE FUNCTION fixture_sql() RETURNS int LANGUAGE sql RETURN 1;

-- want database-code
CREATE EVENT TRIGGER fixture_ddl ON ddl_command_start EXECUTE FUNCTION fixture_touch();

-- want database-code
-- +goose StatementBegin
DO $$
BEGIN
  CREATE ROLE fixture_role;
END
$$;
-- +goose StatementEnd

-- Declarative features are not code, so these must not be reported.
ALTER TABLE fixture_senders ENABLE ROW LEVEL SECURITY;
CREATE POLICY fixture_account ON fixture_senders USING (account_id = current_setting('app.account'));
CREATE INDEX ON fixture_senders (account_id, message_count);
