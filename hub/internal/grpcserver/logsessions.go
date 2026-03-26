package grpcserver

import "sync"

// LogSessions tracks active log tailing sessions.
// Unlike PendingRequests (one-shot), log sessions deliver multiple
// chunks over time until explicitly removed.
// Keys are scoped per server (serverID:requestID) to prevent a
// compromised agent from injecting data into another server's log stream.
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
func (ls *LogSessions) Register(serverID, requestID string) chan []byte {
	ch := make(chan []byte, 64)
	ls.mu.Lock()
	ls.channels[makeKey(serverID, requestID)] = ch
	ls.mu.Unlock()
	return ch
}

// Deliver sends data to the channel for the given session.
func (ls *LogSessions) Deliver(serverID, requestID string, data []byte) bool {
	key := makeKey(serverID, requestID)
	ls.mu.Lock()
	ch, ok := ls.channels[key]
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
func (ls *LogSessions) Remove(serverID, requestID string) {
	key := makeKey(serverID, requestID)
	ls.mu.Lock()
	if ch, ok := ls.channels[key]; ok {
		close(ch)
		delete(ls.channels, key)
	}
	ls.mu.Unlock()
}
