# Log Tailing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build live log streaming through the agent gRPC stream with server-side grep filtering, SSE endpoint for browser delivery, and an integrated Live Tail UI on the server detail page's file viewer.

**Architecture:** When a user clicks "Live Tail" on a text file, the frontend opens an SSE connection to a new Hub endpoint. The Hub generates a request_id, sends a `LogStreamRequest` through the existing gRPC bidirectional stream to the agent, and forwards incoming `LogStreamChunk` messages as SSE events. The agent polls the file every 500ms using seek-to-last-offset, optionally filtering lines by a grep pattern before sending. On disconnect, the Hub sends `LogStreamStop` to cancel the agent's tailing goroutine. The same path-permission system from sub-project 4 gates access.

**Tech Stack:** Go (hub + agent), Protocol Buffers, React, TypeScript, EventSource API (SSE)

**Spec:** `docs/superpowers/specs/2026-03-24-log-tailing-design.md`

---

## File Map

### Proto (`proto/`)

| File | Action | Responsibility |
|------|--------|---------------|
| `proto/aerodocs/v1/agent.proto` | Modify | Update LogStream stubs with request_id, add LogStreamStop message, add to HubMessage oneof |
| `proto/aerodocs/v1/agent.pb.go` | Regenerate | Generated message types |
| `proto/aerodocs/v1/agent_grpc.pb.go` | Regenerate | Generated gRPC service interfaces |

### Go Agent (`agent/`)

| File | Action | Responsibility |
|------|--------|---------------|
| `agent/internal/logtailer/tailer.go` | Create | StartTail: poll file, grep filter, send chunks via sendCh |
| `agent/internal/logtailer/tailer_test.go` | Create | Tests for tail, grep, rotation |
| `agent/internal/client/client.go` | Modify | Add tailSessions map, handle LogStreamRequest/Stop, cleanup on disconnect |

### Go Hub Backend (`hub/`)

| File | Action | Responsibility |
|------|--------|---------------|
| `hub/internal/grpcserver/logsessions.go` | Create | LogSessions: Register/Deliver/Remove for streaming channels |
| `hub/internal/grpcserver/logsessions_test.go` | Create | Tests for LogSessions |
| `hub/internal/grpcserver/handler.go` | Modify | Add LogStreamChunk dispatch in receive loop |
| `hub/internal/grpcserver/server.go` | Modify | Add logSessions field to Server and Config, pass to Handler |
| `hub/internal/model/audit.go` | Modify | Add AuditLogTailStarted constant |
| `hub/internal/server/server.go` | Modify | Add logSessions field to Server struct and Config |
| `hub/internal/server/handlers_logs.go` | Create | SSE endpoint: handleTailLog |
| `hub/internal/server/handlers_logs_test.go` | Create | Tests for SSE handler setup |
| `hub/internal/server/router.go` | Modify | Add log tail route |
| `hub/cmd/aerodocs/main.go` | Modify | Create LogSessions, pass to gRPC and HTTP servers |

### Frontend (`web/`)

| File | Action | Responsibility |
|------|--------|---------------|
| `web/src/pages/server-detail.tsx` | Modify | Add Live Tail toggle, SSE streaming, terminal-like log view |

### Build

| File | Action | Responsibility |
|------|--------|---------------|
| `Makefile` | Verify | Existing proto/build/test targets already cover this |

---

## Task 1: Proto Changes

**Files:** `proto/aerodocs/v1/agent.proto`
**Depends on:** Nothing
**Estimated time:** 5 minutes

### 1a: Update LogStream stubs and add LogStreamStop

- [ ] **Step 1: Update LogStreamRequest, LogStreamChunk stubs and add LogStreamStop**

Edit `proto/aerodocs/v1/agent.proto` -- replace the stubs block at the bottom:

Replace:
```protobuf
// Stubs — sub-projects 5-6
message LogStreamRequest { string path = 1; int64 offset = 2; string grep = 3; }
message LogStreamChunk { bytes data = 1; int64 offset = 2; }
message FileUploadRequest { string path = 1; bytes chunk = 2; bool done = 3; }
message FileUploadAck { bool success = 1; string error = 2; }
```

With:
```protobuf
// Log tailing (sub-project 5)
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

message LogStreamStop {
  string request_id = 1;
}

// Stubs — sub-project 6
message FileUploadRequest { string path = 1; bytes chunk = 2; bool done = 3; }
message FileUploadAck { bool success = 1; string error = 2; }
```

- [ ] **Step 2: Add LogStreamStop to HubMessage oneof**

In the `HubMessage` oneof block, add after `FileReadRequest file_read_request = 13;`:

```protobuf
    LogStreamStop log_stream_stop = 14;
```

The final `HubMessage` oneof should be:
```protobuf
message HubMessage {
  oneof payload {
    HeartbeatAck heartbeat_ack = 1;
    RegisterAck register_ack = 2;
    // Stubs for sub-projects 4-6
    FileListRequest file_list_request = 10;
    LogStreamRequest log_stream_request = 11;
    FileUploadRequest file_upload_request = 12;
    FileReadRequest file_read_request = 13;
    LogStreamStop log_stream_stop = 14;
  }
}
```

The `AgentMessage` oneof stays the same -- `LogStreamChunk` is already at field 11.

### 1b: Regenerate proto

- [ ] **Step 3: Run protoc**

```bash
cd /home/wyiu/personal/aerodocs && make proto
```

Expected: `proto/aerodocs/v1/agent.pb.go` and `proto/aerodocs/v1/agent_grpc.pb.go` are regenerated with `LogStreamRequest` (with request_id), `LogStreamChunk` (with request_id), `LogStreamStop`, and the new `HubMessage_LogStreamStop` oneof variant.

- [ ] **Step 4: Verify proto compiles**

