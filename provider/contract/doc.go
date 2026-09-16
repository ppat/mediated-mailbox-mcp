// Package contract is the contract suite every Provider Port implementation passes, the fake and
// each real adapter alike.
//
// It is ordinary Go code rather than a test file, because Go cannot import another package's test
// files. Each implementation's own tests call it.
package contract
