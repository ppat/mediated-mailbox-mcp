package state

// A test file of a pure core may declare package-level variables.
var table = []string{"a"}

func touch() { table[0] = "b" }