```bash
cd /home/wyiu/personal/aerodocs/proto && go build ./...
```

Expected: clean compile, no errors.

- [ ] **Step 5: Verify hub still compiles**

```bash
cd /home/wyiu/personal/aerodocs/hub && go build ./...
```

Expected: clean compile. The existing handler.go does not reference LogStreamChunk fields, so no breakage.

- [ ] **Step 6: Verify agent still compiles**

```bash
cd /home/wyiu/personal/aerodocs/agent && go build ./...
```

Expected: clean compile.

### Commit

```
git add proto/aerodocs/v1/agent.proto proto/aerodocs/v1/agent.pb.go proto/aerodocs/v1/agent_grpc.pb.go
git commit -m "proto: update LogStream messages with request_id and add LogStreamStop for sub-project 5"
```

---

## Task 2: Agent Log Tailer Package (TDD)

**Files:** `agent/internal/logtailer/tailer.go`, `agent/internal/logtailer/tailer_test.go`
**Depends on:** Task 1
**Estimated time:** 20 minutes

### 2a: Write tests first

- [ ] **Step 1: Create test file**

Create `agent/internal/logtailer/tailer_test.go`:

```go
package logtailer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

func collectChunks(ch <-chan *pb.AgentMessage, timeout time.Duration) []*pb.LogStreamChunk {
	var chunks []*pb.LogStreamChunk
	deadline := time.After(timeout)
	for {
		select {
		case msg := <-ch:
			if c := msg.GetLogStreamChunk(); c != nil {
				chunks = append(chunks, c)
			}
		case <-deadline:
			return chunks
		}
	}
}

func TestStartTail_NewLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Create file with initial content (tailer seeks to end, so this is skipped)
	if err := os.WriteFile(path, []byte("old line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sendCh := make(chan *pb.AgentMessage, 64)
	stop := make(chan struct{})

	go StartTail(path, "", 0, sendCh, "req-1", stop)

	// Wait for tailer to start and seek to end
	time.Sleep(200 * time.Millisecond)

	// Append new lines
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("new line 1\n")
	f.WriteString("new line 2\n")
	f.Close()

	chunks := collectChunks(sendCh, 2*time.Second)
	close(stop)

	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk, got 0")
	}

	// Combine all chunk data
	var combined []byte
	for _, c := range chunks {
		combined = append(combined, c.Data...)
		if c.RequestId != "req-1" {
			t.Errorf("expected request_id=req-1, got %s", c.RequestId)
		}
	}

	text := string(combined)
	if !strings.Contains(text, "new line 1") || !strings.Contains(text, "new line 2") {
		t.Errorf("expected new lines in output, got: %s", text)
	}
	if strings.Contains(text, "old line") {
		t.Error("should not contain old content that was present before tail started")
	}
}

func TestStartTail_GrepFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	sendCh := make(chan *pb.AgentMessage, 64)
	stop := make(chan struct{})

	go StartTail(path, "error", 0, sendCh, "req-2", stop)

	time.Sleep(200 * time.Millisecond)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("info: all good\n")
	f.WriteString("ERROR: something broke\n")
	f.WriteString("debug: trace data\n")
	f.WriteString("warn: error in subsystem\n")
	f.Close()

	chunks := collectChunks(sendCh, 2*time.Second)
	close(stop)

	var combined []byte
	for _, c := range chunks {
		combined = append(combined, c.Data...)
	}

	text := string(combined)
	if !strings.Contains(text, "ERROR: something broke") {
		t.Error("expected ERROR line to pass grep filter")
	}
	if !strings.Contains(text, "error in subsystem") {
		t.Error("expected 'error in subsystem' line to pass grep filter (case-insensitive)")
	}
	if strings.Contains(text, "info: all good") {
		t.Error("'info: all good' should not pass grep filter")
	}
	if strings.Contains(text, "debug: trace data") {
		t.Error("'debug: trace data' should not pass grep filter")
	}
}

func TestStartTail_FileRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Create initial file with some content
	if err := os.WriteFile(path, []byte("initial content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sendCh := make(chan *pb.AgentMessage, 64)
	stop := make(chan struct{})

	go StartTail(path, "", 0, sendCh, "req-3", stop)
	time.Sleep(200 * time.Millisecond)

	// Append so tailer tracks the offset
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("before rotation\n")
	f.Close()

	// Wait for the chunk to arrive
	collectChunks(sendCh, 1*time.Second)

	// Simulate log rotation: truncate file and write new content
	if err := os.WriteFile(path, []byte("after rotation\n"), 0644); err != nil {
		t.Fatal(err)
	}

	chunks := collectChunks(sendCh, 2*time.Second)
	close(stop)

	var combined []byte
	for _, c := range chunks {
		combined = append(combined, c.Data...)
	}

	text := string(combined)
	if !strings.Contains(text, "after rotation") {
		t.Errorf("expected rotated content, got: %s", text)
	}
}

func TestStartTail_StopChannel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	sendCh := make(chan *pb.AgentMessage, 64)
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		StartTail(path, "", 0, sendCh, "req-4", stop)
		close(done)
	}()

	// Close stop channel — tailer should exit
	time.Sleep(200 * time.Millisecond)
	close(stop)

	select {
	case <-done:
		// good — tailer exited
	case <-time.After(2 * time.Second):
		t.Fatal("tailer did not stop within timeout")
	}
}

func TestStartTail_WithOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sendCh := make(chan *pb.AgentMessage, 64)
	stop := make(chan struct{})

	// Start at offset 9 (after "line one\n")
	go StartTail(path, "", 9, sendCh, "req-5", stop)

	// Append new data after the tailer starts
	time.Sleep(200 * time.Millisecond)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("line four\n")
	f.Close()

	chunks := collectChunks(sendCh, 2*time.Second)
	close(stop)

	var combined []byte
	for _, c := range chunks {
		combined = append(combined, c.Data...)
	}
	text := string(combined)

	// Should include existing content from offset 9 onwards plus new line
	if !strings.Contains(text, "line two") {
		t.Errorf("expected 'line two' from offset read, got: %s", text)
	}
	if !strings.Contains(text, "line four") {
		t.Errorf("expected 'line four' from appended data, got: %s", text)
	}
	if strings.Contains(text, "line one") {
		t.Error("should not contain 'line one' — started past it")
	}
}

func TestStartTail_FileNotFound(t *testing.T) {
	sendCh := make(chan *pb.AgentMessage, 64)
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		StartTail("/nonexistent/path/file.log", "", 0, sendCh, "req-6", stop)
		close(done)
	}()

	select {
	case <-done:
		// good — exited cleanly
	case <-time.After(2 * time.Second):
		close(stop)
		t.Fatal("tailer did not exit on missing file")
	}
}
```

