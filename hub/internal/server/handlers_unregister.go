package server

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleUnregisterServer(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")

	// Try to send unregister command to agent if it's connected
	if s.connMgr != nil {
		conn := s.connMgr.GetConn(serverID)
		if conn != nil {
			requestID := uuid.NewString()
			ch := s.pending.Register(serverID, requestID)
			defer s.pending.Remove(serverID, requestID)

			err := s.connMgr.SendToAgent(serverID, &pb.HubMessage{
				Payload: &pb.HubMessage_UnregisterRequest{
					UnregisterRequest: &pb.UnregisterRequest{
						RequestId: requestID,
					},
				},
			})
			if err == nil {
				// Wait for ack (10s timeout) — don't fail if timeout, still delete from DB
				select {
				case <-ch:
					// Got ack, agent is cleaning up
				case <-time.After(10 * time.Second):
					// Timeout — proceed with DB deletion anyway
				}
			}
		}
	}

	// Delete server from database (cascades permissions)
	if err := s.store.DeleteServer(serverID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete server")
		return
	}

	// Audit log
	userID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID:        uuid.NewString(),
		UserID:    &userID,
		Action:    model.AuditServerUnregistered,
		Target:    &serverID,
		IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "unregistered"})
}

// handleSelfUnregister is a public endpoint (no auth) called by the agent binary
// during a re-install. The server_id in the path acts as proof of installation.
// It only deletes the server from the DB — no agent cleanup (the install script handles that).
func (s *Server) handleSelfUnregister(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")

	// Verify the server actually exists
	srv, err := s.store.GetServerByID(serverID)
	if err != nil {
		// Already gone — that's fine
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Verify request comes from the registered agent IP
	reqIP := clientIP(r)
	if srv.IPAddress == nil || *srv.IPAddress != reqIP {
		respondError(w, http.StatusForbidden, "unauthorized")
		return
	}

	// Disconnect agent if connected
	if s.connMgr != nil {
		s.connMgr.Unregister(serverID)
	}

	// Delete from database
	if err := s.store.DeleteServer(serverID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete server")
		return
	}

	// Audit log (no user — this is agent-initiated)
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID:        uuid.NewString(),
		Action:    model.AuditServerUnregistered,
		Target:    &serverID,
		IPAddress: &ip,
	})

	w.WriteHeader(http.StatusNoContent)
}
