// Package lookalike sits in a directory whose name only starts with ui, so it is out of scope.
package lookalike

import "net/http"

func mount() {
	http.NewServeMux().Handle("/", http.NotFoundHandler())
}
