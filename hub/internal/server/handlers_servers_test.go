package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestListServers_Admin(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create servers directly in the store
	s.store.CreateServer(&model.Server{ID: "s1", Name: "alpha", Status: "online", Labels: "{}"})
	s.store.CreateServer(&model.Server{ID: "s2", Name: "beta", Status: "pending", Labels: "{}"})

	req := httptest.NewRequest("GET", "/api/servers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	total := int(resp["total"].(float64))
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
}

func TestListServers_FilterByStatus(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "alpha", Status: "online", Labels: "{}"})
	s.store.CreateServer(&model.Server{ID: "s2", Name: "beta", Status: "pending", Labels: "{}"})

	req := httptest.NewRequest("GET", "/api/servers?status=online", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	total := int(resp["total"].(float64))
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
}

func TestCreateServer(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.CreateServerRequest{
		Name: "web-prod-1",
	})

	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.CreateServerResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Server.Name != "web-prod-1" {
		t.Fatalf("expected name 'web-prod-1', got '%s'", resp.Server.Name)
	}
	if resp.Server.Status != "pending" {
		t.Fatalf("expected status 'pending', got '%s'", resp.Server.Status)
	}
	if resp.RegistrationToken == "" {
		t.Fatal("expected registration_token in response")
	}
	if resp.InstallCommand == "" {
		t.Fatal("expected install_command in response")
	}
}

func TestCreateServer_EmptyName(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.CreateServerRequest{Name: ""})

	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetServer(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "test-srv", Status: "online", Labels: "{}"})

	req := httptest.NewRequest("GET", "/api/servers/s1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var srv model.Server
	json.NewDecoder(rec.Body).Decode(&srv)
	if srv.Name != "test-srv" {
		t.Fatalf("expected 'test-srv', got '%s'", srv.Name)
	}
}

func TestGetServer_NotFound(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/servers/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateServer(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "old-name", Status: "online", Labels: "{}"})

	body, _ := json.Marshal(map[string]string{"name": "new-name", "labels": `{"env":"staging"}`})

	req := httptest.NewRequest("PUT", "/api/servers/s1", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteServer(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "doomed", Status: "online", Labels: "{}"})

	req := httptest.NewRequest("DELETE", "/api/servers/s1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it's gone
	_, err := s.store.GetServerByID("s1")
	if err == nil {
		t.Fatal("expected server to be deleted")
	}
}

func TestBatchDeleteServers(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "a", Status: "online", Labels: "{}"})
	s.store.CreateServer(&model.Server{ID: "s2", Name: "b", Status: "online", Labels: "{}"})
	s.store.CreateServer(&model.Server{ID: "s3", Name: "c", Status: "online", Labels: "{}"})

	body, _ := json.Marshal(model.BatchDeleteRequest{IDs: []string{"s1", "s3"}})

	req := httptest.NewRequest("POST", "/api/servers/batch-delete", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify only s2 remains
	servers, total, _ := s.store.ListServers(model.ServerFilter{Limit: 50})
	if total != 1 {
		t.Fatalf("expected 1 remaining, got %d", total)
	}
	if servers[0].ID != "s2" {
		t.Fatalf("expected s2 to survive, got %s", servers[0].ID)
	}
}

func TestBatchDeleteServers_EmptyList(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.BatchDeleteRequest{IDs: []string{}})

	req := httptest.NewRequest("POST", "/api/servers/batch-delete", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegisterAgent_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create a server via the API to get a raw token
	createBody, _ := json.Marshal(model.CreateServerRequest{Name: "agent-test"})
	createReq := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateServerResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)
	rawToken := createResp.RegistrationToken

	// Register agent (no auth required)
	regBody, _ := json.Marshal(model.RegisterAgentRequest{
		Token:        rawToken,
		Hostname:     "agent-host",
		IPAddress:    "10.10.1.50",
		OS:           "Ubuntu 24.04",
		AgentVersion: "0.1.0",
	})
	regReq := httptest.NewRequest("POST", "/api/servers/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	if regRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", regRec.Code, regRec.Body.String())
	}

	var regResp map[string]interface{}
	json.NewDecoder(regRec.Body).Decode(&regResp)
	if regResp["status"] != "online" {
		t.Fatalf("expected status 'online', got '%v'", regResp["status"])
	}
}

func TestRegisterAgent_InvalidToken(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.RegisterAgentRequest{
		Token: "totally-fake-token", Hostname: "h", IPAddress: "1.1.1.1", OS: "Linux", AgentVersion: "0.1",
	})
	req := httptest.NewRequest("POST", "/api/servers/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRegisterAgent_AlreadyUsed(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create and register
	createBody, _ := json.Marshal(model.CreateServerRequest{Name: "double-reg"})
	createReq := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateServerResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)
	rawToken := createResp.RegistrationToken

	// First registration
	regBody, _ := json.Marshal(model.RegisterAgentRequest{
		Token: rawToken, Hostname: "h", IPAddress: "1.1.1.1", OS: "Linux", AgentVersion: "0.1",
	})
	regReq := httptest.NewRequest("POST", "/api/servers/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	if regRec.Code != http.StatusOK {
		t.Fatalf("first register: expected 200, got %d", regRec.Code)
	}

	// Second registration with same token
	regReq2 := httptest.NewRequest("POST", "/api/servers/register", bytes.NewReader(regBody))
	regRec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec2, regReq2)

	if regRec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for reused token, got %d: %s", regRec2.Code, regRec2.Body.String())
	}
}
