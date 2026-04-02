package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ok200 is a simple handler that returns 200 OK for use as the "next" handler.
var ok200 = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

const testExpected403 = "expected 403, got %d"

func TestCSRFMiddleware_BlocksMutationWithoutToken(t *testing.T) {
	handler := csrfMiddleware(ok200)

	req := httptest.NewRequest(http.MethodPost, testAPISomething, nil)
	req.AddCookie(&http.Cookie{Name: "aerodocs_csrf", Value: "tok123"})
	// No X-CSRF-Token header.

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf(testExpected403, rr.Code)
	}
}

func TestCSRFMiddleware_AllowsMatchingToken(t *testing.T) {
	handler := csrfMiddleware(ok200)

	req := httptest.NewRequest(http.MethodPost, testAPISomething, nil)
	req.AddCookie(&http.Cookie{Name: "aerodocs_csrf", Value: "tok123"})
	req.Header.Set(testCSRFTokenHdr, "tok123")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(testExpected200, rr.Code)
	}
}

func TestCSRFMiddleware_MismatchToken(t *testing.T) {
	handler := csrfMiddleware(ok200)

	req := httptest.NewRequest(http.MethodPost, testAPISomething, nil)
	req.AddCookie(&http.Cookie{Name: "aerodocs_csrf", Value: "tok123"})
	req.Header.Set(testCSRFTokenHdr, "different-value")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf(testExpected403, rr.Code)
	}
}

func TestCSRFMiddleware_AllowsGET(t *testing.T) {
	handler := csrfMiddleware(ok200)

	req := httptest.NewRequest(http.MethodGet, testAPISomething, nil)
	// No CSRF tokens at all.

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(testExpected200, rr.Code)
	}
}

func TestCSRFMiddleware_AllowsHEAD(t *testing.T) {
	handler := csrfMiddleware(ok200)

	req := httptest.NewRequest(http.MethodHead, testAPISomething, nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(testExpected200, rr.Code)
	}
}

func TestCSRFMiddleware_AllowsBearerAuth(t *testing.T) {
	handler := csrfMiddleware(ok200)

	req := httptest.NewRequest(http.MethodPost, testAPISomething, nil)
	req.Header.Set("Authorization", "Bearer some-api-token")
	// No CSRF tokens.

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(testExpected200, rr.Code)
	}
}

func TestCSRFMiddleware_NoCookies_SkipsValidation(t *testing.T) {
	handler := csrfMiddleware(ok200)

	// No cookies at all = not a cookie-based session (e.g., login/register).
	// CSRF validation should be skipped.
	req := httptest.NewRequest(http.MethodPost, testAPISomething, nil)
	req.Header.Set(testCSRFTokenHdr, "tok123")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 (CSRF skipped for non-cookie session), got %d", rr.Code)
	}
}

func TestHostnameOnly(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com:8080", "example.com"},
		{"example.com", "example.com"},
		{"localhost:3000", "localhost"},
		{"localhost", "localhost"},
		{"[::1]:8080", "::1"},
		{"192.168.1.1:443", "192.168.1.1"},
		{"192.168.1.1", "192.168.1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := hostnameOnly(tt.input)
			if result != tt.expected {
				t.Errorf("hostnameOnly(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidOrigin(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		origin   string
		expected bool
	}{
		{"same host no port", "example.com", "https://example.com", true},
		{"same host different port", "example.com:8080", "https://example.com", true},
		{"same host both ports", "example.com:8080", "https://example.com:443", true},
		{"different host", "example.com", "https://evil.com", false},
		{"invalid origin URL", "example.com", "://bad-url", false},
		{"localhost match", "localhost:3000", "http://localhost:5173", true},
		{"localhost vs 127.0.0.1", "127.0.0.1:3000", "http://localhost:5173", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
			req.Host = tt.host
			result := isValidOrigin(req, tt.origin)
			if result != tt.expected {
				t.Errorf("isValidOrigin(host=%q, origin=%q) = %v, want %v",
					tt.host, tt.origin, result, tt.expected)
			}
		})
	}
}

func TestCSRFMiddleware_RejectsInvalidOrigin(t *testing.T) {
	handler := csrfMiddleware(ok200)

	req := httptest.NewRequest(http.MethodPost, testAPISomething, nil)
	req.Host = "example.com"
	req.Header.Set("Origin", "https://evil.com")
	req.AddCookie(&http.Cookie{Name: "aerodocs_csrf", Value: "tok123"})
	req.Header.Set(testCSRFTokenHdr, "tok123")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-origin request, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_AllowsMatchingOrigin(t *testing.T) {
	handler := csrfMiddleware(ok200)

	req := httptest.NewRequest(http.MethodPost, testAPISomething, nil)
	req.Host = "example.com:8080"
	req.Header.Set("Origin", "https://example.com")
	req.AddCookie(&http.Cookie{Name: "aerodocs_csrf", Value: "tok123"})
	req.Header.Set(testCSRFTokenHdr, "tok123")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(testExpected200, rr.Code)
	}
}

func TestCSRFMiddleware_AccessCookiePresent_RequiresCSRF(t *testing.T) {
	handler := csrfMiddleware(ok200)

	// Access cookie present but no CSRF cookie/header = should be blocked.
	req := httptest.NewRequest(http.MethodPost, testAPISomething, nil)
	req.AddCookie(&http.Cookie{Name: "aerodocs_access", Value: "some-token"})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf(testExpected403, rr.Code)
	}
}
