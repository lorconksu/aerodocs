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
			ch := s.pending.Register(requestID)
			defer s.pending.Remove(requestID)

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
