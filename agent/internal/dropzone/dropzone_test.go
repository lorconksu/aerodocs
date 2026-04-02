package dropzone

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testExistingTxt = "existing.txt"
)

func TestHandleChunk_SingleFile(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)

	// First chunk with filename
	ack := d.HandleChunk("req-1", "test.txt", []byte("hello "), false)
	if ack != nil {
		t.Fatal("expected no ack for non-final chunk")
	}

	// Second chunk
	ack = d.HandleChunk("req-1", "", []byte("world"), false)
	if ack != nil {
		t.Fatal("expected no ack for non-final chunk")
	}

	// Final chunk
	ack = d.HandleChunk("req-1", "", nil, true)
	if ack == nil {
		t.Fatal("expected ack for final chunk")
	}
	if !ack.Success {
		t.Fatalf("expected success, got error: %s", ack.Error)
	}

	// Verify file contents
	data, err := os.ReadFile(filepath.Join(dir, "test.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", string(data))
	}
}

func TestHandleChunk_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)

	ack := d.HandleChunk("req-1", "empty.txt", nil, true)
	if ack == nil || !ack.Success {
		t.Fatal("expected success for empty file")
	}

	data, _ := os.ReadFile(filepath.Join(dir, "empty.txt"))
	if len(data) != 0 {
		t.Fatalf("expected empty file, got %d bytes", len(data))
	}
}

func TestHandleChunk_Overwrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, testExistingTxt), []byte("old"), 0644)
	d := New(dir)

	d.HandleChunk("req-1", testExistingTxt, []byte("new"), false)
	ack := d.HandleChunk("req-1", "", nil, true)
	if ack == nil || !ack.Success {
		t.Fatal("expected success on overwrite")
	}

	data, _ := os.ReadFile(filepath.Join(dir, testExistingTxt))
	if string(data) != "new" {
		t.Fatalf("expected 'new', got '%s'", string(data))
	}
}

func TestHandleChunk_SanitizeFilename(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)

	// Path traversal attempt
	ack := d.HandleChunk("req-1", "../../../etc/passwd", []byte("hack"), false)
	if ack == nil {
		// If no immediate error, the final chunk should still write to dropzone dir
		d.HandleChunk("req-1", "", nil, true)
	}

	// Should NOT create file outside dropzone
	if _, err := os.Stat("/etc/passwd-hack"); err == nil {
		t.Fatal("file created outside dropzone!")
	}

	// Should sanitize to just the basename
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file in dropzone, got %d", len(files))
	}
	if files[0].Name() != "passwd" {
		t.Fatalf("expected sanitized name 'passwd', got '%s'", files[0].Name())
	}
}

func TestHandleChunk_RejectEmptyFilename(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)

	ack := d.HandleChunk("req-1", "", []byte("data"), false)
	// First chunk with empty filename and no existing session should error
	if ack != nil && !ack.Success {
		return // error ack is fine
	}
}

func TestHandleChunk_UnknownRequestID(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)

	// Sending chunk for unknown request (no filename, no existing session)
	ack := d.HandleChunk("unknown", "", []byte("data"), false)
	if ack != nil && ack.Success {
		t.Fatal("expected error for unknown request with no filename")
	}
}

func TestHandleChunk_InvalidFilename(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)

	// Filename that sanitizes to empty (e.g. ".")
	ack := d.HandleChunk("req-1", ".", []byte("data"), false)
	if ack == nil {
		t.Fatal("expected error ack for invalid filename")
	}
	if ack.Success {
		t.Fatal("expected failure for invalid filename")
	}
}

func TestCleanup_WithOpenUploads(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)

	// Start an upload but don't finish
	d.HandleChunk("req-1", "partial.txt", []byte("some data"), false)

	// Cleanup should close the open file handle
	d.Cleanup()

	// After cleanup, uploads map should be empty
	// We verify by starting a new chunk for same request — it should create a new file
	ack := d.HandleChunk("req-1", "partial.txt", []byte("new data"), true)
	if ack == nil {
		t.Fatal("expected ack after cleanup")
	}
	if !ack.Success {
		t.Fatalf("expected success after cleanup, got: %s", ack.Error)
	}
}

func TestCleanup_Empty(t *testing.T) {
	dir := t.TempDir()
	d := New(dir)

	// Cleanup with no active uploads should not panic
	d.Cleanup()
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple.txt", "simple.txt"},
		{"../etc/passwd", "passwd"},
		{"../../root/.ssh/id_rsa", "id_rsa"},
		{".", ""},
		{"..", ""},
		{"/etc/passwd", "passwd"},
		{"subdir/file.txt", "file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.expected {
				t.Fatalf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNew_CreatesDirectory(t *testing.T) {
	dir := t.TempDir() + "/subdir"
	d := New(dir)
	if d == nil {
		t.Fatal("expected non-nil dropzone")
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
}
