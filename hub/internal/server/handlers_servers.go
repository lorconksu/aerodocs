package server

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := model.ServerFilter{}
	filter.Limit, filter.Offset = parsePagination(q, 50)

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

	respondJSON(w, http.StatusOK, model.ServerListResponse{
		Servers: servers,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	})
}

func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var req model.CreateServerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
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

	// Build install command using the request's host and configured gRPC address
	scheme := "https"
	if s.isDev {
		scheme = "http"
	}
	host := r.Host
	baseURL := fmt.Sprintf("%s://%s", scheme, host)

	// gRPC target: in production, use the hostname (Traefik proxies gRPC on 443).
	// In dev mode, use the raw gRPC listen address.
	grpcTarget := s.grpcAddr
	if !s.isDev {
		// Strip port from Host header to get just the hostname
		hostname := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			hostname = h
		}
		grpcTarget = hostname + ":443"
	}

	installCmd := fmt.Sprintf(
		"curl -sSL %s/install.sh | sudo bash -s -- --token %s --hub %s --url %s",
		baseURL, rawToken, grpcTarget, baseURL,
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
		respondError(w, http.StatusNotFound, errServerNotFound)
		return
	}

	// Viewers must have permission on this specific server
	role := UserRoleFromContext(r.Context())
	if role != "admin" {
		userID := UserIDFromContext(r.Context())
		paths, err := s.store.GetUserPathsForServer(userID, id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to check permissions")
			return
		}
		if len(paths) == 0 {
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
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "server name is required")
		return
	}

	if err := s.store.UpdateServer(id, req.Name, req.Labels); err != nil {
		respondError(w, http.StatusNotFound, errServerNotFound)
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
		respondError(w, http.StatusNotFound, errServerNotFound)
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
		respondError(w, http.StatusBadRequest, errInvalidRequestBody)
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

	respondJSON(w, http.StatusOK, model.BatchDeleteResponse{
		Status:  "deleted",
		Deleted: len(req.IDs),
	})
}

func (s *Server) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/install.sh")
}

func (s *Server) handleAgentBinary(w http.ResponseWriter, r *http.Request) {
	osName := r.PathValue("os")
	arch := r.PathValue("arch")
	if osName != "linux" || (arch != "amd64" && arch != "arm64") {
		respondError(w, http.StatusNotFound, "unsupported platform")
		return
	}
	filename := fmt.Sprintf("aerodocs-agent-%s-%s", osName, arch)
	binaryPath := filepath.Join(s.agentBinDir, filename)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		respondError(w, http.StatusNotFound, "agent binary not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	http.ServeFile(w, r, binaryPath)
}

