package dropzone

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

const DefaultDir = "/tmp/aerodocs-dropzone"

// MaxUploadSize is the maximum total bytes allowed per file upload (100MB).
const MaxUploadSize = 100 * 1024 * 1024

// MaxConcurrentUploads limits simultaneous in-progress uploads.
const MaxConcurrentUploads = 10

// uploadState tracks an in-progress upload's file handle and byte count.
type uploadState struct {
	file  *os.File
	bytes int64
}

// Dropzone manages file uploads to a staging directory.
type Dropzone struct {
	dir     string
	mu      sync.Mutex
	uploads map[string]*uploadState
}

// New creates a Dropzone that writes files to dir.
func New(dir string) *Dropzone {
	os.MkdirAll(dir, 0700)
	return &Dropzone{
		dir:     dir,
		uploads: make(map[string]*uploadState),
	}
}

// HandleChunk processes a file upload chunk.
// Returns a FileUploadAck when done=true or on error, nil otherwise.
func (d *Dropzone) HandleChunk(requestID, filename string, data []byte, done bool) *pb.FileUploadAck {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, exists := d.uploads[requestID]

	// First chunk — open the file
	if !exists {
		if len(d.uploads) >= MaxConcurrentUploads {
			return &pb.FileUploadAck{
				RequestId: requestID,
				Success:   false,
				Error:     "too many concurrent uploads",
			}
		}

		if filename == "" {
			return &pb.FileUploadAck{
				RequestId: requestID,
				Success:   false,
				Error:     "no filename provided",
			}
		}

		// Sanitize filename
		safe := sanitizeFilename(filename)
		if safe == "" {
			return &pb.FileUploadAck{
				RequestId: requestID,
				Success:   false,
				Error:     "invalid filename",
			}
		}

		path := filepath.Join(d.dir, safe)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			log.Printf("dropzone create error: %v", err)
			return &pb.FileUploadAck{
				RequestId: requestID,
				Success:   false,
				Error:     "file operation failed",
			}
		}
		state = &uploadState{file: f}
		d.uploads[requestID] = state
	}

	// Write data with size limit enforcement
	if len(data) > 0 {
		if state.bytes+int64(len(data)) > MaxUploadSize {
			log.Printf("dropzone: upload %s exceeds %d byte limit", requestID, MaxUploadSize)
			state.file.Close()
			os.Remove(state.file.Name())
			delete(d.uploads, requestID)
			return &pb.FileUploadAck{
				RequestId: requestID,
				Success:   false,
				Error:     "file size limit exceeded",
			}
		}
		if _, err := state.file.Write(data); err != nil {
			log.Printf("dropzone write error: %v", err)
			state.file.Close()
			delete(d.uploads, requestID)
			return &pb.FileUploadAck{
				RequestId: requestID,
				Success:   false,
				Error:     "file operation failed",
			}
		}
		state.bytes += int64(len(data))
	}

	// Final chunk — close file
	if done {
		state.file.Close()
		delete(d.uploads, requestID)
		return &pb.FileUploadAck{
			RequestId: requestID,
			Success:   true,
		}
	}

	return nil
}

// Cleanup closes any open uploads (called on disconnect).
func (d *Dropzone) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, state := range d.uploads {
		state.file.Close()
		delete(d.uploads, id)
	}
}

func sanitizeFilename(name string) string {
	// Take only the base name (strips directory components and ..)
	base := filepath.Base(name)
	if base == "." || base == ".." || base == "/" {
		return ""
	}
	return base
}
