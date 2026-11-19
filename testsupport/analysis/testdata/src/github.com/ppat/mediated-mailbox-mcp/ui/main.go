// Command ui stands for the UI's main.go, which the rule covers too.
package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", http.NotFoundHandler()) // want `\(\*http.ServeMux\).Handle registers a route`
}
