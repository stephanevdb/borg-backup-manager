package stream

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Hub struct {
	mu        sync.RWMutex
	listeners map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{listeners: make(map[string]map[chan []byte]struct{})}
}

func (h *Hub) Subscribe(topic string) chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	if h.listeners[topic] == nil {
		h.listeners[topic] = make(map[chan []byte]struct{})
	}
	h.listeners[topic][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(topic string, ch chan []byte) {
	h.mu.Lock()
	if subs, ok := h.listeners[topic]; ok {
		delete(subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *Hub) Publish(topic string, event string, data any) {
	payload, _ := json.Marshal(data)
	msg := []byte("event: " + event + "\ndata: " + string(payload) + "\n\n")
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.listeners[topic] {
		select {
		case ch <- msg:
		default:
		}
	}
}

func ServeSSE(w http.ResponseWriter, r *http.Request, ch <-chan []byte) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg, open := <-ch:
			if !open {
				return
			}
			_, _ = w.Write(msg)
			flusher.Flush()
		}
	}
}
