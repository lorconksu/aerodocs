package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/agent/agentclient"
)

func TestFileUploadThroughGRPC(t *testing.T) {
	h := StartHarness(t)
	token := h.SetupAdmin(t)
	serverID, regToken := h.CreateServer(t, token, "upload-server")

	// Start agent
	agentClient := agentclient.New(agentclient.Config{
		HubAddr:      h.GRPCAddr,
		ServerID:     "",
		Token:        regToken,
		Hostname:     "test-host",
		IPAddress:    "10.0.0.1",
		OS:           "linux",
		AgentVersion: "0.0.0-test",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go agentClient.Run(ctx)

	// Wait for agent to connect
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, id := range h.ConnMgr.ActiveServerIDs() {
			if id == serverID {
				goto connected
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("agent did not connect within 5s")
connected:

	// Build multipart form with a test file
	uploadFilename := "integration-upload.txt"
	fileContent := []byte("upload integration test content")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", uploadFilename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(fileContent); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	// POST upload
	uploadURL := fmt.Sprintf("http://%s/api/servers/%s/upload", h.HTTPAddr, serverID)
	req, err := http.NewRequest("POST", uploadURL, &buf)
	if err != nil {
		t.Fatalf("create upload request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	uploadResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(uploadResp.Body)
		t.Fatalf("upload: status=%d body=%s", uploadResp.StatusCode, body)
	}

	var uploadResult struct {
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&uploadResult); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if uploadResult.Filename != uploadFilename {
		t.Fatalf("filename mismatch: got %q, want %q", uploadResult.Filename, uploadFilename)
	}
	if uploadResult.Size != int64(len(fileContent)) {
		t.Fatalf("size mismatch: got %d, want %d", uploadResult.Size, len(fileContent))
	}
	t.Logf("upload verified: filename=%s size=%d", uploadResult.Filename, uploadResult.Size)

	// List dropzone — verify the uploaded file appears
	dropzoneResp := h.HTTPGet(t, fmt.Sprintf("/api/servers/%s/dropzone", serverID), token)
	defer dropzoneResp.Body.Close()

	if dropzoneResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dropzoneResp.Body)
		t.Fatalf("list dropzone: status=%d body=%s", dropzoneResp.StatusCode, body)
	}

	var dropzoneResult struct {
		Files []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(dropzoneResp.Body).Decode(&dropzoneResult); err != nil {
		t.Fatalf("decode dropzone response: %v", err)
	}

	found := false
	for _, f := range dropzoneResult.Files {
		if f.Name == uploadFilename {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in dropzone, got %+v", uploadFilename, dropzoneResult.Files)
	}
	t.Logf("dropzone list verified: %q present", uploadFilename)

	// Delete the file from dropzone
	deleteURL := fmt.Sprintf("http://%s/api/servers/%s/dropzone?filename=%s", h.HTTPAddr, serverID, uploadFilename)
	delReq, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		t.Fatalf("create delete request: %v", err)
	}
	delReq.Header.Set("Authorization", "Bearer "+token)

	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(delResp.Body)
		t.Fatalf("delete dropzone file: status=%d body=%s", delResp.StatusCode, body)
	}
	t.Logf("dropzone delete verified: %q removed (204 No Content)", uploadFilename)
}
