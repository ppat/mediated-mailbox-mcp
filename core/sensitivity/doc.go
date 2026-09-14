// Package sensitivity holds the sensitivity-carrying types every component shares, such as sender
// class, content flags and scan state.
//
// Each type keeps its fields unexported, is built only through its constructor, and has the most
// restrictive state as its zero value. Fixtures that must not compile against these types sit under
// testdata/mustnotcompile, one directory per case.
package sensitivity
