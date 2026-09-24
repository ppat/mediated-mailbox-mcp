// Package gmail is the Gmail adapter. It implements the Provider Port and declares its rate
// profile.
//
// It also holds the account's installed-app OAuth grant (ADR-0011). That is the one-time consent,
// which requests only the modify scope, the credentials read from mounted files (ADR-0038), and
// the write-back of a rotated refresh token to the one writable location (ADR-0039).
package gmail
