package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type eventMessage struct {
	Name string
	Data any
}

type EventHub struct {
	mu          sync.RWMutex
	subscribers map[chan eventMessage]struct{}
	closed      bool
}

func NewEventHub() *EventHub {
	return &EventHub{subscribers: make(map[chan eventMessage]struct{})}
}

func (h *EventHub) Subscribe() chan eventMessage {
	ch := make(chan eventMessage, 32)
	h.mu.Lock()
	if h.closed {
		close(ch)
	} else {
		h.subscribers[ch] = struct{}{}
	}
	h.mu.Unlock()
	return ch
}

// Close disconnects all subscribers and prevents new subscriptions. It is
// idempotent and lets http.Server.Shutdown drain long-lived SSE handlers.
func (h *EventHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.subscribers {
		delete(h.subscribers, ch)
		close(ch)
	}
}

func (h *EventHub) Unsubscribe(ch chan eventMessage) {
	h.mu.Lock()
	if _, ok := h.subscribers[ch]; ok {
		delete(h.subscribers, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *EventHub) Emit(name string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers {
		select {
		case ch <- eventMessage{Name: name, Data: data}:
		default:
			// A full channel means this subscriber can no longer receive a
			// lossless event sequence. Disconnect it instead of silently
			// dropping lifecycle events; EventSource will reconnect and the
			// frontend reconciles from the current download snapshot on open.
			delete(h.subscribers, ch)
			close(ch)
		}
	}
}

func (h *EventHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.Subscribe()
	defer h.Unsubscribe(ch)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(msg.Data)
			if err != nil {
				payload = []byte("null")
			}
			_, _ = fmt.Fprintf(w, "event: %s\n", msg.Name)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
