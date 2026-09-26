// Package names declares functions named like the environment readers, which read nothing.
package names

type env struct{}

func (env) Getenv(string) string { return "" }

func Getenv(string) string { return "" }

func Use() string { return Getenv("HOME") + env{}.Getenv("HOME") }
