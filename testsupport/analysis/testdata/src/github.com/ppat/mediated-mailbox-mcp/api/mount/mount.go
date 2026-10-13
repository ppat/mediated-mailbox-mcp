// Package mount stands for the API server, which is out of scope.
package mount

import "net/http"

func mount() {
	http.NewServeMux().Handle("/api/", http.NotFoundHandler())
}
