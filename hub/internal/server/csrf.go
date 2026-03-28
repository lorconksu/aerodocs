package server

import "net/http"

// csrfMiddleware enforces the double-submit cookie pattern for mutating requests.
// Safe methods (GET, HEAD, OPTIONS) are exempt. Requests using Bearer authentication
// are also exempt since they originate from non-browser clients.
func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Safe methods are exempt.
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Bearer auth is exempt (non-browser clients).
		if isUsingBearerAuth(r) {
			next.ServeHTTP(w, r)
			return
		}

		// No cookie-based session means no CSRF risk (e.g., login, register).
		// If neither the access cookie nor the CSRF cookie is present, this
		// request is not using cookie auth, so skip CSRF validation.
		if readCSRFCookie(r) == "" && readAccessToken(r) == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Validate CSRF: X-CSRF-Token header must match aerodocs_csrf cookie.
		cookieToken := readCSRFCookie(r)
		headerToken := readCSRFToken(r)

		if cookieToken == "" || headerToken == "" || cookieToken != headerToken {
			respondError(w, http.StatusForbidden, "CSRF validation failed")
			return
		}

		next.ServeHTTP(w, r)
	})
}
