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
