-- The test library's runtime roles. The grant test creates them inside a transaction it rolls back, so
-- they never outlive the test.
CREATE ROLE check_fixture_reader NOLOGIN;
CREATE ROLE check_fixture_writer NOLOGIN;
