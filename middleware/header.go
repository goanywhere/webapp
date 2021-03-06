package middleware

import "net/http"

// Header adds the given response headers to the upcoming `http.ResponseWriter`.
func Header(values http.Header) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, value := range values {
				w.Header()[key] = value
			}
			next.ServeHTTP(w, r)
		})
	}
}
