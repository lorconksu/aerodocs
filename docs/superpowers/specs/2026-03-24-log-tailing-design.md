# AeroDocs Sub-project 5: Log Tailing — Design Spec

**Date:** 2026-03-24
**Status:** Approved
**Scope:** Live log streaming through the agent gRPC stream with server-side grep filtering, SSE endpoint for browser, and integrated UI on the server detail page.

## 1. Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Browser transport | Server-Sent Events (SSE) | One-directional server→client streaming. Simpler than WebSocket, native browser support via EventSource, auto-reconnect built in. |
| UI location | "Live Tail" button in file viewer | When viewing a text file, a toggle switches from static view to live streaming. Reuses existing file tree navigation — no separate logs page needed. |
| Stop mechanism | LogStreamStop message | Hub sends LogStreamStop to agent when SSE client disconnects. Agent cancels the tailing goroutine. |
| Tailing method | Poll with os.Seek | Agent polls file for new data every 500ms using seek-to-last-offset. Simple, works on all filesystems without fsnotify dependency. |
| Access control | Same as file tree | Uses existing permissions table — user must have access to the path to tail it. |
| Grep filtering | Agent-side | Agent filters lines before sending, reducing network traffic. Hub passes grep pattern from query param. |

## 2. Proto Changes

Update existing stubs in `agent.proto`:

```protobuf
// Update existing stubs with request_id
message LogStreamRequest {
    string request_id = 1;
    string path = 2;
    int64 offset = 3;
    string grep = 4;
}

message LogStreamChunk {
    string request_id = 1;
    bytes data = 2;
    int64 offset = 3;
}

// New message for stopping a log stream
message LogStreamStop {
    string request_id = 1;
}
```

Add to HubMessage oneof:
```protobuf
LogStreamStop log_stream_stop = 14;
```

Existing oneof entries remain:
- `AgentMessage.log_stream_chunk = 11`
- `HubMessage.log_stream_request = 11`

## 3. Agent Side

### Log tailer (`agent/internal/logtailer/tailer.go`)

- `StartTail(path string, grep string, offset int64, sendCh chan<- *pb.AgentMessage, requestID string, stop <-chan struct{})`
- Opens file, seeks to offset (or end if offset=0)
- Polls every 500ms for new data
- If grep is set, filters lines containing the pattern (case-insensitive)
- Sends LogStreamChunk messages with updated offset
- Stops when `stop` channel is closed
- Handles file truncation (log rotation) by resetting to beginning

### Agent dispatcher updates

- Track active tail sessions: `map[string]chan struct{}` (requestID → stop channel)
- On LogStreamRequest: start new tail goroutine, store stop channel
- On LogStreamStop: close the stop channel, remove from map
- On disconnect: close all active stop channels

## 4. Hub Side

### Log sessions (`hub/internal/grpcserver/logsessions.go`)

- `LogSessions` struct: maps `requestID → chan []byte`
- `Register(requestID) chan []byte` — creates buffered channel
- `Deliver(requestID, data)` — sends data to channel
- `Remove(requestID)` — cleans up

### gRPC handler updates

- Add LogStreamChunk case to receive loop → delivers to LogSessions
- LogSessions stored on Handler (like PendingRequests)

### SSE endpoint

```
GET /api/servers/{id}/logs/tail?path=/var/log/syslog&grep=error
Accept: text/event-stream
Authorization: Bearer <token>
```

- Validates path permissions (same as file browsing)
- Generates request_id, registers in LogSessions
- Sends LogStreamRequest to agent via ConnManager
- Streams SSE events: `data: <base64-encoded chunk>\n\n`
- On client disconnect (context.Done): sends LogStreamStop to agent, cleans up LogSessions
- Response headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`

## 5. Frontend

### File viewer integration

When viewing a text file, add a "Live Tail" toggle button next to the Refresh button:
- Off (default): static file view (existing behavior)
- On: switches to streaming mode

### Streaming mode UI

- Content area becomes a terminal-like view with monospace text
- New lines append at bottom, auto-scrolls
- "Pause" button to stop auto-scroll (still receives data, just doesn't scroll)
- Grep filter input at the top
- Line count indicator
- "Stop" button to end the tail session
- Reconnects automatically if SSE connection drops

### Implementation

- Use `EventSource` API for SSE
- Decode base64 chunks and append to a text buffer
- Cap buffer at 10,000 lines (discard oldest)
- Auto-scroll uses `scrollIntoView` on a bottom sentinel div

## 6. Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent offline | SSE returns 502, frontend shows error |
| File not found | Agent sends chunk with empty data and error, Hub closes SSE |
| Permission denied | HTTP 403 before starting SSE |
| Agent disconnect during tail | SSE connection closes, frontend shows reconnect option |
| File rotated/truncated | Agent detects smaller file size, resets to beginning |
