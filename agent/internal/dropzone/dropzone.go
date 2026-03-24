package dropzone

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

const DefaultDir = "/tmp/aerodocs-dropzone"

// Dropzone manages file uploads to a staging directory.
type Dropzone struct {
	dir     string
	mu      sync.Mutex
	uploads map[string]*os.File
}

// New creates a Dropzone that writes files to dir.
func New(dir string) *Dropzone {
	os.MkdirAll(dir, 0755)
	return &Dropzone{
		dir:     dir,
		uploads: make(map[string]*os.File),
	}
}

// HandleChunk processes a file upload chunk.
// Returns a FileUploadAck when done=true or on error, nil otherwise.
func (d *Dropzone) HandleChunk(requestID, filename string, data []byte, done bool) *pb.FileUploadAck {
	d.mu.Lock()
	defer d.mu.Unlock()

	f, exists := d.uploads[requestID]

	// First chunk — open the file
	if !exists {
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
		var err error
		f, err = os.Create(path)
		if err != nil {
			return &pb.FileUploadAck{
				RequestId: requestID,
				Success:   false,
				Error:     fmt.Sprintf("create file: %v", err),
			}
		}
		d.uploads[requestID] = f
	}

	// Write data
	if len(data) > 0 {
		if _, err := f.Write(data); err != nil {
			f.Close()
			delete(d.uploads, requestID)
			return &pb.FileUploadAck{
				RequestId: requestID,
				Success:   false,
				Error:     fmt.Sprintf("write chunk: %v", err),
			}
		}
	}

	// Final chunk — close file
	if done {
		f.Close()
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
	for id, f := range d.uploads {
		f.Close()
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
