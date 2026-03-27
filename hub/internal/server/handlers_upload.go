package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/model"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

const (
	maxUploadSize   = 100 * 1024 * 1024 // 100MB
	uploadChunkSize = 64 * 1024         // 64KB
	uploadTimeout   = 30 * time.Second
)

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")

	// Stream the multipart body directly instead of buffering via ParseMultipartForm.
	// We use r.MultipartReader() which returns a streaming reader.
	reader, filename, parseErr := parseMultipartFileStream(r)
	if parseErr != nil {
		respondError(w, parseErr.statusCode, parseErr.message)
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

	totalSize, streamErr := s.streamFileToAgent(serverID, requestID, filename, reader)
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

// parseMultipartFileStream returns a streaming reader for the "file" part of a
// multipart upload, plus the sanitised filename. It uses r.MultipartReader()
// so the body is never buffered in memory or to a temp file.
func parseMultipartFileStream(r *http.Request) (io.Reader, string, *uploadStreamError) {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return nil, "", &uploadStreamError{http.StatusBadRequest, "missing Content-Type header"}
	}

	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return nil, "", &uploadStreamError{http.StatusBadRequest, "expected multipart/form-data"}
	}

	boundary := params["boundary"]
	if boundary == "" {
		return nil, "", &uploadStreamError{http.StatusBadRequest, "missing multipart boundary"}
	}

	mr := multipart.NewReader(r.Body, boundary)

	// Iterate parts until we find the "file" field.
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, "", &uploadStreamError{http.StatusBadRequest, "no file provided"}
		}
		if err != nil {
			return nil, "", &uploadStreamError{http.StatusBadRequest, "invalid multipart body"}
		}

		if part.FormName() != "file" {
			// Skip non-file parts (drain so the reader advances).
			continue
		}

		filename := filepath.Base(part.FileName())
		if filename == "" || filename == "." || filename == "/" {
			return nil, "", &uploadStreamError{http.StatusBadRequest, "filename is required"}
		}

		return part, filename, nil
	}
}

// sendChunkToAgent sends a single file chunk to the agent.
func (s *Server) sendChunkToAgent(serverID, requestID, filename string, data []byte) *uploadStreamError {
	sendErr := s.connMgr.SendToAgent(serverID, &pb.HubMessage{
		Payload: &pb.HubMessage_FileUploadRequest{
			FileUploadRequest: &pb.FileUploadRequest{
				RequestId: requestID,
				Filename:  filename,
				Chunk:     data,
				Done:      false,
			},
		},
	})
	if sendErr != nil {
		return &uploadStreamError{http.StatusBadGateway, "failed to send to agent"}
	}
	return nil
}

// streamFileToAgent sends the file contents to the agent in chunks, followed by a final "done"
// message. It enforces the maxUploadSize limit by counting bytes as they stream through.
// It returns the total number of bytes sent, or an uploadStreamError on failure.
func (s *Server) streamFileToAgent(serverID, requestID, filename string, file io.Reader) (int64, *uploadStreamError) {
	buf := make([]byte, uploadChunkSize)
	isFirst := true
	totalSize := int64(0)

	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			totalSize += int64(n)
			if totalSize > maxUploadSize {
				return 0, &uploadStreamError{http.StatusRequestEntityTooLarge, "file too large (max 100MB)"}
			}

			fname := ""
			if isFirst {
				fname = filename
				isFirst = false
			}

			if chunkErr := s.sendChunkToAgent(serverID, requestID, fname, buf[:n]); chunkErr != nil {
				return 0, chunkErr
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
