package api

import "net/http"

// Policy is the content security policy on every response the UI serves, the entry document, the
// bundle, the read API, its errors and the probes (ADR-0062). It is a constant, so no request and no
// configuration can loosen it. The policy allows the entry document's token because a meta tag is not
// a script (ADR-0061).
const Policy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; frame-ancestors 'none'"

// withPolicy sets the policy before next writes anything, so no response leaves without it.
func withPolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", Policy)
		next.ServeHTTP(w, r)
	})
}
