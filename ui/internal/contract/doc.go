// Package contract generates the UI's OpenAPI document, ui/contract/openapi.json, from the dataset
// registry and the handler list (ADR-0065).
//
// The generator is test code, so kin-openapi never reaches the binary. The non-test import list refuses
// it in every shipping file. TestDocument builds the document and fails when the checked-in file differs,
// which is the first of the contract pipeline's three drift checks. Run with -update, it writes the file.
//
// Browser types and the dataset descriptor table are generated from the file this package writes, under
// ui/browser. Regenerate in pipeline order, this package first.
//
//go:generate go test . -run ^TestDocument$ -update
package contract
