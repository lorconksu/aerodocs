package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestSetAuthCookies(t *testing.T) {
	w := httptest.NewRecorder()
	setAuthCookies(w, "access123", "refresh456", false)

	cookies := w.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cookies, got %d", len(cookies))
	}

	access := findCookie(cookies, cookieAccess)
	if access == nil {
		t.Fatal("missing access cookie")
	}
	if access.Value != "access123" {
		t.Errorf("access value = %q, want %q", access.Value, "access123")
	}
	if !access.HttpOnly {
		t.Error("access cookie should be httpOnly")
	}
	if !access.Secure {
		t.Error("access cookie should be Secure in non-dev mode")
	}
	if access.SameSite != http.SameSiteStrictMode {
		t.Error("access cookie should be SameSite=Strict")
	}
	if access.Path != "/" {
		t.Errorf("access path = %q, want /", access.Path)
	}
	if access.MaxAge != 900 {
		t.Errorf("access MaxAge = %d, want 900", access.MaxAge)
	}

	refresh := findCookie(cookies, cookieRefresh)
	if refresh == nil {
		t.Fatal("missing refresh cookie")
	}
	if refresh.Value != "refresh456" {
		t.Errorf("refresh value = %q, want %q", refresh.Value, "refresh456")
	}
	if !refresh.HttpOnly {
		t.Error("refresh cookie should be httpOnly")
	}
	if !refresh.Secure {
		t.Error("refresh cookie should be Secure")
	}
	if refresh.Path != "/api/auth/refresh" {
		t.Errorf("refresh path = %q, want /api/auth/refresh", refresh.Path)
	}
	if refresh.MaxAge != 604800 {
		t.Errorf("refresh MaxAge = %d, want 604800", refresh.MaxAge)
	}

	csrf := findCookie(cookies, cookieCSRF)
	if csrf == nil {
		t.Fatal("missing CSRF cookie")
	}
	if csrf.HttpOnly {
		t.Error("CSRF cookie should NOT be httpOnly")
	}
	if !csrf.Secure {
		t.Error("CSRF cookie should be Secure")
	}
	if csrf.SameSite != http.SameSiteStrictMode {
		t.Error("CSRF cookie should be SameSite=Strict")
	}
	if csrf.MaxAge != 604800 {
		t.Errorf("CSRF MaxAge = %d, want 604800", csrf.MaxAge)
	}
	if len(csrf.Value) != 64 {
		t.Errorf("CSRF token length = %d, want 64 hex chars", len(csrf.Value))
	}
}

func TestSetAuthCookies_DevMode(t *testing.T) {
	w := httptest.NewRecorder()
	setAuthCookies(w, "access123", "refresh456", true)

	cookies := w.Result().Cookies()
	access := findCookie(cookies, cookieAccess)
	if access == nil {
		t.Fatal("missing access cookie")
	}
	if access.Secure {
		t.Error("access cookie should NOT be Secure in dev mode")
	}

	// refresh and csrf should still be Secure even in dev mode
	refresh := findCookie(cookies, cookieRefresh)
	if refresh == nil {
		t.Fatal("missing refresh cookie")
	}
	if !refresh.Secure {
		t.Error("refresh cookie should still be Secure in dev mode")
	}

	csrf := findCookie(cookies, cookieCSRF)
	if csrf == nil {
		t.Fatal("missing CSRF cookie")
	}
	if !csrf.Secure {
		t.Error("CSRF cookie should still be Secure in dev mode")
	}
}

func TestClearAuthCookies(t *testing.T) {
	w := httptest.NewRecorder()
	clearAuthCookies(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cookies, got %d", len(cookies))
	}

	for _, c := range cookies {
		if c.MaxAge != -1 {
			t.Errorf("cookie %q MaxAge = %d, want -1", c.Name, c.MaxAge)
		}
	}
}

func TestReadAccessToken_FromCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieAccess, Value: "from-cookie"})

	got := readAccessToken(r)
	if got != "from-cookie" {
		t.Errorf("readAccessToken = %q, want %q", got, "from-cookie")
	}
}

func TestReadAccessToken_FallbackToBearer(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer from-header")

	got := readAccessToken(r)
	if got != "from-header" {
		t.Errorf("readAccessToken = %q, want %q", got, "from-header")
	}
}

func TestReadAccessToken_CookiePriority(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieAccess, Value: "from-cookie"})
	r.Header.Set("Authorization", "Bearer from-header")

	got := readAccessToken(r)
	if got != "from-cookie" {
		t.Errorf("readAccessToken = %q, want %q (cookie should take priority)", got, "from-cookie")
	}
}

func TestIsUsingBearerAuth(t *testing.T) {
	t.Run("with bearer header", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer some-token")
		if !isUsingBearerAuth(r) {
			t.Error("expected true with Bearer header")
		}
	})

	t.Run("without bearer header", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		if isUsingBearerAuth(r) {
			t.Error("expected false without Bearer header")
		}
	})

	t.Run("with non-bearer auth", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Basic abc123")
		if isUsingBearerAuth(r) {
			t.Error("expected false with Basic auth header")
		}
	})
}
