// Command worker stands for the worker's entry.
package main

func main() {
	done := make(chan struct{})
	go close(done) // want "starts a goroutine outside schedule.Go"
	<-done
}
