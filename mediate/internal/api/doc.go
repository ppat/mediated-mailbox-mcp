// Package api is the mediator's API root, a thin protocol adapter over the service layer.
//
// Its import list admits the service layer and nothing below it, so enforcement stays beneath both
// roots.
//
// The root is generated from the operation registry (ADR-0053). It holds no route table. Each
// request's path names an operation, and the root serves it only when the registry holds that
// operation, so the root serves exactly the registry's operations. The contract document,
// mediate/contract/openapi.json, is generated from the same registry by this package's tests, so
// kin-openapi never reaches the binary. TestDocument fails when the checked-in file differs, and run
// with -update it writes the file.
//
//go:generate go test . -run ^TestDocument$ -update
package api
