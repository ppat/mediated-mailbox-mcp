// Package other sits outside this module, where the rule does not reach.
package other

import "os"

func Read() string { return os.Getenv("HOME") }
