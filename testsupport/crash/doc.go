// Package crash is the crash harness. It generates operation sequences with a crash step, reduces a
// failing sequence against an in-memory model, and replays the reduced sequence against PostgreSQL.
//
// It imports rapid, so only crash-sequence test files may import it.
package crash
