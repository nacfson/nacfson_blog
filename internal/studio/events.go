package studio

import "sync"

// Hub fans git Events out to Server-Sent Events subscribers and keeps a
// short history so a freshly opened tab shows recent activity.
type Hub struct {
	mu      sync.Mutex
	history []Event
	limit   int
	subs    map[chan Event]struct{}
}

// NewHub keeps the last limit events.
func NewHub(limit int) *Hub {
	return &Hub{limit: limit, subs: map[chan Event]struct{}{}}
}

// Publish records e and delivers it to subscribers without blocking.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history = append(h.history, e)
	if len(h.history) > h.limit {
		h.history = h.history[len(h.history)-h.limit:]
	}
	for ch := range h.subs {
		select {
		case ch <- e:
		default: // slow subscriber: drop rather than stall git
		}
	}
}

// Subscribe returns recent history and a channel for new events.
func (h *Hub) Subscribe() ([]Event, chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan Event, 64)
	h.subs[ch] = struct{}{}
	hist := append([]Event(nil), h.history...)
	return hist, ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}
}
