package middleware

import (
	"net/http"
	"strings"
)

// MethodOverride checks POST requests for a "_method" form parameter,
// and overrides the HTTP request method with its value (e.g. PUT or DELETE)
// to support form method overrides in HTML forms.
func MethodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// ParseForm first to make sure FormValue works correctly
			_ = r.ParseForm()
			method := r.FormValue("_method")
			if method != "" {
				r.Method = strings.ToUpper(method)
			}
		}
		next.ServeHTTP(w, r)
	})
}
