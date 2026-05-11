package services

import (
	"log"
	"sync"
)

// maxPendingEvents is the maximum number of important events buffered per user
// when they have no active subscribers (e.g. between disconnect and reconnect).
const maxPendingEvents = 50

// importantEventTypes are the event types worth buffering for offline users.
// Transient status updates (daemon_status, daemon_tool_call) are not buffered.
var importantEventTypes = map[string]bool{
	"cortex_response":      true,
	"cortex_thinking":      true,
	"task_created":         true,
	"task_completed":       true,
	"task_failed":          true,
	"daemon_deployed":      true,
	"daemon_completed":     true,
	"daemon_failed":        true,
	"error":                true,
	"bridge_state_updated": true,
}

// NexusEventBus is an in-memory pub/sub for Nexus events, scoped per user.
// It decouples daemon execution from WebSocket lifecycle — daemons publish events
// here, and any connected WS client subscribes.
//
// Important events (cortex_response, task_completed, etc.) are buffered per-user
// when no subscriber is connected. On reconnect, pending events are drained to
// the new subscriber so proactive messages are never lost.
type NexusEventBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[string]chan NexusEvent // userID → subID → chan
	pending     map[string][]NexusEvent              // userID → buffered important events
}

// NewNexusEventBus creates a new event bus
func NewNexusEventBus() *NexusEventBus {
	return &NexusEventBus{
		subscribers: make(map[string]map[string]chan NexusEvent),
		pending:     make(map[string][]NexusEvent),
	}
}

// Subscribe creates a new event channel for a user. Returns a receive-only channel.
// Pending events are NOT auto-drained — call DrainPending() separately so the
// WebSocket handler can format them as a structured "missed_updates" message.
func (b *NexusEventBus) Subscribe(userID, subID string, bufSize int) <-chan NexusEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan NexusEvent, bufSize)
	if _, ok := b.subscribers[userID]; !ok {
		b.subscribers[userID] = make(map[string]chan NexusEvent)
	}
	b.subscribers[userID][subID] = ch

	count := len(b.subscribers[userID])
	log.Printf("[EVENT-BUS] Subscribe: user=%s sub=%s (total=%d)", userID, subID, count)

	return ch
}

// DrainPending returns and clears all buffered events for a user.
// Called by the WebSocket handler on reconnect to build "missed_updates".
func (b *NexusEventBus) DrainPending(userID string) []NexusEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	events := b.pending[userID]
	delete(b.pending, userID)

	if len(events) > 0 {
		log.Printf("[EVENT-BUS] Drained %d pending events for user %s", len(events), userID)
	}
	return events
}

// Unsubscribe removes a subscription. The channel is NOT closed — the subscriber's
// goroutine should exit via its own done signal, and the channel will be GC'd.
func (b *NexusEventBus) Unsubscribe(userID, subID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if conns, ok := b.subscribers[userID]; ok {
		delete(conns, subID)
		if len(conns) == 0 {
			delete(b.subscribers, userID)
		}
		log.Printf("[EVENT-BUS] Unsubscribe: user=%s sub=%s (remaining=%d)", userID, subID, len(conns))
	}
}

// Publish sends an event to all subscribers for a user. Non-blocking — if a
// subscriber's channel is full, the event is dropped for that subscriber.
//
// If no subscribers are connected and the event is an important type
// (cortex_response, task_completed, etc.), it is buffered in a per-user
// pending queue so it can be delivered on reconnect.
func (b *NexusEventBus) Publish(userID string, event NexusEvent) {
	b.mu.RLock()
	conns, hasSubscribers := b.subscribers[userID]

	if hasSubscribers && len(conns) > 0 {
		delivered := false
		for _, ch := range conns {
			select {
			case ch <- event:
				delivered = true
			default:
				// Subscriber is full — skip this one
			}
		}
		b.mu.RUnlock()

		// If all subscribers were full and this is important, buffer it
		if !delivered && importantEventTypes[event.Type] {
			b.bufferEvent(userID, event)
		}
		return
	}
	b.mu.RUnlock()

	// No subscribers — buffer important events for when user reconnects
	if importantEventTypes[event.Type] {
		b.bufferEvent(userID, event)
	}
}

// bufferEvent adds an important event to the user's pending queue.
// Evicts oldest events if the buffer exceeds maxPendingEvents.
func (b *NexusEventBus) bufferEvent(userID string, event NexusEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.pending[userID] = append(b.pending[userID], event)

	// Cap the buffer — evict oldest if over limit
	if len(b.pending[userID]) > maxPendingEvents {
		b.pending[userID] = b.pending[userID][len(b.pending[userID])-maxPendingEvents:]
	}

	log.Printf("[EVENT-BUS] Buffered event type=%s for offline user %s (pending=%d)",
		event.Type, userID, len(b.pending[userID]))
}

// SubscriberCount returns the number of active subscribers for a user
func (b *NexusEventBus) SubscriberCount(userID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if conns, ok := b.subscribers[userID]; ok {
		return len(conns)
	}
	return 0
}

// PendingCount returns the number of buffered events for a disconnected user
func (b *NexusEventBus) PendingCount(userID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.pending[userID])
}
