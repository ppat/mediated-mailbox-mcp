package loads

// A test file may start a goroutine.
func waitFor(f func()) {
	done := make(chan struct{})
	go func() { f(); close(done) }()
	<-done
}
