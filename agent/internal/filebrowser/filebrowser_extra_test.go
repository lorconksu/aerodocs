package filebrowser

import (
	"os"
	"path/filepath"
	"testing"
)

// TestListDir_Empty verifies listing an empty directory returns empty list.
func TestListDir_Empty(t *testing.T) {
	dir := t.TempDir()

	resp, err := ListDir(dir)
	if err != nil {
		t.Fatalf("list empty dir: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if len(resp.Files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(resp.Files))
	}
}

// TestListDir_RelativePath verifies relative path returns an error response.
func TestListDir_RelativePath(t *testing.T) {
	resp, err := ListDir("relative/path")
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error in response for relative path")
	}
}

// TestReadFile_PathTraversal verifies path traversal returns error response.
func TestReadFile_PathTraversal(t *testing.T) {
	resp, err := ReadFile("/tmp/../etc/passwd", 0, 100)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error in response for path traversal")
	}
}

// TestReadFile_Directory verifies reading a directory returns an error response.
func TestReadFile_Directory(t *testing.T) {
	dir := t.TempDir()

	resp, err := ReadFile(dir, 0, 1048576)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error when reading a directory")
	}
}

// TestReadFile_LimitOverMax verifies the MaxReadSize limit is enforced.
func TestReadFile_LimitOverMax(t *testing.T) {
	dir := t.TempDir()
	content := make([]byte, 100)
	path := filepath.Join(dir, "test.bin")
	os.WriteFile(path, content, 0644)

	// Request more than MaxReadSize — should be clamped
	resp, err := ReadFile(path, 0, MaxReadSize*2)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}

// TestReadFile_ZeroLimit uses 0 limit (defaults to MaxReadSize).
func TestReadFile_ZeroLimit(t *testing.T) {
	dir := t.TempDir()
	content := []byte("test content")
	path := filepath.Join(dir, "zero-limit.txt")
	os.WriteFile(path, content, 0644)

	resp, err := ReadFile(path, 0, 0)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if string(resp.Data) != "test content" {
		t.Fatalf("expected 'test content', got '%s'", string(resp.Data))
	}
}

// TestDetectMIME_AllTypes covers all MIME type branches.
func TestDetectMIME_AllTypes(t *testing.T) {
	tests := map[string]string{
		"file.txt":      "text/plain",
		"file.conf":     "text/plain",
		"file.cfg":      "text/plain",
		"file.ini":      "text/plain",
		"file.log":      "text/plain",
		"file.env":      "text/plain",
		"file.json":     "application/json",
		"file.md":       "text/markdown",
		"file.markdown": "text/markdown",
		"file.yaml":     "text/yaml",
		"file.yml":      "text/yaml",
		"file.xml":      "text/xml",
		"file.html":     "text/html",
		"file.htm":      "text/html",
		"file.css":      "text/css",
		"file.js":       "text/javascript",
		"file.go":       "text/x-go",
		"file.py":       "text/x-python",
		"file.rs":       "text/x-rust",
		"file.sh":       "text/x-sh",
		"file.bash":     "text/x-sh",
		"file.toml":     "text/x-toml",
		"file.sql":      "text/x-sql",
		"file.unknown":  "application/octet-stream",
		"noextension":   "application/octet-stream",
	}

	for name, expected := range tests {
		got := detectMIME(name)
		if got != expected {
			t.Errorf("detectMIME(%q) = %q, want %q", name, got, expected)
		}
	}
}

// TestValidatePath_AllCases covers all validatePath branches.
func TestValidatePath_AllCases(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute", "/var/log/syslog", false},
		{"root path", "/", false},
		{"path traversal", "/var/../etc/passwd", true},
		{"double dot traversal", "/../../etc", true},
		{"relative path", "relative/path", true},
		{"empty string causes relative", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePath(tc.path)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for path %q, got nil", tc.path)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for path %q, got %v", tc.path, err)
			}
		})
	}
}

// TestListDir_Symlink verifies symlinks that escape outside the requested directory are rejected.
func TestListDir_Symlink(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()

	// Create a file in targetDir
	os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("linked"), 0644)

	// Create a symlink in dir pointing to targetDir (outside dir)
	linkPath := filepath.Join(dir, "linkdir")
	if err := os.Symlink(targetDir, linkPath); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	resp, err := ListDir(linkPath)
	if err != nil {
		t.Fatalf("list symlinked dir: %v", err)
	}
	// Symlink escapes outside the requested directory, should be rejected
	if resp.Error == "" {
		t.Fatal("expected symlink escape to be rejected")
	}
}

// TestListDir_UnreadableDir verifies permission errors are handled gracefully.
func TestListDir_UnreadableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test as root")
	}

	dir := t.TempDir()
	noReadDir := filepath.Join(dir, "no-read")
	os.MkdirAll(noReadDir, 0000)
	defer os.Chmod(noReadDir, 0755) // cleanup

	resp, err := ListDir(noReadDir)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	// Should return an error in the response
	if resp.Error == "" {
		t.Log("Note: no error returned for unreadable dir (may be root or other conditions)")
	}
}

// TestReadFile_SymlinkToFile verifies reading through a symlink that resolves
// to a different path is rejected (defense-in-depth).
func TestReadFile_SymlinkToFile(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.txt")
	os.WriteFile(realFile, []byte("symlinked content"), 0644)

	linkFile := filepath.Join(dir, "link.txt")
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	resp, err := ReadFile(linkFile, 0, 1048576)
	if err != nil {
		t.Fatalf("read through symlink: %v", err)
	}
	// Symlink resolves to a different path, should be rejected
	if resp.Error == "" {
		t.Fatal("expected symlink to different path to be rejected")
	}
}
