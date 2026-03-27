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
	overflow map[string]uint64
}

func NewLogSessions() *LogSessions {
	return &LogSessions{
		channels: make(map[string]chan []byte),
		overflow: make(map[string]uint64),
	}
}

// Register creates a buffered channel for a log session.
func (ls *LogSessions) Register(serverID, requestID string) chan []byte {
	ch := make(chan []byte, 64)
	key := makeKey(serverID, requestID)
	ls.mu.Lock()
	ls.channels[key] = ch
	ls.overflow[key] = 0
	ls.mu.Unlock()
	return ch
}

// Deliver sends data to the channel for the given session.
// When the channel buffer is full, the data is dropped and an internal
// overflow counter is incremented so consumers can detect data loss.
func (ls *LogSessions) Deliver(serverID, requestID string, data []byte) bool {
	key := makeKey(serverID, requestID)
	ls.mu.Lock()
	ch, ok := ls.channels[key]
	if !ok {
		ls.mu.Unlock()
		return false
	}
	select {
	case ch <- data:
		ls.mu.Unlock()
		return true
	default:
		// Channel full — drop data to prevent blocking, track the loss
		ls.overflow[key]++
		ls.mu.Unlock()
		return false
	}
}

// DrainOverflow returns the number of log chunks dropped since the last
// call for this session and resets the counter to zero. Consumers should
// call this periodically (e.g. after reading from the channel) to detect
// and report data loss to the client.
func (ls *LogSessions) DrainOverflow(serverID, requestID string) uint64 {
	key := makeKey(serverID, requestID)
	ls.mu.Lock()
	n := ls.overflow[key]
	if n > 0 {
		ls.overflow[key] = 0
	}
	ls.mu.Unlock()
	return n
}

// Remove closes and deletes a log session.
func (ls *LogSessions) Remove(serverID, requestID string) {
	key := makeKey(serverID, requestID)
	ls.mu.Lock()
	if ch, ok := ls.channels[key]; ok {
		close(ch)
		delete(ls.channels, key)
		delete(ls.overflow, key)
	}
	ls.mu.Unlock()
}
