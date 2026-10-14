// Package lookalike sits in a directory whose name only starts with api, so it is not the API server's.
package lookalike

func start(f func()) {
	go f()
}
