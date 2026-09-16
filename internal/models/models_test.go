package models

import (
	"sync"
	"testing"
	"time"
)

func TestDebateSession_AppendTranscript(t *testing.T) {
	s := &DebateSession{ID: "test-1", Topic: "test"}
	entry := TranscriptEntry{
		ID:        "e1",
		AgentName: "Test Agent",
		Text:      "Hello world",
		Timestamp: time.Now(),
	}

	s.AppendTranscript(entry)

	transcript := s.GetTranscript()
	if len(transcript) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(transcript))
	}
	if transcript[0].Text != "Hello world" {
		t.Errorf("expected 'Hello world', got '%s'", transcript[0].Text)
	}
}

func TestDebateSession_GetRecentTranscript(t *testing.T) {
	s := &DebateSession{ID: "test-2"}
	for i := 0; i < 10; i++ {
		s.AppendTranscript(TranscriptEntry{ID: string(rune('a' + i)), Text: "entry"})
	}

	recent := s.GetRecentTranscript(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent entries, got %d", len(recent))
	}
	if recent[0].ID != "h" {
		t.Errorf("expected first recent to be ID 'h', got '%s'", recent[0].ID)
	}
	if recent[2].ID != "j" {
		t.Errorf("expected last recent to be ID 'j', got '%s'", recent[2].ID)
	}
}

func TestDebateSession_GetRecentTranscript_Empty(t *testing.T) {
	s := &DebateSession{ID: "test-3"}
	recent := s.GetRecentTranscript(5)
	if recent != nil {
		t.Errorf("expected nil for empty transcript, got %v", recent)
	}
}

func TestDebateSession_AppendGraphUpdate(t *testing.T) {
	s := &DebateSession{ID: "test-4"}
	update := GraphUpdate{
		NewNodes: []GraphNode{
			{ID: "n1", Label: "Node 1", Type: "claim"},
			{ID: "n2", Label: "Node 2", Type: "concept"},
		},
		NewEdges: []GraphEdge{
			{From: "n1", To: "n2", Relation: "supports"},
		},
	}
	s.AppendGraphUpdate(update)

	if len(s.Graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(s.Graph.Nodes))
	}
	if len(s.Graph.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(s.Graph.Edges))
	}

	// Deduplication: add same node again
	s.AppendGraphUpdate(GraphUpdate{
		NewNodes: []GraphNode{{ID: "n1", Label: "Node 1", Type: "claim"}},
	})
	if len(s.Graph.Nodes) != 2 {
		t.Errorf("expected still 2 nodes after dedup, got %d", len(s.Graph.Nodes))
	}
}

func TestDebateSession_Concurrency(t *testing.T) {
	s := &DebateSession{ID: "test-concurrent"}
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.AppendTranscript(TranscriptEntry{ID: string(rune('a' + id%26)), Text: "concurrent"})
		}(i)
	}

	wg.Wait()
	transcript := s.GetTranscript()
	if len(transcript) != 50 {
		t.Errorf("expected 50 entries, got %d", len(transcript))
	}
}

func TestSetStatus(t *testing.T) {
	s := &DebateSession{ID: "test-status"}
	s.SetStatus(StatusOpening)
	if s.Status != StatusOpening {
		t.Errorf("expected opening, got %s", s.Status)
	}
	s.SetStatus(StatusVerdict)
	if s.Status != StatusVerdict {
		t.Errorf("expected verdict, got %s", s.Status)
	}
}

func TestSetVerdict(t *testing.T) {
	s := &DebateSession{ID: "test-verdict"}
	v := &Verdict{
		WinnerAgentID: "agent-1",
		Reasoning:     "Test reasoning",
	}
	s.SetVerdict(v)
	if s.Verdict == nil {
		t.Fatal("expected non-nil verdict")
	}
	if s.Verdict.WinnerAgentID != "agent-1" {
		t.Errorf("expected winner agent-1, got %s", s.Verdict.WinnerAgentID)
	}
}