- [ ] **Step 2: Verify tests fail (no implementation yet)**

```bash
cd /home/wyiu/personal/aerodocs/agent && go test ./internal/logtailer/ 2>&1 | head -5
```

Expected: compilation error (package does not exist yet).

### 2b: Implement the tailer

- [ ] **Step 3: Create the logtailer package**

Create `agent/internal/logtailer/tailer.go`:

```go
package logtailer

import (
	"bufio"
	"io"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

const pollInterval = 500 * time.Millisecond

// StartTail opens a file and streams new content to sendCh until the stop
// channel is closed. If offset > 0, it starts reading from that position
// (and sends any existing content from that offset first). If offset == 0,
// it seeks to the end and only sends new data. If grep is non-empty, only
// lines containing that substring (case-insensitive) are sent.
func StartTail(path string, grep string, offset int64, sendCh chan<- *pb.AgentMessage, requestID string, stop <-chan struct{}) {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("logtailer: cannot open %s: %v", path, err)
		return
	}
	defer f.Close()

	grepLower := strings.ToLower(grep)

	// Determine starting position
	if offset > 0 {
		// Verify the file is at least that large
		info, err := f.Stat()
		if err != nil {
			log.Printf("logtailer: stat %s: %v", path, err)
			return
		}
		if offset > info.Size() {
			offset = 0
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			log.Printf("logtailer: seek %s: %v", path, err)
			return
		}
	} else {
		// Seek to end — only tail new content
		endPos, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			log.Printf("logtailer: seek end %s: %v", path, err)
			return
		}
		offset = endPos
	}

	// Read any existing content from the current position (for offset > 0 case)
	if buf, n, newOffset := readNew(f, offset); n > 0 {
		filtered := filterLines(buf[:n], grepLower)
		if len(filtered) > 0 {
			sendChunk(sendCh, requestID, filtered, newOffset)
		}
		offset = newOffset
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// Check for file rotation (truncation)
			info, err := os.Stat(path)
			if err != nil {
				// File might be temporarily unavailable during rotation
				continue
			}
			if info.Size() < offset {
				// File was truncated — reopen and start from beginning
				f.Close()
				f, err = os.Open(path)
				if err != nil {
					log.Printf("logtailer: reopen %s after rotation: %v", path, err)
					return
				}
				offset = 0
			}

			if buf, n, newOffset := readNew(f, offset); n > 0 {
				filtered := filterLines(buf[:n], grepLower)
				if len(filtered) > 0 {
					sendChunk(sendCh, requestID, filtered, newOffset)
				}
				offset = newOffset
			}
		}
	}
}

// readNew reads any new data from the file starting at the given offset.
// Returns the buffer, bytes read, and new offset.
func readNew(f *os.File, offset int64) ([]byte, int, int64) {
	info, err := f.Stat()
	if err != nil || info.Size() <= offset {
		return nil, 0, offset
	}

	toRead := info.Size() - offset
	// Cap read size to 64KB per poll to avoid huge allocations
	if toRead > 65536 {
		toRead = 65536
	}

	buf := make([]byte, toRead)
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, 0, offset
	}

	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, 0, offset
	}
	return buf, n, offset + int64(n)
}

// filterLines splits data into lines and returns only those matching the grep
// pattern. If grepLower is empty, all data is returned unchanged.
func filterLines(data []byte, grepLower string) []byte {
	if grepLower == "" {
		return data
	}

	var result []byte
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), grepLower) {
			result = append(result, []byte(line+"\n")...)
		}
	}
	return result
}

func sendChunk(sendCh chan<- *pb.AgentMessage, requestID string, data []byte, offset int64) {
	sendCh <- &pb.AgentMessage{
		Payload: &pb.AgentMessage_LogStreamChunk{
			LogStreamChunk: &pb.LogStreamChunk{
				RequestId: requestID,
				Data:      data,
				Offset:    offset,
			},
		},
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/wyiu/personal/aerodocs/agent && go test ./internal/logtailer/ -v -count=1
```

Expected: all 6 tests pass.

- [ ] **Step 5: Verify agent compiles**

```bash
cd /home/wyiu/personal/aerodocs/agent && go build ./...
```

Expected: clean compile.

### Commit

```
git add agent/internal/logtailer/tailer.go agent/internal/logtailer/tailer_test.go
git commit -m "agent: add logtailer package with poll-based file tailing and grep filter"
```

---

## Task 3: Agent Dispatcher Updates

**Files:** `agent/internal/client/client.go`
**Depends on:** Task 1, Task 2
**Estimated time:** 10 minutes

- [ ] **Step 1: Add logtailer import and tailSessions field to Client**

In `agent/internal/client/client.go`, add the import:

```go
"github.com/wyiu/aerodocs/agent/internal/logtailer"
```

Add a new field to the `Client` struct after `maxBackoff`:

```go
	tailMu       sync.Mutex
	tailSessions map[string]chan struct{}
```

