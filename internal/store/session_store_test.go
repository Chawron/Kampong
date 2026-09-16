package store

import (
	"os"
	"testing"
	"time"

	"github.com/kampong/debate/internal/models"
)

func TestSessionStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	session := &models.DebateSession{
		ID:        "test-123",
		Topic:     "Chicken or egg",
		Mode:      models.ModeDeep,
		Status:    models.StatusVerdict,
		Round:     3,
		TotalRounds: 5,
		Agents:    make([]models.Agent, 0),
		CreatedAt: time.Now(),
	}

	if err := s.Save(session); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.Load("test-123")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected session, got nil")
	}
	if loaded.Topic != "Chicken or egg" {
		t.Errorf("expected 'Chicken or egg', got %q", loaded.Topic)
	}
}

func TestSessionStore_LoadNotFound(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewSessionStore(dir)

	loaded, err := s.Load("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatal("expected nil for missing session")
	}
}

func TestSessionStore_List(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewSessionStore(dir)

	s.Save(&models.DebateSession{
		ID: "a", Topic: "Topic A", Mode: models.ModeQuick,
		Status: models.StatusVerdict, CreatedAt: time.Now().Add(-1 * time.Hour),
	})
	s.Save(&models.DebateSession{
		ID: "b", Topic: "Topic B", Mode: models.ModeDeep,
		Status: models.StatusError, CreatedAt: time.Now(),
	})

	metas, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 debates, got %d", len(metas))
	}
	if metas[0].ID != "b" {
		t.Errorf("expected newest first, got %q first", metas[0].ID)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewSessionStore(dir)

	s.Save(&models.DebateSession{ID: "x", Topic: "X", Mode: models.ModeQuick, Status: models.StatusVerdict, CreatedAt: time.Now()})

	if err := s.Delete("x"); err != nil {
		t.Fatal(err)
	}

	loaded, _ := s.Load("x")
	if loaded != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestSessionStore_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewSessionStore(dir)

	// Attempt path traversal should be sanitized
	loaded, err := s.Load("../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatal("expected nil for path traversal attempt")
	}

	// Delete is idempotent — deleting a non-existent session returns nil
	// rather than an error so callers can retry safely.
	if err := s.Delete("../../something"); err != nil {
		t.Fatalf("expected nil for idempotent delete, got %v", err)
	}

	// Verify no files outside store dir
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("expected empty store dir after failed operations")
	}
}
