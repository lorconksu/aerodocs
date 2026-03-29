package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

// TestGetSMTPConfig_Default verifies that an empty store returns a default config.
func TestGetSMTPConfig_Default(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/settings/smtp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var cfg model.SMTPConfig
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if cfg.Host != "" {
		t.Errorf("expected empty host, got %q", cfg.Host)
	}
	if cfg.Password != "" {
		t.Errorf("expected empty password, got %q", cfg.Password)
	}
	// Default port is 587
	if cfg.Port != 587 {
		t.Errorf("expected default port 587, got %d", cfg.Port)
	}
}

// TestUpdateAndGetSMTPConfig verifies that a PUT saves and GET returns the config with masked password.
func TestUpdateAndGetSMTPConfig(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// PUT new config
	putBody := mustJSON(t, model.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     465,
		Username: "user@example.com",
		Password: "supersecret",
		From:     "noreply@example.com",
		TLS:      true,
		Enabled:  true,
	})
	putReq := httptest.NewRequest("PUT", "/api/settings/smtp", putBody)
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	s.routes().ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT, got %d: %s", putRec.Code, putRec.Body.String())
	}

	var putResp map[string]string
	json.NewDecoder(putRec.Body).Decode(&putResp)
	if putResp["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", putResp["status"])
	}

	// GET the config back
	getReq := httptest.NewRequest("GET", "/api/settings/smtp", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	s.routes().ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var cfg model.SMTPConfig
	json.NewDecoder(getRec.Body).Decode(&cfg)

	if cfg.Host != "smtp.example.com" {
		t.Errorf("expected host smtp.example.com, got %q", cfg.Host)
	}
	if cfg.Port != 465 {
		t.Errorf("expected port 465, got %d", cfg.Port)
	}
	if cfg.Username != "user@example.com" {
		t.Errorf("expected username user@example.com, got %q", cfg.Username)
	}
	// Password must be masked
	if cfg.Password != "********" {
		t.Errorf("expected masked password, got %q", cfg.Password)
	}
	if cfg.From != "noreply@example.com" {
		t.Errorf("expected from noreply@example.com, got %q", cfg.From)
	}
	if !cfg.TLS {
		t.Error("expected TLS=true")
	}
	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
}

// TestUpdateSMTPConfig_MaskedPasswordPreservesExisting verifies that sending "********"
// as the password does not overwrite the stored password.
func TestUpdateSMTPConfig_MaskedPasswordPreservesExisting(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// First, store a real password
	firstPut := mustJSON(t, model.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Password: "original-secret",
		From:     "from@example.com",
	})
	req1 := httptest.NewRequest("PUT", "/api/settings/smtp", firstPut)
	req1.Header.Set("Authorization", "Bearer "+token)
	s.routes().ServeHTTP(httptest.NewRecorder(), req1)

	// Now update with masked password — original should be preserved
	secondPut := mustJSON(t, model.SMTPConfig{
		Host:     "smtp.updated.com",
		Port:     587,
		Password: "********",
		From:     "from@example.com",
	})
	req2 := httptest.NewRequest("PUT", "/api/settings/smtp", secondPut)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// Read password directly from store to confirm it wasn't overwritten
	storedPassword, err := s.store.GetConfig("smtp_password")
	if err != nil {
		t.Fatalf("get config smtp_password: %v", err)
	}
	if storedPassword != "original-secret" {
		t.Errorf("expected preserved password 'original-secret', got %q", storedPassword)
	}

	// Host should have been updated
	storedHost, _ := s.store.GetConfig("smtp_host")
	if storedHost != "smtp.updated.com" {
		t.Errorf("expected updated host smtp.updated.com, got %q", storedHost)
	}
}

// TestGetNotificationPreferences_Default verifies that all event types are returned with defaults.
func TestGetNotificationPreferences_Default(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/notifications/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	prefs, ok := resp["preferences"]
	if !ok {
		t.Fatal("expected 'preferences' key in response")
	}

	prefsSlice, ok := prefs.([]interface{})
	if !ok {
		t.Fatal("expected preferences to be an array")
	}

	if len(prefsSlice) != len(model.AllNotifyEvents) {
		t.Errorf("expected %d preferences, got %d", len(model.AllNotifyEvents), len(prefsSlice))
	}
}

// TestUpdateNotificationPreferences verifies that preferences can be toggled.
func TestUpdateNotificationPreferences(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Update a preference
	updateBody := mustJSON(t, model.NotificationPreferencesRequest{
		Preferences: []model.NotificationPrefUpdate{
			{EventType: model.NotifyAgentOnline, Enabled: true},
			{EventType: model.NotifyAgentOffline, Enabled: false},
		},
	})
	putReq := httptest.NewRequest("PUT", "/api/notifications/preferences", updateBody)
	putReq.Header.Set("Authorization", "Bearer "+token)
	putRec := httptest.NewRecorder()
	s.routes().ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT, got %d: %s", putRec.Code, putRec.Body.String())
	}

	var putResp map[string]string
	json.NewDecoder(putRec.Body).Decode(&putResp)
	if putResp["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", putResp)
	}

	// Verify by GETting preferences
	getReq := httptest.NewRequest("GET", "/api/notifications/preferences", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	s.routes().ServeHTTP(getRec, getReq)

	var resp map[string]interface{}
	json.NewDecoder(getRec.Body).Decode(&resp)

	prefsSlice := resp["preferences"].([]interface{})
	prefMap := make(map[string]bool)
	for _, p := range prefsSlice {
		entry := p.(map[string]interface{})
		prefMap[entry["event_type"].(string)] = entry["enabled"].(bool)
	}

	if !prefMap[model.NotifyAgentOnline] {
		t.Errorf("expected agent.online to be enabled")
	}
	if prefMap[model.NotifyAgentOffline] {
		t.Errorf("expected agent.offline to be disabled")
	}
}

// TestListNotificationLog_Empty verifies that an empty log returns an empty entries list.
func TestListNotificationLog_Empty(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/notifications/log", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if _, ok := resp["entries"]; !ok {
		t.Fatal("expected 'entries' key in response")
	}
	if _, ok := resp["total"]; !ok {
		t.Fatal("expected 'total' key in response")
	}

	total := int(resp["total"].(float64))
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}

	entries := resp["entries"].([]interface{})
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestSMTPConfig_NonAdminForbidden verifies that non-admin users cannot access admin-only endpoints.
func TestSMTPConfig_NonAdminForbidden(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	viewerToken := createViewerAndGetToken(t, s, adminToken)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/settings/smtp"},
		{"PUT", "/api/settings/smtp"},
		{"POST", "/api/settings/smtp/test"},
		{"GET", "/api/notifications/log"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rec := httptest.NewRecorder()
		s.routes().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected 403 for viewer, got %d", ep.method, ep.path, rec.Code)
		}
	}
}
