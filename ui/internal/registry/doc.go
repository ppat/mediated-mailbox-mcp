// Package registry holds the UI's dataset registry entries. Each entry points at a generated
// data-access accessor, and the contract document is generated from the registry and the handler
// list. It also holds what more than one of ui/internal/reads, ui/internal/setup and
// ui/internal/policymanagement reads of the stored rows: an account's progress and rate state
// (Progress, Rate), the shapes a stored value is written in, OptionalText among them, and a range's
// instants (BoundsOf).
package registry
