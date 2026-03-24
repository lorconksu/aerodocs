package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/model"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

func (s *Server) handleTailLog(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")
	path := r.URL.Query().Get("path")
	grep := r.URL.Query().Get("grep")

	if path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	// Validate path
	if err := validateRequestPath(path); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check permissions
	userID := UserIDFromContext(r.Context())
	role := UserRoleFromContext(r.Context())
	if role != "admin" {
		allowed, err := s.isPathAllowed(userID, serverID, path)
		if err != nil || !allowed {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	// Check agent connected
	if s.connMgr == nil {
		respondError(w, http.StatusBadGateway, "agent not connected")
		return
	}
	conn := s.connMgr.GetConn(serverID)
	if conn == nil {
		respondError(w, http.StatusBadGateway, "agent not connected")
		return
	}

	// Check Flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Extend write deadline for long-lived SSE connection
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{}) // No deadline

	// Register log session
	requestID := uuid.NewString()
	ch := s.logSessions.Register(requestID)

	// Send LogStreamRequest to agent
	err := s.connMgr.SendToAgent(serverID, &pb.HubMessage{
		Payload: &pb.HubMessage_LogStreamRequest{
			LogStreamRequest: &pb.LogStreamRequest{
				RequestId: requestID,
				Path:      path,
				Grep:      grep,
			},
		},
	})
	if err != nil {
		s.logSessions.Remove(requestID)
		respondError(w, http.StatusBadGateway, "failed to send request to agent")
		return
	}

	// Audit log
	ip := clientIP(r)
	detail := path
	if grep != "" {
		detail += " grep=" + grep
	}
	s.store.LogAudit(model.AuditEntry{
		ID:        uuid.NewString(),
		UserID:    &userID,
		Action:    model.AuditLogTailStarted,
		Target:    &serverID,
		Detail:    &detail,
		IPAddress: &ip,
	})

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Cleanup on exit
	defer func() {
		s.logSessions.Remove(requestID)
		// Send stop to agent
		_ = s.connMgr.SendToAgent(serverID, &pb.HubMessage{
			Payload: &pb.HubMessage_LogStreamStop{
				LogStreamStop: &pb.LogStreamStop{
					RequestId: requestID,
				},
			},
		})
	}()

	// Stream loop
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-ch:
			if !ok {
				return // Channel closed
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			fmt.Fprintf(w, "data: %s\n\n", encoded)
			flusher.Flush()
		}
	}
}
