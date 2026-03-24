package logtailer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

func TestTailNewLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	os.WriteFile(path, []byte("line1\n"), 0644)

	sendCh := make(chan *pb.AgentMessage, 10)
	stop := make(chan struct{})

	go StartTail(path, "", 0, sendCh, "req-1", stop)

	// Wait for initial position, then append
	time.Sleep(100 * time.Millisecond)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("line2\nline3\n")
	f.Close()

	// Wait for poll
	time.Sleep(700 * time.Millisecond)
	close(stop)

	// Should have received at least one chunk with new data
	found := false
	for len(sendCh) > 0 {
		msg := <-sendCh
		chunk := msg.GetLogStreamChunk()
		if chunk != nil && strings.Contains(string(chunk.Data), "line2") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to receive chunk with 'line2'")
	}
}

func TestTailWithGrep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	os.WriteFile(path, []byte(""), 0644)

	sendCh := make(chan *pb.AgentMessage, 10)
	stop := make(chan struct{})

	go StartTail(path, "error", 0, sendCh, "req-1", stop)

	time.Sleep(100 * time.Millisecond)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("info: all good\nerror: something broke\nwarn: hmm\n")
	f.Close()

	time.Sleep(700 * time.Millisecond)
	close(stop)

	found := false
	for len(sendCh) > 0 {
		msg := <-sendCh
		chunk := msg.GetLogStreamChunk()
		if chunk != nil {
			data := string(chunk.Data)
			if strings.Contains(data, "error") && !strings.Contains(data, "info: all good") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected grep to filter to only error lines")
	}
}

func TestTailFileRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	os.WriteFile(path, []byte("old content that is quite long\n"), 0644)

	sendCh := make(chan *pb.AgentMessage, 10)
	stop := make(chan struct{})

	go StartTail(path, "", 0, sendCh, "req-1", stop)

	time.Sleep(100 * time.Millisecond)

	// Simulate rotation: truncate and write new content
	os.WriteFile(path, []byte("new\n"), 0644)

	time.Sleep(700 * time.Millisecond)
	close(stop)

	found := false
	for len(sendCh) > 0 {
		msg := <-sendCh
		chunk := msg.GetLogStreamChunk()
		if chunk != nil && strings.Contains(string(chunk.Data), "new") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to detect rotation and read new content")
	}
}

func TestTailStop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	os.WriteFile(path, []byte("line1\n"), 0644)

	sendCh := make(chan *pb.AgentMessage, 10)
	stop := make(chan struct{})

	go StartTail(path, "", 0, sendCh, "req-1", stop)
	time.Sleep(100 * time.Millisecond)
	close(stop)
	time.Sleep(200 * time.Millisecond)

	// After stop, no more messages should be sent
	initialLen := len(sendCh)
	time.Sleep(700 * time.Millisecond)
	if len(sendCh) != initialLen {
		t.Fatal("expected no new messages after stop")
	}
}

func TestTailFileNotFound(t *testing.T) {
	sendCh := make(chan *pb.AgentMessage, 10)
	stop := make(chan struct{})

	go StartTail("/nonexistent/file.log", "", 0, sendCh, "req-1", stop)
	time.Sleep(200 * time.Millisecond)
	close(stop)

	// Should not panic, may send error chunk or nothing
}
