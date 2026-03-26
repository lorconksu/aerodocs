package filebrowser

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

const MaxReadSize = 1048576 // 1MB

func validatePath(path string) error {
	// Reject paths that contain ".." components before cleaning
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("path must be absolute")
	}
	return nil
}

func ListDir(path string) (*pb.FileListResponse, error) {
	if err := validatePath(path); err != nil {
		return &pb.FileListResponse{Error: err.Error()}, nil
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		log.Printf("ListDir: cannot resolve path %q: %v", path, err)
		return &pb.FileListResponse{Error: "cannot access path"}, nil
	}
	// Ensure symlinks don't escape above the requested directory
	cleanPath := filepath.Clean(path)
	if resolved != cleanPath && !strings.HasPrefix(resolved, cleanPath) {
		return &pb.FileListResponse{Error: "path resolves outside requested directory"}, nil
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		log.Printf("ListDir: cannot read directory %q: %v", resolved, err)
		return &pb.FileListResponse{Error: "cannot read directory"}, nil
	}

	var dirs, files []*pb.FileNode
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}

		node := &pb.FileNode{
			Name:  e.Name(),
			Path:  filepath.Join(path, e.Name()),
			IsDir: e.IsDir(),
			Size:  info.Size(),
		}

		// Check readability
		f, err := os.Open(filepath.Join(resolved, e.Name()))
		if err == nil {
			node.Readable = true
			f.Close()
		}

		if e.IsDir() {
			dirs = append(dirs, node)
		} else {
			files = append(files, node)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	allFiles := append(dirs, files...)
	return &pb.FileListResponse{Files: allFiles}, nil
}

func ReadFile(path string, offset, limit int64) (*pb.FileReadResponse, error) {
	if err := validatePath(path); err != nil {
		return &pb.FileReadResponse{Error: err.Error()}, nil
	}

	if limit > MaxReadSize {
		limit = MaxReadSize
	}
	if limit <= 0 {
		limit = MaxReadSize
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		log.Printf("ReadFile: cannot resolve path %q: %v", path, err)
		return &pb.FileReadResponse{Error: "cannot access path"}, nil
	}
	// Ensure symlinks don't escape above the requested directory
	cleanPath := filepath.Clean(path)
	if resolved != cleanPath && !strings.HasPrefix(resolved, cleanPath) {
		return &pb.FileReadResponse{Error: "path resolves outside requested directory"}, nil
	}

	info, err := os.Stat(resolved)
	if err != nil {
		log.Printf("ReadFile: cannot stat file %q: %v", resolved, err)
		return &pb.FileReadResponse{Error: "cannot read file"}, nil
	}
	if info.IsDir() {
		return &pb.FileReadResponse{Error: "path is a directory"}, nil
	}

	f, err := os.Open(resolved)
	if err != nil {
		log.Printf("ReadFile: cannot open file %q: %v", resolved, err)
		return &pb.FileReadResponse{Error: "cannot read file"}, nil
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			log.Printf("ReadFile: seek failed for %q at offset %d: %v", resolved, offset, err)
			return &pb.FileReadResponse{Error: "cannot read file"}, nil
		}
	}

	data := make([]byte, limit)
	n, err := io.ReadFull(f, data)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		log.Printf("ReadFile: read failed for %q: %v", resolved, err)
		return &pb.FileReadResponse{Error: "cannot read file"}, nil
	}

	return &pb.FileReadResponse{
		Data:      data[:n],
		TotalSize: info.Size(),
		MimeType:  detectMIME(filepath.Base(path)),
	}, nil
}

func detectMIME(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt", ".conf", ".cfg", ".ini", ".log", ".env":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".md", ".markdown":
		return "text/markdown"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".xml":
		return "text/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "text/javascript"
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".rs":
		return "text/x-rust"
	case ".sh", ".bash":
		return "text/x-sh"
	case ".toml":
		return "text/x-toml"
	case ".sql":
		return "text/x-sql"
	default:
		return "application/octet-stream"
	}
}
