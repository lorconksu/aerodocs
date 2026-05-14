package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

type fakeLDAPAuthenticator struct {
	identity LDAPIdentity
	err      error
}

func (f fakeLDAPAuthenticator) Authenticate(_ context.Context, _, _ string) (LDAPIdentity, error) {
	return f.identity, f.err
}

func TestHandleLogin_LDAPCreatesShadowUserAndRequiresTOTPSetup(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)
	s.ldapAuthenticator = fakeLDAPAuthenticator{
		identity: LDAPIdentity{
			Username:   "alice",
			Email:      "alice@example.com",
			DN:         "uid=alice,ou=people,dc=example,dc=com",
			ExternalID: "entry-alice",
			Groups:     []string{"aerodocs-viewers", "aerodocs-terminal-users"},
		},
	}

	req := httptest.NewRequest("POST", testLoginPath, mustJSON(t, model.LoginRequest{
		Username: "alice",
		Password: "ldap-password",
	}))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp model.LoginResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if !resp.RequiresTOTPSetup || resp.SetupToken == "" {
		t.Fatalf("expected LDAP login to require AeroDocs TOTP setup, got %+v", resp)
	}

	user, err := s.store.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("get ldap shadow user: %v", err)
	}
	if user.AuthProvider != model.AuthProviderLDAP {
		t.Fatalf("expected ldap auth provider, got %s", user.AuthProvider)
	}
	if user.Role != model.RoleViewer {
		t.Fatalf("expected viewer role from LDAP groups, got %s", user.Role)
	}
	if !user.TerminalAccess {
		t.Fatal("expected terminal access from LDAP terminal group")
	}
	if user.PasswordHash != "" {
		t.Fatal("expected LDAP shadow user to have no local password hash")
	}
}

func TestLoadLDAPAuthenticatorRejectsInsecureTransportByDefault(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)
	setLDAPConfigForTest(t, s, map[string]string{
		"ldap.enabled":      "true",
		"ldap.url":          "ldap://ldap.example.com:389",
		"ldap.user_base_dn": "ou=people,dc=example,dc=com",
	})

	_, err := s.loadLDAPAuthenticator()
	if err == nil || !strings.Contains(err.Error(), "LDAP secure transport required") {
		t.Fatalf("expected secure transport error, got %v", err)
	}
}

func TestLoadLDAPAuthenticatorAllowsStartTLS(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)
	setLDAPConfigForTest(t, s, map[string]string{
		"ldap.enabled":      "true",
		"ldap.url":          "ldap://ldap.example.com:389",
		"ldap.start_tls":    "true",
		"ldap.user_base_dn": "ou=people,dc=example,dc=com",
	})

	authenticator, err := s.loadLDAPAuthenticator()
	if err != nil {
		t.Fatalf("load ldap authenticator: %v", err)
	}
	if authenticator == nil {
		t.Fatal("expected ldap authenticator")
	}
}

func TestLoadLDAPConfigDecryptsEncryptedBindPassword(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)
	encrypted, err := encryptConfigSecret(s.jwtSecret, "bind-password")
	if err != nil {
		t.Fatalf("encrypt bind password: %v", err)
	}
	setLDAPConfigForTest(t, s, map[string]string{
		"ldap.enabled":       "true",
		"ldap.url":           "ldaps://ldap.example.com:636",
		"ldap.bind_password": encrypted,
		"ldap.user_base_dn":  "ou=people,dc=example,dc=com",
	})

	cfg, err := s.loadLDAPConfig()
	if err != nil {
		t.Fatalf("load ldap config: %v", err)
	}
	if cfg.BindPassword != "bind-password" {
		t.Fatalf("expected decrypted bind password, got %q", cfg.BindPassword)
	}
}

func TestBuildLDAPTLSConfigUsesConfiguredServerName(t *testing.T) {
	cfg, err := buildLDAPTLSConfig(LDAPConfig{
		URL:           "ldaps://10.10.1.39:636",
		TLSServerName: "freeipa.yiucloud.com",
	})
	if err != nil {
		t.Fatalf("build ldap tls config: %v", err)
	}
	if cfg.ServerName != "freeipa.yiucloud.com" {
		t.Fatalf("expected configured server name, got %q", cfg.ServerName)
	}
}

func TestBuildLDAPTLSConfigRejectsInvalidCA(t *testing.T) {
	_, err := buildLDAPTLSConfig(LDAPConfig{
		URL:       "ldaps://ldap.example.com:636",
		CACertPEM: "not a pem certificate",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid LDAP CA certificate") {
		t.Fatalf("expected invalid CA error, got %v", err)
	}
}

func setLDAPConfigForTest(t *testing.T, s *Server, values map[string]string) {
	t.Helper()
	for key, value := range values {
		if err := s.store.SetConfig(key, value); err != nil {
			t.Fatalf("set config %s: %v", key, err)
		}
	}
}
