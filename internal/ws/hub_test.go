package ws

import (
	"testing"
	"time"
)

func TestHub_SubscribeUnsubscribe(t *testing.T) {
	h := NewHub()
	send := make(chan []byte, 10)
	client := h.Subscribe("debate-1", send)

	if h.ActiveClients("debate-1") != 1 {
		t.Errorf("expected 1 client, got %d", h.ActiveClients("debate-1"))
	}

	h.Unsubscribe(client)

	if h.ActiveClients("debate-1") != 0 {
		t.Errorf("expected 0 clients after unsubscribe, got %d", h.ActiveClients("debate-1"))
	}
}

func TestHub_Broadcast(t *testing.T) {
	h := NewHub()
	send := make(chan []byte, 10)
	h.Subscribe("debate-2", send)

	h.Broadcast("debate-2", Event{
		Event:    "test",
		DebateID: "debate-2",
		Data:     map[string]string{"msg": "hello"},
	})

	select {
	case msg := <-send:
		if len(msg) == 0 {
			t.Error("expected non-empty message")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for broadcast message")
	}
}

func TestHub_BroadcastNoClients(t *testing.T) {
	h := NewHub()
	// Should not panic when broadcasting to no clients
	h.Broadcast("nonexistent", Event{
		Event:    "test",
		DebateID: "nonexistent",
	})
}

func TestHub_MultipleClients(t *testing.T) {
	h := NewHub()
	send1 := make(chan []byte, 10)
	send2 := make(chan []byte, 10)
	h.Subscribe("debate-3", send1)
	h.Subscribe("debate-3", send2)

	if h.ActiveClients("debate-3") != 2 {
		t.Errorf("expected 2 clients, got %d", h.ActiveClients("debate-3"))
	}
}

func TestHub_DifferentDebates(t *testing.T) {
	h := NewHub()
	send1 := make(chan []byte, 10)
	send2 := make(chan []byte, 10)
	h.Subscribe("debate-a", send1)
	h.Subscribe("debate-b", send2)

	if h.ActiveClients("debate-a") != 1 {
		t.Errorf("expected 1 client for debate-a")
	}
	if h.ActiveClients("debate-b") != 1 {
		t.Errorf("expected 1 client for debate-b")
	}

	// Broadcast to debate-a should not reach debate-b
	h.Broadcast("debate-a", Event{Event: "test", DebateID: "debate-a"})

	select {
	case <-send1:
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Error("debate-a client should have received message")
	}

	select {
	case <-send2:
		t.Error("debate-b client should NOT have received message")
	case <-time.After(50 * time.Millisecond):
		// expected — no message
	}
}

func TestSendToClient(t *testing.T) {
	h := NewHub()
	send := make(chan []byte, 10)
	client := h.Subscribe("debate-4", send)

	h.SendToClient(client, Event{Event: "direct", DebateID: "debate-4"})

	select {
	case msg := <-send:
		if len(msg) == 0 {
			t.Error("expected non-empty message")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for direct message")
	}
}