Add the `sync` import as well.

- [ ] **Step 2: Initialize tailSessions in New()**

In the `New()` function, add to the returned struct:

```go
		tailSessions: make(map[string]chan struct{}),
```

- [ ] **Step 3: Add LogStreamRequest handler to handleMessage**

In the `handleMessage` switch, add before the `default` case:

```go
	case *pb.HubMessage_LogStreamRequest:
		req := p.LogStreamRequest
		stopCh := make(chan struct{})
		c.tailMu.Lock()
		// Close any existing session with same ID
		if old, ok := c.tailSessions[req.RequestId]; ok {
			close(old)
		}
		c.tailSessions[req.RequestId] = stopCh
		c.tailMu.Unlock()

		go func() {
			logtailer.StartTail(req.Path, req.Grep, req.Offset, sendCh, req.RequestId, stopCh)
			// Clean up when tail finishes
			c.tailMu.Lock()
			delete(c.tailSessions, req.RequestId)
			c.tailMu.Unlock()
		}()

	case *pb.HubMessage_LogStreamStop:
		c.tailMu.Lock()
		if stopCh, ok := c.tailSessions[p.LogStreamStop.RequestId]; ok {
			close(stopCh)
			delete(c.tailSessions, p.LogStreamStop.RequestId)
		}
		c.tailMu.Unlock()
```

- [ ] **Step 4: Add cleanup of tail sessions on disconnect**

In `connectAndStream`, in the defer block after `defer close(hbStop)`, add:

```go
	defer c.stopAllTails()
```

Add the helper method on Client:

```go
func (c *Client) stopAllTails() {
	c.tailMu.Lock()
	defer c.tailMu.Unlock()
	for id, stopCh := range c.tailSessions {
		close(stopCh)
		delete(c.tailSessions, id)
	}
}
```

- [ ] **Step 5: Verify agent compiles**

```bash
cd /home/wyiu/personal/aerodocs/agent && go build ./...
```

Expected: clean compile.

- [ ] **Step 6: Run all agent tests**

```bash
cd /home/wyiu/personal/aerodocs/agent && go test ./... -v -count=1
```

Expected: all tests pass.

### Commit

```
git add agent/internal/client/client.go
git commit -m "agent: handle LogStreamRequest/Stop with tail session lifecycle"
```

---

## Task 4: Hub LogSessions (TDD)

**Files:** `hub/internal/grpcserver/logsessions.go`, `hub/internal/grpcserver/logsessions_test.go`
**Depends on:** Task 1
**Estimated time:** 10 minutes

### 4a: Write tests first

- [ ] **Step 1: Create test file**

Create `hub/internal/grpcserver/logsessions_test.go`:

```go
package grpcserver

import (
	"testing"
	"time"
)

func TestLogSessions_RegisterAndDeliver(t *testing.T) {
	ls := NewLogSessions()
	ch := ls.Register("req-1")

	if ch == nil {
		t.Fatal("Register returned nil channel")
	}

	// Deliver data
	ok := ls.Deliver("req-1", []byte("hello world"))
	if !ok {
		t.Error("Deliver returned false for registered session")
	}

	// Read from channel
	select {
	case data := <-ch:
		if string(data) != "hello world" {
			t.Errorf("expected 'hello world', got '%s'", string(data))
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for data")
	}
}

func TestLogSessions_DeliverUnknown(t *testing.T) {
	ls := NewLogSessions()
	ok := ls.Deliver("nonexistent", []byte("data"))
	if ok {
		t.Error("Deliver should return false for unknown request_id")
	}
}

func TestLogSessions_Remove(t *testing.T) {
	ls := NewLogSessions()
	_ = ls.Register("req-2")
	ls.Remove("req-2")

	ok := ls.Deliver("req-2", []byte("should fail"))
	if ok {
		t.Error("Deliver should return false after Remove")
	}
}

func TestLogSessions_MultipleDeliveries(t *testing.T) {
	ls := NewLogSessions()
	ch := ls.Register("req-3")

	// Deliver multiple chunks
	for i := 0; i < 5; i++ {
		ok := ls.Deliver("req-3", []byte("chunk"))
		if !ok {
			t.Errorf("Deliver %d failed", i)
		}
	}

	// Read all
	for i := 0; i < 5; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for chunk %d", i)
		}
	}
}

func TestLogSessions_BufferFull(t *testing.T) {
	ls := NewLogSessions()
	_ = ls.Register("req-4")

	// Fill the buffer (capacity 64)
	for i := 0; i < 64; i++ {
		ls.Deliver("req-4", []byte("x"))
	}

	// 65th delivery should drop (return false) without blocking
	ok := ls.Deliver("req-4", []byte("overflow"))
	if ok {
		t.Error("expected Deliver to return false when buffer is full")
	}
}
```

- [ ] **Step 2: Verify tests fail**

```bash
cd /home/wyiu/personal/aerodocs/hub && go test ./internal/grpcserver/ -run TestLogSessions 2>&1 | head -5
```

Expected: compilation error (LogSessions not defined).

### 4b: Implement LogSessions

- [ ] **Step 3: Create LogSessions**

Create `hub/internal/grpcserver/logsessions.go`:

