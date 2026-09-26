// Package service is the mediator's service layer, the one layer both protocol roots call. The
// Redaction Gate and the Mutation Authorizer are enacted beneath it.
//
// The operation registry is the client surface's single source (ADR-0053). The API root and the MCP
// root are generated from it, and neither adds an operation of its own, so an operation exists on
// both roots or on neither.
package service
