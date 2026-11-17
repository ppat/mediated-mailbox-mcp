-- The test library's one user, as the real library has one application user. The grant test creates it
-- inside a transaction it rolls back, so it never outlives the test.
CREATE ROLE check_fixture_user NOLOGIN;