```go
package grpcserver

import "sync"

// LogSessions tracks active log tailing sessions. Unlike PendingRequests
// (which closes after one delivery), LogSessions channels remain open for
// continuous streaming until explicitly removed.
type LogSessions struct {
	mu       sync.Mutex
	channels map[string]chan []byte
}

func NewLogSessions() *LogSessions {
	return &LogSessions{
		channels: make(map[string]chan []byte),
	}
}

// Register creates a buffered channel for the given requestID and returns it.
// The caller reads from this channel to receive log chunks.
func (ls *LogSessions) Register(requestID string) chan []byte {
	ch := make(chan []byte, 64)
	ls.mu.Lock()
	ls.channels[requestID] = ch
	ls.mu.Unlock()
	return ch
}

// Deliver sends data to the channel for the given requestID.
// Returns false if the requestID is unknown or the channel buffer is full.
func (ls *LogSessions) Deliver(requestID string, data []byte) bool {
	ls.mu.Lock()
	ch, ok := ls.channels[requestID]
	ls.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- data:
		return true
	default:
		return false
	}
}

// Remove deletes the channel for the given requestID.
// The channel is closed so readers can detect removal.
func (ls *LogSessions) Remove(requestID string) {
	ls.mu.Lock()
	ch, ok := ls.channels[requestID]
	if ok {
		close(ch)
		delete(ls.channels, requestID)
	}
	ls.mu.Unlock()
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/wyiu/personal/aerodocs/hub && go test ./internal/grpcserver/ -run TestLogSessions -v -count=1
```

Expected: all 5 tests pass.

### Commit

```
git add hub/internal/grpcserver/logsessions.go hub/internal/grpcserver/logsessions_test.go
git commit -m "hub: add LogSessions for streaming log chunk delivery"
```

---

## Task 5: Hub gRPC Handler Dispatch + Wiring

**Files:** `hub/internal/grpcserver/handler.go`, `hub/internal/grpcserver/server.go`, `hub/internal/server/server.go`, `hub/cmd/aerodocs/main.go`
**Depends on:** Task 4
**Estimated time:** 10 minutes

### 5a: Add logSessions to Handler

- [ ] **Step 1: Add logSessions field to Handler struct**

In `hub/internal/grpcserver/handler.go`, add to the Handler struct:

```go
type Handler struct {
	pb.UnimplementedAgentServiceServer
	store       *store.Store
	connMgr     *connmgr.ConnManager
	pending     *PendingRequests
	logSessions *LogSessions
}
```

- [ ] **Step 2: Add LogStreamChunk dispatch to receive loop**

In the receive loop of `Connect()`, add a new case after the `FileReadResponse` case:

```go
		case *pb.AgentMessage_LogStreamChunk:
			if h.logSessions != nil {
				h.logSessions.Deliver(p.LogStreamChunk.RequestId, p.LogStreamChunk.Data)
			}
```

### 5b: Wire logSessions through Server and Config

- [ ] **Step 3: Add logSessions to grpcserver.Server and Config**

In `hub/internal/grpcserver/server.go`, add to Config:

```go
type Config struct {
	Addr        string
	Store       *store.Store
	ConnMgr     *connmgr.ConnManager
	Pending     *PendingRequests
	LogSessions *LogSessions
}
```

Add to Server struct:

```go
type Server struct {
	grpcServer  *grpc.Server
	store       *store.Store
	connMgr     *connmgr.ConnManager
	pending     *PendingRequests
	logSessions *LogSessions
	addr        string
}
```

Update `New()` to pass logSessions:

```go
func New(cfg Config) *Server {
	if cfg.Pending == nil {
		cfg.Pending = NewPendingRequests()
	}
	if cfg.LogSessions == nil {
		cfg.LogSessions = NewLogSessions()
	}
	s := &Server{
		store:       cfg.Store,
		connMgr:     cfg.ConnMgr,
		pending:     cfg.Pending,
		logSessions: cfg.LogSessions,
		addr:        cfg.Addr,
	}
	s.grpcServer = grpc.NewServer()
	handler := &Handler{
		store:       cfg.Store,
		connMgr:     cfg.ConnMgr,
		pending:     s.pending,
		logSessions: s.logSessions,
	}
	pb.RegisterAgentServiceServer(s.grpcServer, handler)
	return s
}
```

- [ ] **Step 4: Add logSessions to HTTP server**

In `hub/internal/server/server.go`, add to Server struct:

```go
	logSessions *grpcserver.LogSessions
```

Add to Config:

```go
	LogSessions *grpcserver.LogSessions
```

In `New()`, add:

```go
	s := &Server{
		store:       cfg.Store,
		jwtSecret:   cfg.JWTSecret,
		isDev:       cfg.IsDev,
		frontendFS:  cfg.FrontendFS,
		agentBinDir: cfg.AgentBinDir,
		grpcAddr:    cfg.GRPCAddr,
		connMgr:     cfg.ConnMgr,
		pending:     cfg.Pending,
		logSessions: cfg.LogSessions,
	}
```

- [ ] **Step 5: Wire LogSessions in main.go**

In `hub/cmd/aerodocs/main.go`, after `pending := grpcserver.NewPendingRequests()`, add:

```go
	logSessions := grpcserver.NewLogSessions()
```

Add `LogSessions: logSessions,` to both the `server.Config` and `grpcserver.Config` structs:

In the server.New call:

```go
	srv := server.New(server.Config{
		Addr:        *addr,
		Store:       st,
		JWTSecret:   jwtSecret,
		IsDev:       *dev,
		FrontendFS:  &hub.FrontendFS,
		AgentBinDir: *agentBinDir,
		GRPCAddr:    *grpcAddr,
		ConnMgr:     cm,
		Pending:     pending,
		LogSessions: logSessions,
	})
```

In the grpcserver.New call:

```go
	grpcSrv := grpcserver.New(grpcserver.Config{
		Addr:        *grpcAddr,
		Store:       st,
		ConnMgr:     cm,
		Pending:     pending,
		LogSessions: logSessions,
	})
```

- [ ] **Step 6: Verify hub compiles**

```bash
cd /home/wyiu/personal/aerodocs/hub && go build ./...
```

Expected: clean compile.

- [ ] **Step 7: Run all hub tests**

```bash
cd /home/wyiu/personal/aerodocs/hub && go test ./... -count=1
```

Expected: all tests pass.

### Commit

