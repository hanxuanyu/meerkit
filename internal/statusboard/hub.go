package statusboard

import "sync"

type StreamEvent struct {
	Type      string       `json:"type"`
	MonitorID string       `json:"monitor_id,omitempty"`
	RecordID  string       `json:"record_id,omitempty"`
	Items     []StreamItem `json:"items,omitempty"`
	ItemID    string       `json:"item_id,omitempty"`
}

type StreamItem struct {
	ItemID string `json:"item_id"`
	Sample any    `json:"sample,omitempty"`
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan StreamEvent]struct{}
}

func NewHub() *Hub { return &Hub{subscribers: map[chan StreamEvent]struct{}{}} }

func (h *Hub) Subscribe() (<-chan StreamEvent, func()) {
	stream := make(chan StreamEvent, 32)
	h.mu.Lock()
	h.subscribers[stream] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return stream, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subscribers, stream)
			close(stream)
			h.mu.Unlock()
		})
	}
}

func (h *Hub) Publish(event StreamEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}
