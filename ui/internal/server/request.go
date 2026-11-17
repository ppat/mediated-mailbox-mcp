package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

// maxBody bounds how much of a request body a state-changing route reads.
const maxBody = 64 << 10

// ReadBody decodes the request's JSON body into v, admitting no field v does not declare, and answers
// the request when it cannot.
func ReadBody(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		RequestLogOf(r.Context()).Err = err
		WriteFailure(w, r, ClientFault(http.StatusBadRequest, "malformed_body", "the request body is not the JSON this request takes"))
		return false
	}
	return true
}

// DatabaseFailure reports a failed read. A cancelled request is the client going away, and its
// failure is logged, never reported as the database's.
func DatabaseFailure(w http.ResponseWriter, r *http.Request, err error) {
	RequestLogOf(r.Context()).Err = err
	if errors.Is(err, context.Canceled) {
		return
	}
	WriteFailure(w, r, DatabaseFault())
}

// UIFailure reports the UI server failing on its own.
func UIFailure(w http.ResponseWriter, r *http.Request, err error) {
	RequestLogOf(r.Context()).Err = err
	WriteFailure(w, r, UIFault())
}

// The database's codes for the refusals a write's constraint names, which a setup or a policy write
// answers as a client mistake (ADR-0016, ADR-0106).
const (
	UniqueViolation   = "23505"
	ForeignKeyBlocked = "23503"
	CheckViolation    = "23514"
)

// Violated returns the constraint a database error names, when it is code's violation.
func Violated(err error, code string) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == code {
		return pgErr.ConstraintName, true
	}
	return "", false
}

// Fault is a test's fault at a named point between a write's statements, which proves the statements
// land together or not at all (ADR-0060). The setups and policy management call it between their
// writes, and every build that ships leaves it nil.
type Fault func(at string) error

// At returns the fault injected at the named point, nil when no test injected one.
func (f Fault) At(at string) error {
	if f == nil {
		return nil
	}
	return f(at)
}
