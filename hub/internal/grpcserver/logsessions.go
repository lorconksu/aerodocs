package grpcserver

import "sync"

// LogSessions tracks active log tailing sessions.
// Unlike PendingRequests (one-shot), log sessions deliver multiple
// chunks over time until explicitly removed.
type LogSessions struct {
	mu       sync.Mutex
	channels map[string]chan []byte
}

func NewLogSessions() *LogSessions {
	return &LogSessions{
		channels: make(map[string]chan []byte),
	}
}

// Register creates a buffered channel for a log session.
func (ls *LogSessions) Register(requestID string) chan []byte {
	ch := make(chan []byte, 64)
	ls.mu.Lock()
	ls.channels[requestID] = ch
	ls.mu.Unlock()
	return ch
}

// Deliver sends data to the channel for the given session.
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
		// Channel full — drop data to prevent blocking
		return false
	}
}

// Remove closes and deletes a log session.
func (ls *LogSessions) Remove(requestID string) {
	ls.mu.Lock()
	if ch, ok := ls.channels[requestID]; ok {
		close(ch)
		delete(ls.channels, requestID)
	}
	ls.mu.Unlock()
}
