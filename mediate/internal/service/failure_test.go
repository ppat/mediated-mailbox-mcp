package service_test

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
)

// A failed call is logged at the level its origin sets, the same on both roots. A client's refused
// request is routine, a provider's failure a warning, and a failure inside the mediator an error
// (ADR-0122).
func TestAFailureIsLoggedAtItsOriginsLevel(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want slog.Level
	}{
		{"a refused call", service.ErrUnknownAccount, slog.LevelInfo},
		{"a refused argument", service.Refuse("limit is not a number"), slog.LevelInfo},
		{"a throttle", service.FromProvider(fmt.Errorf("listing: %w", mail.ErrThrottled)), slog.LevelWarn},
		{"a refused credential", service.FromProvider(fmt.Errorf("listing: %w", mail.ErrAuthentication)), slog.LevelWarn},
		{"a failure inside the mediator", errors.New("the database did not answer"), slog.LevelError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := service.FailureLevel(c.err); got != c.want {
				t.Errorf("FailureLevel(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
