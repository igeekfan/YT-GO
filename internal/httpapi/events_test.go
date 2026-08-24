package httpapi

import "testing"

func TestEventHubDisconnectsSlowSubscriberInsteadOfDroppingEvents(t *testing.T) {
	hub := NewEventHub()
	ch := hub.Subscribe()

	for i := 0; i < cap(ch); i++ {
		hub.Emit("download:update", i)
	}
	hub.Emit("download:remove", "task-id")

	hub.mu.RLock()
	subscriberCount := len(hub.subscribers)
	hub.mu.RUnlock()
	if subscriberCount != 0 {
		t.Fatalf("slow subscriber remained registered after overflow: %d", subscriberCount)
	}

	received := 0
	for range ch {
		received++
	}
	if received != cap(ch) {
		t.Fatalf("received %d buffered events, want %d", received, cap(ch))
	}

	// The handler's deferred cleanup must remain safe after Emit removed and
	// closed the subscriber.
	hub.Unsubscribe(ch)
}
