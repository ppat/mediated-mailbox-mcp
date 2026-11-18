// Package core is the pure decision of the shared transaction helper, whether a transaction may go
// on once the account setting it made has been read back (ADR-0047, ADR-0112). The shell package
// db/tx does the transaction and the two statements, hands this package the value it read, and
// goes on only when the answer is nil.
package core

import "errors"

// ErrAccountNotSet is the answer when the account setting does not read back as the account, so the
// statements would see no rows rather than fail.
var ErrAccountNotSet = errors.New("the transaction's account setting does not hold the account")

// ErrBaseNotSet is the answer when a base-policy transaction's account does not read back as empty.
var ErrBaseNotSet = errors.New("the transaction's account setting does not read back as empty")

// AccountHeld answers whether a transaction that set its account to account may go on, given the
// value the setting reads back as. It may only when the setting holds the account. An empty account
// never passes, since it is the state a transaction that never set the account is in, and a setting
// that read back empty is the state every transaction ends in.
func AccountHeld(account, readBack string) error {
	if readBack == "" || readBack != account {
		return ErrAccountNotSet
	}
	return nil
}

// BaseHeld answers whether a base-policy transaction, whose account is the empty string, may go on,
// given the value the setting reads back as. It may only when the setting reads back empty.
func BaseHeld(readBack string) error {
	if readBack != "" {
		return ErrBaseNotSet
	}
	return nil
}