```
git add hub/internal/grpcserver/handler.go hub/internal/grpcserver/server.go hub/internal/server/server.go hub/cmd/aerodocs/main.go
git commit -m "hub: wire LogSessions through gRPC handler, server, and main"
```

---

## Task 6: Hub Audit Constant + SSE Endpoint

**Files:** `hub/internal/model/audit.go`, `hub/internal/server/handlers_logs.go`, `hub/internal/server/handlers_logs_test.go`, `hub/internal/server/router.go`, `hub/internal/server/server.go`
**Depends on:** Task 5
**Estimated time:** 20 minutes

### 6a: Add audit constant

- [ ] **Step 1: Add log tail audit constant**

In `hub/internal/model/audit.go`, add to the file access events block:

```go
// File access events
const (
	AuditFileRead       = "file.read"
	AuditLogTailStarted = "log.tail_started"
	AuditPathGranted    = "path.granted"
	AuditPathRevoked    = "path.revoked"
)
```

### 6b: Write SSE handler tests

- [ ] **Step 2: Create test file**

Create `hub/internal/server/handlers_logs_test.go`:

```go
package server

import (
	"testing"
)

func TestValidateLogTailParams(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute path", "/var/log/syslog", false},
		{"valid nested path", "/var/log/nginx/access.log", false},
		{"empty path", "", true},
		{"relative path", "var/log/syslog", true},
		{"path traversal", "/var/log/../../etc/passwd", true},
		{"root path", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequestPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRequestPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
```

### 6c: Implement SSE endpoint

- [ ] **Step 3: Create handlers_logs.go**

Create `hub/internal/server/handlers_logs.go`:

```go
package server

import (
	"encoding/base64"
	"fmt"
	"net/http"

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

	if err := validateRequestPath(path); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check permissions (same as file browsing)
	userID := UserIDFromContext(r.Context())
	role := UserRoleFromContext(r.Context())
	if role != "admin" {
		allowed, err := s.isPathAllowed(userID, serverID, path)
		if err != nil || !allowed {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	// Check agent is connected
	if s.connMgr == nil {
		respondError(w, http.StatusBadGateway, "agent not connected")
		return
	}
	conn := s.connMgr.GetConn(serverID)
	if conn == nil {
		respondError(w, http.StatusBadGateway, "agent not connected")
		return
	}

	// Verify LogSessions is available
	if s.logSessions == nil {
		respondError(w, http.StatusInternalServerError, "log sessions not initialized")
		return
	}

	// Assert Flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Generate request ID and register log session
	requestID := uuid.NewString()
	ch := s.logSessions.Register(requestID)
	defer s.logSessions.Remove(requestID)

	// Send LogStreamRequest to agent
	err := s.connMgr.SendToAgent(serverID, &pb.HubMessage{
		Payload: &pb.HubMessage_LogStreamRequest{
			LogStreamRequest: &pb.LogStreamRequest{
				RequestId: requestID,
				Path:      path,
				Offset:    0,
				Grep:      grep,
			},
		},
	})
	if err != nil {
		respondError(w, http.StatusBadGateway, "failed to send request to agent")
		return
	}

	// Audit log
	ip := clientIP(r)
	detail := fmt.Sprintf("%s (grep: %s)", path, grep)
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

	// Stream loop
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected — send LogStreamStop to agent
			_ = s.connMgr.SendToAgent(serverID, &pb.HubMessage{
				Payload: &pb.HubMessage_LogStreamStop{
					LogStreamStop: &pb.LogStreamStop{
						RequestId: requestID,
					},
				},
			})
			return

		case data, ok := <-ch:
			if !ok {
				// Channel closed (session removed)
				return
			}
			// Encode chunk as base64 and send as SSE event
			encoded := base64.StdEncoding.EncodeToString(data)
			fmt.Fprintf(w, "data: %s\n\n", encoded)
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/wyiu/personal/aerodocs/hub && go test ./internal/server/ -run TestValidateLogTail -v -count=1
```

Expected: all tests pass.

### 6d: Add route and increase WriteTimeout

- [ ] **Step 5: Register the SSE route**

In `hub/internal/server/router.go`, add after the file browsing endpoints block:

```go
	// Log tailing SSE endpoint (any authenticated user, permission-checked in handler)
	mux.Handle("GET /api/servers/{id}/logs/tail", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleTailLog))))
```

- [ ] **Step 6: Increase WriteTimeout for SSE**

SSE connections are long-lived, so the default 30s `WriteTimeout` will kill them. In `hub/internal/server/server.go`, update the `httpServer` config to use 0 (no timeout) for WriteTimeout, or wrap the SSE handler with `http.TimeoutHandler` bypass. The simplest approach is to set WriteTimeout to 0:

In the `New()` function, change:

```go
		WriteTimeout: 30 * time.Second,
```

To:

```go
		WriteTimeout: 0, // SSE endpoints require no write timeout
```

Note: This is safe because individual handlers already have their own timeouts (e.g., `agentTimeout` for file operations). The SSE endpoint manages its own lifecycle via context cancellation.

- [ ] **Step 7: Verify hub compiles**

```bash
cd /home/wyiu/personal/aerodocs/hub && go build ./...
```

Expected: clean compile.

- [ ] **Step 8: Run all hub tests**

```bash
cd /home/wyiu/personal/aerodocs/hub && go test ./... -count=1
```

Expected: all tests pass.

### Commit

```
git add hub/internal/model/audit.go hub/internal/server/handlers_logs.go hub/internal/server/handlers_logs_test.go hub/internal/server/router.go hub/internal/server/server.go
git commit -m "hub: add SSE log tail endpoint with audit logging"
```

---

## Task 7: Frontend Live Tail UI

**Files:** `web/src/pages/server-detail.tsx`
**Depends on:** Task 6
**Estimated time:** 25 minutes

