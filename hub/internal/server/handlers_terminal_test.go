package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

const testTerminalSessionsSuffix = "/terminal/sessions"

func createTerminalSessionForTest(t *testing.T, s *Server, token, serverID string) string {
	t.Helper()

	body := mustJSON(t, map[string]interface{}{
		"cols": 120,
		"rows": 36,
	})
	req := httptest.NewRequest("POST", testServersPrefix+serverID+testTerminalSessionsSuffix, body)
	req.Header.Set("Authorization", testBearerPrefix+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.TerminalSessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode terminal session response: %v", err)
	}
	if resp.SessionID == "" {
		t.Fatal("expected terminal session id")
	}
	return resp.SessionID
}

func TestHandleCreateTerminalSession(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	sessionID := createTerminalSessionForTest(t, s, adminToken, serverID)

	conn := s.connMgr.GetConn(serverID)
	stream, ok := conn.Stream.(*mockGRPCStream)
	if !ok {
		t.Fatal("expected mockGRPCStream")
	}
	if len(stream.sent) == 0 {
		t.Fatal("expected a terminal open request to be sent to the agent")
	}
	openReq := stream.sent[len(stream.sent)-1].GetTerminalOpenRequest()
	if openReq == nil {
		t.Fatal("expected last message to be a terminal open request")
	}
	if openReq.SessionId != sessionID {
		t.Fatalf("expected session id %s, got %s", sessionID, openReq.SessionId)
	}
}

func TestHandleCreateTerminalSession_ViewerDenied(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)
	viewerToken := createViewerAndGetToken(t, s, adminToken)

	req := httptest.NewRequest("POST", testServersPrefix+serverID+testTerminalSessionsSuffix, mustJSON(t, map[string]interface{}{"cols": 80, "rows": 24}))
	req.Header.Set("Authorization", testBearerPrefix+viewerToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateTerminalSession_APITokenDenied(t *testing.T) {
	s, _, serverID := testServerWithAgent(t)
	user, err := s.store.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("get admin user: %v", err)
	}
	raw, hash, prefix, err := auth.GenerateAPIToken()
	if err != nil {
		t.Fatalf("generate api token: %v", err)
	}
	if err := s.store.CreateAPIToken(&model.APIToken{
		ID:          uuid.NewString(),
		UserID:      user.ID,
		Name:        "automation",
		TokenHash:   hash,
		TokenPrefix: prefix,
	}); err != nil {
		t.Fatalf("create api token: %v", err)
	}

	req := httptest.NewRequest("POST", testServersPrefix+serverID+testTerminalSessionsSuffix, mustJSON(t, map[string]interface{}{"cols": 80, "rows": 24}))
	req.Header.Set("Authorization", testBearerPrefix+raw)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTerminalInputAndResize(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)
	sessionID := createTerminalSessionForTest(t, s, adminToken, serverID)

	inputBody := mustJSON(t, map[string]string{"data": "pwd\n"})
	inputReq := httptest.NewRequest("POST", testServersPrefix+serverID+testTerminalSessionsSuffix+"/"+sessionID+"/input", inputBody)
	inputReq.Header.Set("Authorization", testBearerPrefix+adminToken)
	inputRec := httptest.NewRecorder()
	s.routes().ServeHTTP(inputRec, inputReq)
	if inputRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", inputRec.Code, inputRec.Body.String())
	}

	resizeBody := mustJSON(t, map[string]int{"cols": 132, "rows": 40})
	resizeReq := httptest.NewRequest("POST", testServersPrefix+serverID+testTerminalSessionsSuffix+"/"+sessionID+"/resize", resizeBody)
	resizeReq.Header.Set("Authorization", testBearerPrefix+adminToken)
	resizeRec := httptest.NewRecorder()
	s.routes().ServeHTTP(resizeRec, resizeReq)
	if resizeRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", resizeRec.Code, resizeRec.Body.String())
	}

	conn := s.connMgr.GetConn(serverID)
	stream := conn.Stream.(*mockGRPCStream)
	foundInput := false
	foundResize := false
	for _, msg := range stream.sent {
		if payload := msg.GetTerminalInput(); payload != nil && payload.SessionId == sessionID && string(payload.Data) == "pwd\n" {
			foundInput = true
		}
		if payload := msg.GetTerminalResize(); payload != nil && payload.SessionId == sessionID && payload.Cols == 132 && payload.Rows == 40 {
			foundResize = true
		}
	}
	if !foundInput {
		t.Fatal("expected terminal input message")
	}
	if !foundResize {
		t.Fatal("expected terminal resize message")
	}
}

func TestHandleTerminalStream(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)
	sessionID := createTerminalSessionForTest(t, s, adminToken, serverID)

	if !s.terminalSessions.End(serverID, sessionID, 0, "") {
		t.Fatal("expected terminal session end")
	}

	req := httptest.NewRequest("GET", testServersPrefix+serverID+testTerminalSessionsSuffix+"/"+sessionID+"/stream", nil)
	req.Header.Set("Authorization", testBearerPrefix+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "bW9jayQg") {
		t.Fatalf("expected base64 terminal output in stream, got %q", body)
	}
	if !strings.Contains(body, "event: exit") {
		t.Fatalf("expected exit event in stream, got %q", body)
	}
}

func TestHandleCloseTerminalSession(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)
	sessionID := createTerminalSessionForTest(t, s, adminToken, serverID)

	req := httptest.NewRequest("DELETE", testServersPrefix+serverID+testTerminalSessionsSuffix+"/"+sessionID, bytes.NewReader(nil))
	req.Header.Set("Authorization", testBearerPrefix+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := s.terminalSessions.Get(serverID, sessionID); ok {
		t.Fatal("expected terminal session to be removed")
	}

	conn := s.connMgr.GetConn(serverID)
	stream := conn.Stream.(*mockGRPCStream)
	foundClose := false
	for _, msg := range stream.sent {
		if payload := msg.GetTerminalClose(); payload != nil && payload.SessionId == sessionID {
			foundClose = true
		}
	}
	if !foundClose {
		t.Fatal("expected terminal close message")
	}
}
