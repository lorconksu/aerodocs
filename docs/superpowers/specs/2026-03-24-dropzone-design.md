# AeroDocs Sub-project 6: Dropzone — Design Spec

**Date:** 2026-03-24
**Status:** Approved
**Scope:** Admin-only file upload from Hub to agent's `/tmp/aerodocs-dropzone/` staging directory via chunked gRPC streaming, with drag-and-drop UI and upload progress.

## 1. Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Access control | Admin only | File uploads are a privileged operation. No per-path permissions needed — files always go to the fixed dropzone. |
| Destination | /tmp/aerodocs-dropzone/ | Fixed staging directory. Cleaned on reboot. Moving files to final location requires server access (SSH) — intentional 2FA for file transfer. |
| File size limit | 100 MB | Covers configs, scripts, binaries, small archives. Chunked transfer keeps memory low. |
| Upload target | One server at a time | Upload from server detail page. Simple and clear. |
| Chunk size | 64 KB | Good balance between throughput and memory. ~1,600 chunks for a 100MB file. |
| UI location | Server detail page | "Dropzone" section below the file viewer, admin only. |

## 2. Proto Changes

Update existing stubs in `agent.proto`:

```protobuf
message FileUploadRequest {
    string request_id = 1;
    string filename = 2;
    bytes chunk = 3;
    bool done = 4;
}

message FileUploadAck {
    string request_id = 1;
    bool success = 2;
    string error = 3;
}
```

Existing oneof entries remain:
- `AgentMessage.file_upload_ack = 12`
- `HubMessage.file_upload_request = 12`

## 3. Upload Flow

1. Admin drags file onto dropzone UI (or clicks to browse)
2. Frontend sends `POST /api/servers/{id}/upload` with multipart form data
3. Hub validates: admin role, file size < 100MB, agent connected
4. Hub reads the file in 64KB chunks and sends `FileUploadRequest` messages to agent via gRPC stream
   - First chunk: `filename` set, `done = false`
   - Middle chunks: `filename` empty, `done = false`
   - Last chunk: `done = true`
5. Agent receives chunks, writes to `/tmp/aerodocs-dropzone/{filename}`
6. Agent sends `FileUploadAck` when `done = true` (success or error)
7. Hub waits for ack via PendingRequests (with 30s timeout for large files)
8. Hub returns success/failure JSON to frontend
9. Frontend shows success notification and refreshes the dropped files list

## 4. Agent Side

### Dropzone package (`agent/internal/dropzone/dropzone.go`)

- Manages active uploads: `map[requestID]*os.File`
- `HandleChunk(requestID, filename string, data []byte, done bool) *pb.FileUploadAck`
  - First chunk (filename non-empty): creates `/tmp/aerodocs-dropzone/{filename}`, stores file handle
  - Subsequent chunks: appends data to the open file
  - Done=true: closes file, returns success ack
  - On error: closes file, removes partial file, returns error ack
- Creates `/tmp/aerodocs-dropzone/` on startup if it doesn't exist
- Sanitizes filename: strips path components, rejects empty or `.` names

### Agent dispatcher updates

- Handle `FileUploadRequest` in `handleMessage`: call `dropzone.HandleChunk`, send ack on done

## 5. Hub Side

### HTTP endpoint

```
POST /api/servers/{id}/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data

Form field: "file" (the uploaded file)

Response 200: { "filename": "nginx.conf", "size": 1234 }
Response 400: { "error": "no file provided" }
Response 413: { "error": "file too large (max 100MB)" }
Response 502: { "error": "agent not connected" }
Response 504: { "error": "upload timeout" }
```

### Dropzone listing

```
GET /api/servers/{id}/dropzone
Authorization: Bearer <token>

Response 200: { "files": [{ "name": "nginx.conf", "path": "/tmp/aerodocs-dropzone/nginx.conf", "is_dir": false, "size": 1234, "readable": true }] }
```

Uses existing `FileListRequest` to list `/tmp/aerodocs-dropzone/` — no new endpoint logic needed, just a convenience wrapper.

### Audit event

```go
AuditFileUploaded = "file.uploaded"
```

Logged on successful upload with server_id as target, filename as detail.

## 6. Frontend

### Dropzone section (server detail page, admin only)

- Collapsible "Dropzone" section below the file viewer (similar to "Manage File Access")
- Drag-and-drop area with visual feedback (dashed border, highlight on drag)
- Click-to-browse fallback
- Upload progress bar showing percentage (chunks sent / total chunks)
- "Dropped Files" list below, refreshed after each upload
  - Shows filename, size, timestamp
  - Uses GET /api/servers/{id}/dropzone to fetch the list
- Error display for failed uploads

## 7. Error Handling

| Scenario | Behavior |
|----------|----------|
| File > 100MB | HTTP 413 at Hub, before sending to agent |
| Agent offline | HTTP 502 |
| Agent disk full | Agent sends error ack, Hub returns 500 |
| Upload interrupted (network) | Partial file left in dropzone, can retry |
| Filename collision | Overwrite existing file (simpler than versioning) |
| Invalid filename | Hub rejects (empty, contains path separators) |
