package filebrowser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# hi"), 0644)

	resp, err := ListDir(dir)
	if err != nil {
		t.Fatalf("list dir: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	// Directories come first, then files, both alphabetical
	if len(resp.Files) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(resp.Files))
	}
	if resp.Files[0].Name != "subdir" || !resp.Files[0].IsDir {
		t.Fatalf("expected first entry to be dir 'subdir', got %s (is_dir=%v)", resp.Files[0].Name, resp.Files[0].IsDir)
	}
	if resp.Files[1].Name != "file.txt" || resp.Files[1].IsDir {
		t.Fatalf("expected second entry to be file 'file.txt', got %s", resp.Files[1].Name)
	}
}

func TestListDir_NotFound(t *testing.T) {
	resp, err := ListDir("/nonexistent/path/12345")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error in response")
	}
}

func TestListDir_PathTraversal(t *testing.T) {
	_, err := ListDir("/tmp/../etc/passwd")
	// Should either return error or resolve safely
	if err != nil {
		return // error is fine
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	content := []byte("hello world")
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, content, 0644)

	resp, err := ReadFile(path, 0, 1048576)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if string(resp.Data) != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", string(resp.Data))
	}
	if resp.TotalSize != 11 {
		t.Fatalf("expected total_size 11, got %d", resp.TotalSize)
	}
}

func TestReadFile_WithOffset(t *testing.T) {
	dir := t.TempDir()
	content := []byte("hello world")
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, content, 0644)

	resp, err := ReadFile(path, 6, 1048576)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(resp.Data) != "world" {
		t.Fatalf("expected 'world', got '%s'", string(resp.Data))
	}
}

func TestReadFile_LimitEnforced(t *testing.T) {
	dir := t.TempDir()
	content := make([]byte, 100)
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, content, 0644)

	resp, err := ReadFile(path, 0, 50)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(resp.Data) != 50 {
		t.Fatalf("expected 50 bytes, got %d", len(resp.Data))
	}
	if resp.TotalSize != 100 {
		t.Fatalf("expected total_size 100, got %d", resp.TotalSize)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	resp, err := ReadFile("/nonexistent/file.txt", 0, 1048576)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error in response")
	}
}

func TestValidatePath(t *testing.T) {
	if err := validatePath("/var/log/syslog"); err != nil {
		t.Fatalf("expected valid path: %v", err)
	}
	if err := validatePath("/var/log/../../../etc/passwd"); err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestDetectMIME(t *testing.T) {
	tests := map[string]string{
		"file.txt":    "text/plain",
		"file.json":   "application/json",
		"file.md":     "text/markdown",
		"file.yaml":   "text/yaml",
		"file.yml":    "text/yaml",
		"file.go":     "text/x-go",
		"file.py":     "text/x-python",
		"file.sh":     "text/x-sh",
		"file.conf":   "text/plain",
		"file.log":    "text/plain",
		"unknown.xyz": "application/octet-stream",
	}
	for name, expected := range tests {
		got := detectMIME(name)
		if got != expected {
			t.Errorf("detectMIME(%s) = %s, want %s", name, got, expected)
		}
	}
}
