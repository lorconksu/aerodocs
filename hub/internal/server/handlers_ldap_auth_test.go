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

func TestAuthenticateLDAPLoginRejectsUnauthorizedGroups(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)
	s.ldapAuthenticator = fakeLDAPAuthenticator{
		identity: LDAPIdentity{
			Username: "bob",
			Email:    "bob@example.com",
			Groups:   []string{"ipausers"},
		},
	}

	_, err := s.authenticateLDAPLogin(context.Background(), "bob", "ldap-password")
	if err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("expected unauthorized LDAP user error, got %v", err)
	}
}

func TestAuthenticateLDAPLoginMapsAdminAndTerminalAccess(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)
	s.ldapAuthenticator = fakeLDAPAuthenticator{
		identity: LDAPIdentity{
			Username:   "carol",
			Email:      "carol@example.com",
			DN:         "uid=carol,ou=people,dc=example,dc=com",
			ExternalID: "entry-carol",
			Groups:     []string{"aerodocs-admins", "aerodocs-terminal-users"},
		},
	}

	user, err := s.authenticateLDAPLogin(context.Background(), "carol", "ldap-password")
	if err != nil {
		t.Fatalf("authenticate ldap login: %v", err)
	}
	if user.Role != model.RoleAdmin {
		t.Fatalf("role = %s, want admin", user.Role)
	}
	if !user.TerminalAccess {
		t.Fatal("expected terminal access")
	}
	if user.LDAPUsername != "carol" || user.LDAPDN == "" || user.ExternalID != "entry-carol" {
		t.Fatalf("unexpected ldap metadata: %+v", user)
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

func TestLoadLDAPConfigUsesDefaultsAndBooleanVariants(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)
	setLDAPConfigForTest(t, s, map[string]string{
		"ldap.enabled":                  "YES",
		"ldap.url":                      "ldaps://ldap.example.com:636",
		"ldap.user_base_dn":             "ou=people,dc=example,dc=com",
		"ldap.allow_insecure_transport": "on",
	})

	cfg, err := s.loadLDAPConfig()
	if err != nil {
		t.Fatalf("load ldap config: %v", err)
	}
	if !cfg.Enabled || !cfg.AllowInsecure {
		t.Fatalf("expected boolean config variants to parse true: %+v", cfg)
	}
	if cfg.UserSearchFilter != "(uid={username})" ||
		cfg.GroupSearchFilter != "(|(member={dn})(memberUid={username}))" ||
		cfg.UsernameAttribute != "uid" ||
		cfg.EmailAttribute != "mail" ||
		cfg.ExternalIDAttribute != "entryUUID" ||
		cfg.GroupNameAttribute != "cn" {
		t.Fatalf("unexpected default LDAP config: %+v", cfg)
	}
}

func TestLoadLDAPAuthenticatorReturnsNilWhenDisabled(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)

	authenticator, err := s.loadLDAPAuthenticator()
	if err != nil {
		t.Fatalf("load ldap authenticator: %v", err)
	}
	if authenticator != nil {
		t.Fatalf("expected disabled LDAP config to return nil authenticator")
	}
}

func TestLoadLDAPAuthenticatorRejectsIncompleteConfig(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)
	setLDAPConfigForTest(t, s, map[string]string{
		"ldap.enabled": "true",
		"ldap.url":     "ldaps://ldap.example.com:636",
	})

	_, err := s.loadLDAPAuthenticator()
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected incomplete config error, got %v", err)
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

func TestDecryptConfigSecretRejectsInvalidCiphertext(t *testing.T) {
	_, err := decryptConfigSecret("test-secret", "enc:not-hex")
	if err == nil || !strings.Contains(err.Error(), "invalid encrypted secret format") {
		t.Fatalf("expected encrypted secret format error, got %v", err)
	}

	_, err = decryptConfigSecret("test-secret", "enc:abcd")
	if err == nil {
		t.Fatal("expected encrypted secret decrypt error")
	}
}

func TestValidateLDAPTransport(t *testing.T) {
	tests := []struct {
		name    string
		cfg     LDAPConfig
		wantErr string
	}{
		{name: "ldaps", cfg: LDAPConfig{URL: "ldaps://ldap.example.com:636"}},
		{name: "starttls", cfg: LDAPConfig{URL: "ldap://ldap.example.com:389", StartTLS: true}},
		{name: "allow insecure", cfg: LDAPConfig{URL: "ldap://ldap.example.com:389", AllowInsecure: true}},
		{name: "plain ldap rejected", cfg: LDAPConfig{URL: "ldap://ldap.example.com:389"}, wantErr: "secure transport required"},
		{name: "unsupported scheme", cfg: LDAPConfig{URL: "http://ldap.example.com"}, wantErr: "unsupported LDAP URL scheme"},
		{name: "bad url", cfg: LDAPConfig{URL: "://bad"}, wantErr: "invalid LDAP URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLDAPTransport(tt.cfg)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("validateLDAPTransport: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
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

func TestRenderLDAPFilterEscapesValues(t *testing.T) {
	filter := renderLDAPFilter("(&(uid={username})(member={dn}))", map[string]string{
		"username": `alice*)(uid=*)`,
		"dn":       `uid=alice,ou=people,dc=example,dc=com`,
	})
	if strings.Contains(filter, "alice*") || strings.Contains(filter, "(uid=*)") {
		t.Fatalf("expected escaped filter, got %s", filter)
	}
	if !strings.Contains(filter, `alice\2a\29\28uid=\2a\29`) {
		t.Fatalf("expected escaped username in filter, got %s", filter)
	}
}

func TestLDAPBindAuthenticatorRejectsBlankCredentials(t *testing.T) {
	a := ldapBindAuthenticator{cfg: LDAPConfig{URL: "ldaps://ldap.example.com:636"}}
	for _, tc := range []struct {
		username string
		password string
	}{
		{username: "", password: "secret"},
		{username: "alice", password: ""},
		{username: "   ", password: "secret"},
	} {
		_, err := a.Authenticate(context.Background(), tc.username, tc.password)
		if err == nil || !strings.Contains(err.Error(), errInvalidLDAPCredentials) {
			t.Fatalf("expected invalid credentials for %+v, got %v", tc, err)
		}
	}
}

func TestMapLDAPRolePriorityAndNoMatch(t *testing.T) {
	role, ok := mapLDAPRole([]string{"aerodocs-viewers", "aerodocs-admins"})
	if !ok || role != model.RoleAdmin {
		t.Fatalf("expected admin role priority, got role=%s ok=%v", role, ok)
	}
	role, ok = mapLDAPRole([]string{"ipausers"})
	if ok || role != "" {
		t.Fatalf("expected no role match, got role=%s ok=%v", role, ok)
	}
	if hasAnyLDAPGroup([]string{"ipausers"}, []string{"aerodocs-admins"}) {
		t.Fatal("did not expect unrelated group match")
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
