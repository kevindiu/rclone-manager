package events

import "sync"

// Event represents a live event broadcast to connected SSE subscribers.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Hub manages live Server-Sent Events subscribers safely across goroutines.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

// NewHub creates a new SSE event Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan Event]struct{}),
	}
}

// Subscribe registers a new client channel and returns an unsubscribe cleanup function.
func (h *Hub) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	cleanup := func() {
		h.mu.Lock()
		if _, ok := h.clients[ch]; ok {
			delete(h.clients, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
	return ch, cleanup
}

// Broadcast sends an event to all connected subscribers without blocking slow clients.
func (h *Hub) Broadcast(eventType string, data any) {
	ev := Event{Type: eventType, Data: data}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
		}
	}
}
