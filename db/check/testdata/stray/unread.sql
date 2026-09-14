-- A statement file in a directory the test library's sqlc configuration does not list, so no check
-- reads it. The test for unread SQL files must report it.
SELECT 1;
