package grpcserver

import (
	"sync"

	"google.golang.org/protobuf/proto"
)

// PendingRequests tracks in-flight request-response correlations.
type PendingRequests struct {
	mu       sync.Mutex
	channels map[string]chan proto.Message
}

func NewPendingRequests() *PendingRequests {
	return &PendingRequests{
		channels: make(map[string]chan proto.Message),
	}
}

func (p *PendingRequests) Register(requestID string) chan proto.Message {
	ch := make(chan proto.Message, 1)
	p.mu.Lock()
	p.channels[requestID] = ch
	p.mu.Unlock()
	return ch
}

func (p *PendingRequests) Deliver(requestID string, msg proto.Message) bool {
	p.mu.Lock()
	ch, ok := p.channels[requestID]
	p.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- msg:
		return true
	default:
		return false
	}
}

func (p *PendingRequests) Remove(requestID string) {
	p.mu.Lock()
	delete(p.channels, requestID)
	p.mu.Unlock()
}