### 7a: Add Live Tail state and hooks

- [ ] **Step 1: Add new imports to server-detail.tsx**

Add to the Lucide imports:

```tsx
import {
  // ... existing imports ...
  Play,
  Square,
  Pause,
  Search,
} from 'lucide-react'
```

Add the `getAccessToken` import:

```tsx
import { getAccessToken } from '@/lib/auth'
```

- [ ] **Step 2: Add isTextFile helper function**

Add after the `isMarkdownFile` function:

```tsx
function isTextFile(mimeType: string | undefined, path: string): boolean {
  if (mimeType && mimeType.startsWith('text/')) return true
  const textExts = [
    'log', 'txt', 'conf', 'cfg', 'ini', 'toml', 'yaml', 'yml',
    'json', 'xml', 'html', 'htm', 'css', 'js', 'jsx', 'ts', 'tsx',
    'py', 'go', 'sh', 'bash', 'zsh', 'sql', 'md', 'markdown',
    'env', 'properties', 'service', 'timer', 'socket',
  ]
  const ext = path.split('.').pop()?.toLowerCase() ?? ''
  const name = path.split('/').pop()?.toLowerCase() ?? ''
  return textExts.includes(ext) || name === 'dockerfile' || name === 'makefile'
}
```

### 7b: Add LiveTail component

- [ ] **Step 3: Create the LiveTail component**

Add before the `ServerDetailPage` component:

```tsx
// --- LiveTail Component ---

const MAX_TAIL_LINES = 10000

function LiveTail({
  serverId,
  filePath,
  onStop,
}: {
  serverId: string
  filePath: string
  onStop: () => void
}) {
  const [lines, setLines] = useState<string[]>([])
  const [lineCount, setLineCount] = useState(0)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [paused, setPaused] = useState(false)
  const [grep, setGrep] = useState('')
  const [grepInput, setGrepInput] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const eventSourceRef = useRef<EventSource | null>(null)

  // Auto-scroll when new lines arrive and not paused
  useEffect(() => {
    if (!paused && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [lines, paused])

  // SSE connection
  useEffect(() => {
    const token = getAccessToken()
    if (!token) {
      setError('Not authenticated')
      return
    }

    const params = new URLSearchParams({ path: filePath })
    if (grep) params.set('grep', grep)

    // EventSource doesn't support custom headers, so pass token as query param
    // We'll need to handle this on the server side, or use a different approach
    // Standard approach: use fetch-based SSE with custom headers
    const url = `/api/servers/${serverId}/logs/tail?${params.toString()}`

    let aborted = false

    const connect = () => {
      const controller = new AbortController()

      fetch(url, {
        headers: {
          'Authorization': `Bearer ${token}`,
          'Accept': 'text/event-stream',
        },
        signal: controller.signal,
      }).then(async (response) => {
        if (!response.ok) {
          const body = await response.json().catch(() => ({ error: 'Connection failed' }))
          setError((body as { error?: string }).error || `HTTP ${response.status}`)
          return
        }

        setConnected(true)
        setError(null)

        const reader = response.body?.getReader()
        if (!reader) {
          setError('Streaming not supported')
          return
        }

        const decoder = new TextDecoder()
        let buffer = ''

        while (!aborted) {
          const { done, value } = await reader.read()
          if (done) break

          buffer += decoder.decode(value, { stream: true })

          // Parse SSE events from buffer
          const eventEnd = buffer.lastIndexOf('\n\n')
          if (eventEnd === -1) continue

          const complete = buffer.substring(0, eventEnd)
          buffer = buffer.substring(eventEnd + 2)

          const events = complete.split('\n\n')
          for (const event of events) {
            const dataMatch = event.match(/^data: (.+)$/m)
            if (!dataMatch) continue

            try {
              const bytes = atob(dataMatch[1])
              const text = new TextDecoder().decode(
                Uint8Array.from(bytes, (c) => c.charCodeAt(0))
              )
              const newLines = text.split('\n').filter((l) => l.length > 0)
              if (newLines.length > 0) {
                setLines((prev) => {
                  const updated = [...prev, ...newLines]
                  if (updated.length > MAX_TAIL_LINES) {
                    return updated.slice(updated.length - MAX_TAIL_LINES)
                  }
                  return updated
                })
                setLineCount((prev) => prev + newLines.length)
              }
            } catch {
              // Skip malformed data
            }
          }
        }
      }).catch((err) => {
        if (!aborted) {
          setError(err instanceof Error ? err.message : 'Connection lost')
          setConnected(false)
        }
      })

      // Store abort function for cleanup
      eventSourceRef.current = { close: () => { controller.abort() } } as unknown as EventSource
    }

    connect()

    return () => {
      aborted = true
      eventSourceRef.current?.close()
      setConnected(false)
    }
  }, [serverId, filePath, grep])

  const handleGrepSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setLines([])
    setLineCount(0)
    setGrep(grepInput)
  }

  return (
    <div className="flex flex-col h-full">
      {/* Controls bar */}
      <div className="flex items-center gap-2 px-4 py-2 border-b border-border bg-surface/50 shrink-0">
        <div className="flex items-center gap-1.5">
          <span
            className={`w-2 h-2 rounded-full ${connected ? 'bg-status-online animate-pulse' : 'bg-status-offline'}`}
          />
          <span className="text-xs text-text-muted">
            {connected ? 'Streaming' : error ? 'Error' : 'Connecting...'}
          </span>
        </div>

        <div className="flex-1" />

        {/* Grep filter */}
        <form onSubmit={handleGrepSubmit} className="flex items-center gap-1.5">
          <div className="relative">
            <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-text-faint" />
            <input
              type="text"
              placeholder="Filter..."
              value={grepInput}
              onChange={(e) => setGrepInput(e.target.value)}
              className="pl-7 pr-2 py-1 text-xs bg-elevated border border-border rounded w-40 text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent font-mono"
            />
          </div>
          <button
            type="submit"
            className="px-2 py-1 text-xs bg-elevated border border-border rounded text-text-secondary hover:text-text-primary transition-colors"
          >
            Apply
          </button>
        </form>

        {/* Line count */}
        <span className="text-xs text-text-faint tabular-nums">
          {lineCount.toLocaleString()} lines
        </span>

        {/* Pause/Resume */}
        <button
          onClick={() => setPaused(!paused)}
          className={`flex items-center gap-1 px-2 py-1 text-xs rounded border transition-colors ${
            paused
              ? 'bg-status-warning/15 border-status-warning/30 text-status-warning'
              : 'bg-elevated border-border text-text-secondary hover:text-text-primary'
          }`}
          title={paused ? 'Resume auto-scroll' : 'Pause auto-scroll'}
        >
          {paused ? <Play className="w-3 h-3" /> : <Pause className="w-3 h-3" />}
          {paused ? 'Resume' : 'Pause'}
        </button>

        {/* Stop */}
        <button
          onClick={onStop}
          className="flex items-center gap-1 px-2 py-1 text-xs bg-status-error/15 border border-status-error/30 text-status-error rounded hover:bg-status-error/25 transition-colors"
        >
          <Square className="w-3 h-3" />
          Stop
        </button>
      </div>

      {/* Error banner */}
      {error && (
        <div className="bg-status-error/10 border-b border-status-error/20 px-4 py-2 text-xs text-status-error shrink-0">
          {error}
        </div>
      )}

      {/* Log output */}
      <div
        ref={containerRef}
        className="flex-1 overflow-auto bg-base p-4 font-mono text-xs leading-5"
      >
        {lines.length === 0 && connected && (
          <div className="text-text-faint italic">Waiting for log output...</div>
        )}
        {lines.map((line, i) => (
          <div key={i} className="text-text-secondary hover:bg-elevated/30 whitespace-pre-wrap break-all">
            {line}
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
```

