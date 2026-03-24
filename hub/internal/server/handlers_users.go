package server

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"users": users})
}

func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if targetID == "" {
		respondError(w, http.StatusBadRequest, "missing user id")
		return
	}

	var req model.UpdateRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != model.RoleAdmin && req.Role != model.RoleViewer {
		respondError(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
		return
	}

	adminID := UserIDFromContext(r.Context())
	if adminID == targetID {
		respondError(w, http.StatusBadRequest, "cannot change your own role")
		return
	}

	if err := s.store.UpdateUserRole(targetID, req.Role); err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	user, err := s.store.GetUserByID(targetID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated user")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditUserRoleUpdated, Target: &targetID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if targetID == "" {
		respondError(w, http.StatusBadRequest, "missing user id")
		return
	}

	adminID := UserIDFromContext(r.Context())
	if adminID == targetID {
		respondError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	if err := s.store.DeleteUser(targetID); err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditUserDeleted, Target: &targetID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "user deleted"})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req model.CreateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateUsername(req.Username); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Role != model.RoleAdmin && req.Role != model.RoleViewer {
		respondError(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
		return
	}

	tempPassword := auth.GenerateTemporaryPassword()
	hash, err := auth.HashPassword(tempPassword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         req.Role,
	}

	if err := s.store.CreateUser(user); err != nil {
		respondError(w, http.StatusConflict, "user already exists")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditUserCreated, Target: &user.ID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusCreated, model.CreateUserResponse{
		User:              *user,
		TemporaryPassword: tempPassword,
	})
}
