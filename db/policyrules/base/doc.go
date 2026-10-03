// Package base is the data-access subsection for the base policy, its rules and their history rows,
// which belong to no account (ADR-0004, ADR-0102). Its statements run only in a base-policy
// transaction, which names no account, and no other subsection's do (ADR-0112). The txhelper analyser
// holds both, and the statement check scopes each statement here by the null account alone. Only the
// UI's list admits it.
package base