### 7c: Integrate Live Tail into file viewer

- [ ] **Step 4: Add tailing state to ServerDetailPage**

In the `ServerDetailPage` component, add after the `markdownView` state:

```tsx
  const [isTailing, setIsTailing] = useState(false)
```

Reset tailing when file changes. Update the `handleSelectFile` callback:

```tsx
  const handleSelectFile = useCallback((node: FileNode) => {
    setSelectedFile(node)
    setMarkdownView('rendered')
    setIsTailing(false)
  }, [])
```

- [ ] **Step 5: Add Live Tail button to the breadcrumb controls bar**

In the breadcrumb controls area (the `div` with `flex items-center gap-2 shrink-0 ml-2`), add the Live Tail button after the Refresh button and before the closing `</div>`:

```tsx
                    {selectedFile &&
                      !selectedFile.is_dir &&
                      isTextFile(fileContent?.mime_type, selectedFile.path) &&
                      !isTailing && (
                        <button
                          onClick={() => setIsTailing(true)}
                          className="flex items-center gap-1 px-2 py-0.5 text-xs bg-accent/15 border border-accent/30 rounded text-accent hover:bg-accent/25 transition-colors"
                          title="Start live tailing this file"
                        >
                          <Play className="w-3 h-3" />
                          Live Tail
                        </button>
                      )}
```

- [ ] **Step 6: Conditionally render LiveTail or file content**

Replace the file content section (the `{/* File content */}` div with `className="flex-1 overflow-auto"`) with a conditional:

```tsx
                {/* File content area */}
                {isTailing && selectedFile ? (
                  <LiveTail
                    serverId={serverId!}
                    filePath={selectedFile.path}
                    onStop={() => setIsTailing(false)}
                  />
                ) : (
                  <div className="flex-1 overflow-auto">
                    {fileLoading ? (
                      /* ... existing loading/error/content rendering stays exactly the same ... */
                    )}
                  </div>
                )}
```

The existing `<div className="flex-1 overflow-auto">` block with its loading, error, and content rendering remains intact inside the else branch. The only change is wrapping it in the ternary to show LiveTail when `isTailing` is true.

- [ ] **Step 7: Verify the frontend builds**

```bash
cd /home/wyiu/personal/aerodocs/web && npm run build 2>&1 | tail -5
```

Expected: build succeeds with no type errors.

### Commit

```
git add web/src/pages/server-detail.tsx
git commit -m "web: add Live Tail UI with SSE streaming and grep filter"
```

---

## Task 8: Final Verification

**Depends on:** All previous tasks
**Estimated time:** 10 minutes

- [ ] **Step 1: Full hub build**

```bash
cd /home/wyiu/personal/aerodocs/hub && go build ./...
```

Expected: clean compile.

- [ ] **Step 2: Full agent build**

```bash
cd /home/wyiu/personal/aerodocs/agent && go build ./...
```

Expected: clean compile.

- [ ] **Step 3: Run all hub tests**

```bash
cd /home/wyiu/personal/aerodocs/hub && go test ./... -v -count=1
```

Expected: all tests pass.

- [ ] **Step 4: Run all agent tests**

```bash
cd /home/wyiu/personal/aerodocs/agent && go test ./... -v -count=1
```

Expected: all tests pass.

- [ ] **Step 5: Frontend build**

```bash
cd /home/wyiu/personal/aerodocs/web && npm run build
```

Expected: clean build, no errors.

- [ ] **Step 6: Verify proto is in sync**

```bash
cd /home/wyiu/personal/aerodocs && make proto && git diff --stat proto/
```

Expected: no diff (proto files already up to date).

### Final Commit (if any fixups needed)

```
git add -A
git commit -m "chore: final cleanup for sub-project 5 log tailing"
```
