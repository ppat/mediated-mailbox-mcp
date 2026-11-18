package server

// Statuses is the table of each code's status, which the package's tests read so a code missing from
// it fails rather than falling back to a bad request. It exists only in this package's test binary.
var Statuses = statuses
