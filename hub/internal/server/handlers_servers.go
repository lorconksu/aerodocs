package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := model.ServerFilter{
		Limit:  50,
		Offset: 0,
	}

	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}
	if v := q.Get("status"); v != "" {
		filter.Status = &v
	}
	if v := q.Get("search"); v != "" {
		filter.Search = &v
	}

	role := UserRoleFromContext(r.Context())
	userID := UserIDFromContext(r.Context())

	var servers []model.Server
	var total int
	var err error

	if role == "admin" {
		servers, total, err = s.store.ListServers(filter)
	} else {
		servers, total, err = s.store.ListServersForUser(userID, filter)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"servers": servers,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}

func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var req model.CreateServerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "server name is required")
		return
	}

	if req.Labels == "" {
		req.Labels = "{}"
	}

	// Generate raw registration token
	rawToken := uuid.NewString()

	// Hash it for storage
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := fmt.Sprintf("%x", hash)

	expiresAt := time.Now().Add(1 * time.Hour).UTC().Format("2006-01-02 15:04:05")

	srv := &model.Server{
		ID:                uuid.NewString(),
		Name:              req.Name,
		Status:            "pending",
		RegistrationToken: &tokenHash,
		TokenExpiresAt:    &expiresAt,
		Labels:            req.Labels,
	}

	if err := s.store.CreateServer(srv); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create server")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditServerCreated, Target: &srv.ID, IPAddress: &ip,
	})

	installCmd := fmt.Sprintf(
		"curl -sSL https://hub.example.com/install.sh | sudo bash -s -- --token %s --hub https://hub.example.com",
		rawToken,
	)

	respondJSON(w, http.StatusCreated, model.CreateServerResponse{
		Server:            *srv,
		RegistrationToken: rawToken,
		InstallCommand:    installCmd,
	})
}

func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	srv, err := s.store.GetServerByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "server not found")
		return
	}

	// Viewers must have permission
	role := UserRoleFromContext(r.Context())
	if role != "admin" {
		userID := UserIDFromContext(r.Context())
		servers, _, err := s.store.ListServersForUser(userID, model.ServerFilter{Limit: 1})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to check permissions")
			return
		}
		found := false
		for _, permitted := range servers {
			if permitted.ID == id {
				found = true
				break
			}
		}
		if !found {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	respondJSON(w, http.StatusOK, srv)
}

func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Name   string `json:"name"`
		Labels string `json:"labels"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "server name is required")
		return
	}

	if err := s.store.UpdateServer(id, req.Name, req.Labels); err != nil {
		respondError(w, http.StatusNotFound, "server not found")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditServerUpdated, Target: &id, IPAddress: &ip,
	})

	srv, _ := s.store.GetServerByID(id)
	respondJSON(w, http.StatusOK, srv)
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.store.DeleteServer(id); err != nil {
		respondError(w, http.StatusNotFound, "server not found")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditServerDeleted, Target: &id, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleBatchDeleteServers(w http.ResponseWriter, r *http.Request) {
	var req model.BatchDeleteRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "ids list cannot be empty")
		return
	}

	if err := s.store.DeleteServers(req.IDs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete servers")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	detail := fmt.Sprintf("deleted %d servers", len(req.IDs))
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditServerBatchDeleted, Detail: &detail, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"deleted": len(req.IDs),
	})
}

func (s *Server) handleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	// Hash the raw token to look up the server
	hash := sha256.Sum256([]byte(req.Token))
	tokenHash := fmt.Sprintf("%x", hash)

	srv, err := s.store.GetServerByToken(tokenHash)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or expired registration token")
		return
	}

	// Check if token is expired
	if srv.TokenExpiresAt != nil {
		expiresAt, err := time.Parse("2006-01-02 15:04:05", *srv.TokenExpiresAt)
		if err == nil && time.Now().UTC().After(expiresAt) {
			respondError(w, http.StatusUnauthorized, "invalid or expired registration token")
			return
		}
	}

	// Activate the server
	if err := s.store.ActivateServer(srv.ID, req.Hostname, req.IPAddress, req.OS, req.AgentVersion); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to activate server")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID:     uuid.NewString(),
		Action: model.AuditServerRegistered, Target: &srv.ID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{
		"server_id": srv.ID,
		"status":    "online",
	})
}
