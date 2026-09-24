package markdown

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// A conversion that fails or panics returns ErrUnconvertible and no Markdown, even when the library
// hands back partial output with its error, and a panic's value never reaches the error. No input is
// known to make the library panic or return partial output, so these cases hand guarded a conversion
// that does, which needs the unexported function.
func TestGuardedFailsClosed(t *testing.T) {
	type outcome struct {
		Markdown      string
		Unconvertible bool
		CarriesBody   bool
	}
	cases := []struct {
		name    string
		convert func() (string, error)
	}{
		{"an error with partial output", func() (string, error) {
			return marker.Body("partial"), errors.New("markdown: conversion stopped")
		}},
		{"a panic carrying body text", func() (string, error) {
			panic(marker.Body("panic"))
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := guarded(c.convert)
			got := outcome{
				Markdown:      out,
				Unconvertible: errors.Is(err, ErrUnconvertible),
				CarriesBody:   err != nil && strings.Contains(err.Error(), marker.BodyPrefix),
			}
			if diff := cmp.Diff(outcome{Unconvertible: true}, got, compare.Options); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
