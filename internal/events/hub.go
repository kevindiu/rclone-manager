package events

import "sync"

// Event represents a live event broadcast to connected SSE subscribers.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Hub manages live Server-Sent Events subscribers safely across goroutines.
type Hub struct {
	mu      sync.Mutex
	closed  bool
	clients map[chan Event]struct{}
}

// NewHub creates a new SSE event Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan Event]struct{}),
	}
}

// Subscribe registers a new client channel and returns an unsubscribe cleanup function.
// If the hub is already closed, the returned channel is closed immediately.
func (h *Hub) Subscribe() (chan Event, func()) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}
	ch := make(chan Event, 64)
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

// Broadcast sends an event to all connected subscribers. Slow subscribers whose
// buffer is full are evicted and closed so they reconnect with a fresh snapshot.
func (h *Hub) Broadcast(eventType string, data any) {
	ev := Event{Type: eventType, Data: data}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
			delete(h.clients, ch)
			close(ch)
		}
	}
}

// Close disconnects all active SSE subscribers so HTTP shutdown does not stall
// waiting for long-lived streams. Calling Close more than once is a no-op.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.clients {
		delete(h.clients, ch)
		close(ch)
	}
}
