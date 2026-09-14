// Package fake is the provider fake. It implements the Provider Port in memory for tests, and the
// rate limiter's throttling simulator is an instance of it.
//
// It is test-only. The import list for non-test code refuses it, so no deployable's running code
// can reach it.
package fake
