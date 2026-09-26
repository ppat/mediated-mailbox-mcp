// Package mcp is the mediator's MCP root, a thin protocol adapter over the service layer that
// registers the tool set generated from the operation registry.
//
// Its import list admits the service layer and nothing below it, so enforcement stays beneath both
// roots. It is the only package of the mediator the list admits the MCP SDK in, so a tool cannot be
// registered anywhere else (ADR-0086). It registers one tool per registry operation and nothing
// else, no prompt and no resource, so the tool set mirrors the API root one-to-one (ADR-0053).
package mcp
