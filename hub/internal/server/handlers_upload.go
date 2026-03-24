package server

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

const (
	maxUploadSize   = 100 * 1024 * 1024 // 100MB
	uploadChunkSize = 64 * 1024         // 64KB
	uploadTimeout   = 30 * time.Second
)

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+1024) // small buffer for form overhead

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		respondError(w, http.StatusRequestEntityTooLarge, "file too large (max 100MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "no file provided")
		return
	}
	defer file.Close()

	filename := header.Filename
	if filename == "" {
		respondError(w, http.StatusBadRequest, "filename is required")
		return
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

	// Generate request ID and register for ack
	requestID := uuid.NewString()
	ch := s.pending.Register(requestID)
	defer s.pending.Remove(requestID)

	// Stream file in chunks
	buf := make([]byte, uploadChunkSize)
	isFirst := true
	totalSize := int64(0)

	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			totalSize += int64(n)
			fname := ""
			if isFirst {
				fname = filename
				isFirst = false
			}

			sendErr := s.connMgr.SendToAgent(serverID, &pb.HubMessage{
				Payload: &pb.HubMessage_FileUploadRequest{
					FileUploadRequest: &pb.FileUploadRequest{
						RequestId: requestID,
						Filename:  fname,
						Chunk:     buf[:n],
						Done:      false,
					},
				},
			})
			if sendErr != nil {
				respondError(w, http.StatusBadGateway, "failed to send to agent")
				return
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			respondError(w, http.StatusInternalServerError, "failed to read file")
			return
		}
	}

	// Send final "done" message
	sendErr := s.connMgr.SendToAgent(serverID, &pb.HubMessage{
		Payload: &pb.HubMessage_FileUploadRequest{
			FileUploadRequest: &pb.FileUploadRequest{
				RequestId: requestID,
				Done:      true,
			},
		},
	})
	if sendErr != nil {
		respondError(w, http.StatusBadGateway, "failed to send to agent")
		return
	}

	// Wait for ack
	select {
	case msg := <-ch:
		ack, ok := msg.(*pb.FileUploadAck)
		if !ok {
			respondError(w, http.StatusInternalServerError, "unexpected response type")
			return
		}
		if !ack.Success {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("upload failed: %s", ack.Error))
			return
		}

		// Audit log
		userID := UserIDFromContext(r.Context())
		ip := clientIP(r)
		detail := filename
		s.store.LogAudit(model.AuditEntry{
			ID:        uuid.NewString(),
			UserID:    &userID,
			Action:    model.AuditFileUploaded,
			Target:    &serverID,
			Detail:    &detail,
			IPAddress: &ip,
		})

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"filename": filename,
			"size":     totalSize,
		})
	case <-time.After(uploadTimeout):
		respondError(w, http.StatusGatewayTimeout, "upload timeout")
	}
}

func (s *Server) handleListDropzone(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")

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

	requestID := uuid.NewString()
	ch := s.pending.Register(requestID)
	defer s.pending.Remove(requestID)

	err := s.connMgr.SendToAgent(serverID, &pb.HubMessage{
		Payload: &pb.HubMessage_FileListRequest{
			FileListRequest: &pb.FileListRequest{
				RequestId: requestID,
				Path:      "/tmp/aerodocs-dropzone",
			},
		},
	})
	if err != nil {
		respondError(w, http.StatusBadGateway, "agent not connected")
		return
	}

	select {
	case msg := <-ch:
		resp, ok := msg.(*pb.FileListResponse)
		if !ok {
			respondError(w, http.StatusInternalServerError, "unexpected response type")
			return
		}
		if resp.Error != "" {
			// Dropzone dir may not exist yet — return empty list
			respondJSON(w, http.StatusOK, map[string]interface{}{"files": []interface{}{}})
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{"files": resp.Files})
	case <-time.After(10 * time.Second):
		respondError(w, http.StatusGatewayTimeout, "agent timeout")
	}
}
