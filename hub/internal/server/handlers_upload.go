package server

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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

	filename := filepath.Base(header.Filename)
	if filename == "" || filename == "." || filename == "/" {
		respondError(w, http.StatusBadRequest, "filename is required")
		return
	}

	// Check agent connected
	if _, ok := s.requireAgent(w, r); !ok {
		return
	}

	// Generate request ID and register for ack
	requestID := uuid.NewString()
	ch := s.pending.Register(serverID, requestID)
	defer s.pending.Remove(serverID, requestID)

	totalSize, streamErr := s.streamFileToAgent(serverID, requestID, filename, file)
	if streamErr != nil {
		respondError(w, streamErr.statusCode, streamErr.message)
		return
	}

	// Wait for ack
	select {
	case msg := <-ch:
		ack, ok := msg.(*pb.FileUploadAck)
		if !ok {
			respondError(w, http.StatusInternalServerError, errUnexpectedResponse)
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

		respondJSON(w, http.StatusOK, model.UploadFileResponse{
			Filename: filename,
			Size:     totalSize,
		})
	case <-time.After(uploadTimeout):
		respondError(w, http.StatusGatewayTimeout, "upload timeout")
	}
}

type uploadStreamError struct {
	statusCode int
	message    string
}

// streamFileToAgent sends the file contents to the agent in chunks, followed by a final "done"
// message. It returns the total number of bytes sent, or an uploadStreamError on failure.
func (s *Server) streamFileToAgent(serverID, requestID, filename string, file io.Reader) (int64, *uploadStreamError) {
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
				return 0, &uploadStreamError{http.StatusBadGateway, "failed to send to agent"}
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, &uploadStreamError{http.StatusInternalServerError, "failed to read file"}
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
		return 0, &uploadStreamError{http.StatusBadGateway, "failed to send to agent"}
	}

	return totalSize, nil
}

func (s *Server) handleDeleteDropzoneFile(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		respondError(w, http.StatusBadRequest, "filename is required")
		return
	}

	filename = filepath.Base(filename)
	if filename == "." || filename == "/" {
		respondError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	serverID, ok := s.requireAgent(w, r)
	if !ok {
		return
	}

	path := "/tmp/aerodocs-dropzone/" + filename
	raw := s.sendAgentRequest(w, serverID, func(requestID string) *pb.HubMessage {
		return &pb.HubMessage{
			Payload: &pb.HubMessage_FileDeleteRequest{
				FileDeleteRequest: &pb.FileDeleteRequest{
					RequestId: requestID,
					Path:      path,
				},
			},
		}
	}, agentTimeoutDuration)
	if raw == nil {
		return
	}

	resp, ok := raw.(*pb.FileDeleteResponse)
	if !ok {
		respondError(w, http.StatusInternalServerError, errUnexpectedResponse)
		return
	}
	if !resp.Success {
		respondError(w, http.StatusInternalServerError, resp.Error)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListDropzone(w http.ResponseWriter, r *http.Request) {
	serverID, ok := s.requireAgent(w, r)
	if !ok {
		return
	}

	raw := s.sendAgentRequest(w, serverID, func(requestID string) *pb.HubMessage {
		return &pb.HubMessage{
			Payload: &pb.HubMessage_FileListRequest{
				FileListRequest: &pb.FileListRequest{
					RequestId: requestID,
					Path:      "/tmp/aerodocs-dropzone",
				},
			},
		}
	}, agentTimeoutDuration)
	if raw == nil {
		return
	}

	resp, ok := raw.(*pb.FileListResponse)
	if !ok {
		respondError(w, http.StatusInternalServerError, errUnexpectedResponse)
		return
	}
	if resp.Error != "" {
		// Dropzone dir may not exist yet — return empty list
		respondJSON(w, http.StatusOK, model.FileListResult{Files: []*pb.FileNode{}})
		return
	}
	respondJSON(w, http.StatusOK, model.FileListResult{Files: resp.Files})
}
