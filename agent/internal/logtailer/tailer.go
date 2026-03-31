package logtailer

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

const pollInterval = 500 * time.Millisecond

// blockedPaths are sensitive system paths that the log tailer must never read.
var blockedPaths = []string{
	"/etc/shadow",
	"/etc/gshadow",
	"/etc/sudoers",
	"/etc/sudoers.d",
	"/root/.ssh",
	"/proc/self",
	"/proc/kcore",
	"/sys/firmware",
}

var blockedPrefixes = []string{
	"/proc/",
	"/sys/kernel/",
}

func validateLogPath(path string) error {
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("path must be absolute")
	}
	for _, blocked := range blockedPaths {
		if cleaned == blocked || strings.HasPrefix(cleaned, blocked+"/") {
			return fmt.Errorf("access to this path is restricted")
		}
	}
	for _, prefix := range blockedPrefixes {
		if strings.HasPrefix(cleaned, prefix) {
			return fmt.Errorf("access to this path is restricted")
		}
	}
	// Resolve symlinks to prevent reading sensitive files via symlink
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return fmt.Errorf("cannot resolve path")
	}
	for _, blocked := range blockedPaths {
		if resolved == blocked || strings.HasPrefix(resolved, blocked+"/") {
			return fmt.Errorf("access to this path is restricted")
		}
	}
	for _, prefix := range blockedPrefixes {
		if strings.HasPrefix(resolved, prefix) {
			return fmt.Errorf("access to this path is restricted")
		}
	}
	return nil
}

// StartTail opens a file, seeks to offset (0 = end of file), and polls for new data.
// Matching lines (if grep is non-empty) are sent as LogStreamChunk messages.
// Stops when the stop channel is closed.
func StartTail(path string, grep string, offset int64, sendCh chan<- *pb.AgentMessage, requestID string, stop <-chan struct{}) {
	if err := validateLogPath(path); err != nil {
		sendCh <- &pb.AgentMessage{
			Payload: &pb.AgentMessage_LogStreamChunk{
				LogStreamChunk: &pb.LogStreamChunk{
					RequestId: requestID,
					Data:      []byte("error: " + err.Error()),
				},
			},
		}
		return
	}

	f, err := os.Open(path)
	if err != nil {
		log.Printf("logtailer: cannot open %s: %v", path, err)
		return
	}
	defer f.Close()

	// Seek to position
	if offset <= 0 {
		// Seek to end
		pos, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			log.Printf("logtailer: seek error: %v", err)
			return
		}
		offset = pos
	} else {
		_, err := f.Seek(offset, io.SeekStart)
		if err != nil {
			log.Printf("logtailer: seek error: %v", err)
			return
		}
	}

	grepLower := strings.ToLower(grep)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			offset = readNewData(f, path, offset, grepLower, sendCh, requestID)
		}
	}
}

func readNewData(f *os.File, path string, lastOffset int64, grepLower string, sendCh chan<- *pb.AgentMessage, requestID string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return lastOffset
	}

	currentSize := info.Size()

	// File rotation detection: file got smaller
	if currentSize < lastOffset {
		// Reopen file (it may have been replaced)
		newF, err := os.Open(path)
		if err != nil {
			return lastOffset
		}
		// Read from beginning of new file, capped to 1MB to prevent memory exhaustion.
		data, err := io.ReadAll(io.LimitReader(newF, 1<<20))
		newF.Close()
		if err != nil {
			return lastOffset
		}
		sendFiltered(data, currentSize, grepLower, sendCh, requestID)
		// Re-seek the original file handle
		f.Seek(0, io.SeekEnd)
		return currentSize
	}

	if currentSize == lastOffset {
		return lastOffset // No new data
	}

	// Seek to last position and read new data (cap at 1MB per poll)
	f.Seek(lastOffset, io.SeekStart)
	readSize := currentSize - lastOffset
	if readSize > 1<<20 {
		readSize = 1 << 20
	}
	data := make([]byte, readSize)
	n, err := f.Read(data)
	if err != nil && err != io.EOF {
		return lastOffset
	}
	if n == 0 {
		return lastOffset
	}

	data = data[:n]
	newOffset := lastOffset + int64(n)

	sendFiltered(data, newOffset, grepLower, sendCh, requestID)
	return newOffset
}

func sendFiltered(data []byte, offset int64, grepLower string, sendCh chan<- *pb.AgentMessage, requestID string) {
	var outData []byte

	if grepLower == "" {
		outData = data
	} else {
		// Filter lines
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		var filtered []string
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), grepLower) {
				filtered = append(filtered, line)
			}
		}
		if len(filtered) == 0 {
			return
		}
		outData = []byte(strings.Join(filtered, "\n") + "\n")
	}

	if len(outData) == 0 {
		return
	}

	sendCh <- &pb.AgentMessage{
		Payload: &pb.AgentMessage_LogStreamChunk{
			LogStreamChunk: &pb.LogStreamChunk{
				RequestId: requestID,
				Data:      outData,
				Offset:    offset,
			},
		},
	}
}
