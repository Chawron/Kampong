package ws

import (
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"

	"github.com/kampong/debate/internal/store"
)

// Human-in-the-loop WebSocket event types (Pillar 2):
//
//	"debate_paused"     — data: {reason: string}
//	"debate_resumed"    — data: {}
//	"user_injection"    — data: {type, text, target_agent_id, timestamp, injection_id}
//	"claim_challenged"  — data: {entry_id, challenge_id, challenge_text, challenged_agent, timestamp}

// Event is a WebSocket message sent from the server to clients.
type Event struct {
	Event    string      `json:"event"`
	DebateID string      `json:"debate_id"`
	Seq      int64       `json:"seq,omitempty"`
	Data     interface{} `json:"data"`
}

// Client represents a single WebSocket connection subscribed to a debate.
type Client struct {
	debateID string
	send     chan []byte
}

// Hub manages WebSocket connections per debate session.
type Hub struct {
	mu       sync.RWMutex
	clients  map[string]map[*Client]bool // debateID -> set of clients
	seq      atomic.Int64                // monotonic per-hub sequence counter
	store    store.EventStore             // optional persistence of broadcast events
}

// NewHub creates a WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]bool),
	}
}

// SetEventStore wires an optional persistence backend so Broadcast events
// can be replayed to late-joining clients.
func (h *Hub) SetEventStore(s store.EventStore) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.store = s
}

// Subscribe adds a client to a debate session's broadcast list.
func (h *Hub) Subscribe(debateID string, send chan []byte) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := &Client{debateID: debateID, send: send}

	if h.clients[debateID] == nil {
		h.clients[debateID] = make(map[*Client]bool)
	}
	h.clients[debateID][client] = true

	return client
}

// Unsubscribe removes a client from a debate session.
func (h *Hub) Unsubscribe(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.debateID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, client.debateID)
		}
	}
}

// Broadcast sends an event to all clients subscribed to a debate.
//
// When an EventStore is attached, the marshalled event is also persisted so
// late-joining clients can fetch the history via ReplayFrom(debateID, fromSeq).
func (h *Hub) Broadcast(debateID string, event Event) {
	h.mu.RLock()
	clients := h.clients[debateID]
	es := h.store
	h.mu.RUnlock()

	// Assign a monotonic sequence number for replay ordering.
	event.Seq = h.seq.Add(1)

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("WS MARSHAL ERROR: %v", err)
		return
	}

	// Best-effort persistence — don't block the live broadcast on a slow DB.
	if es != nil {
		ev := store.Event{
			DebateID:  debateID,
			Seq:       event.Seq,
			EventType: event.Event,
			Payload:   data,
		}
		if err := es.AppendEvent(ev); err != nil {
			log.Printf("WS PERSIST ERROR [%s seq=%d]: %v", debateID, event.Seq, err)
		}
	}

	if clients == nil {
		return
	}
	for client := range clients {
		select {
		case client.send <- data:
		default:
			// Client buffer full — drop message
			log.Printf("WS DROP: client buffer full for debate %s", debateID)
		}
	}
}

// SendToClient sends an event to a specific client.
func (h *Hub) SendToClient(client *Client, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("WS MARSHAL ERROR: %v", err)
		return
	}

	select {
	case client.send <- data:
	default:
	}
}

// ActiveClients returns the number of connected clients for a debate.
func (h *Hub) ActiveClients(debateID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[debateID])
}

// ReplayFrom emits all stored events for a debate with seq > fromSeq into the
// provided channel. Used by the WS handler to backfill clients that joined
// after the debate started.
//
// Returns the number of events replayed. No-op when no EventStore is attached.
func (h *Hub) ReplayFrom(debateID string, fromSeq int64, send chan []byte) (int, error) {
	h.mu.RLock()
	es := h.store
	h.mu.RUnlock()
	if es == nil {
		return 0, nil
	}
	events, err := es.EventsAfter(debateID, fromSeq)
	if err != nil {
		return 0, err
	}
	for _, e := range events {
		select {
		case send <- e.Payload:
		default:
			// Channel full — drop and stop; the client will fall back to the live feed.
			return len(events), nil
		}
	}
	return len(events), nil
}

// ReplayHistory sends the full debate history to a newly connected client.
// Deprecated: kept for the old interface; use ReplayFrom instead.
func (h *Hub) ReplayHistory(client *Client, session interface {
	GetTranscript() interface{}
}) {
	// This is called when a client reconnects mid-debate.
	// The actual replay logic is in the WS handler since it has access to the session store.
}