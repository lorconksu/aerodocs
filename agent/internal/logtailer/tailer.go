package logtailer

import (
	"bufio"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

const pollInterval = 500 * time.Millisecond

// StartTail opens a file, seeks to offset (0 = end of file), and polls for new data.
// Matching lines (if grep is non-empty) are sent as LogStreamChunk messages.
// Stops when the stop channel is closed.
func StartTail(path string, grep string, offset int64, sendCh chan<- *pb.AgentMessage, requestID string, stop <-chan struct{}) {
	// Defense in depth: validate path even though hub should have checked
	if strings.Contains(path, "..") || !filepath.IsAbs(path) {
		sendCh <- &pb.AgentMessage{
			Payload: &pb.AgentMessage_LogStreamChunk{
				LogStreamChunk: &pb.LogStreamChunk{
					RequestId: requestID,
					Data:      []byte("error: invalid path"),
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
