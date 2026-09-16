package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kampong/debate/internal/models"
)

// JSONStore persists debate sessions as JSON files in a directory.
//
// Implements Session + EventStore (events are appended to a sidecar JSONL file
// per debate; safe enough for the JSON backend).
type JSONStore struct {
	dir string
	mu  sync.RWMutex
}

// NewSessionStore creates a JSON session store at the given directory.
//
// Retained name for backward compat with existing callers; returns the
// Session interface so callers can swap to SQLite later without changing
// call sites.
func NewSessionStore(dir string) (*JSONStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	return &JSONStore{dir: dir}, nil
}

// Compile-time check: *JSONStore satisfies Session and EventStore.
var (
	_ Session     = (*JSONStore)(nil)
	_ EventStore  = (*JSONStore)(nil)
)

// Save persists a debate session to a JSON file.
func (s *JSONStore) Save(session *models.DebateSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// Load reads a debate session from a JSON file. Returns (nil, nil) when the
// session does not exist (back-compat for old callers). Callers that need to
// distinguish missing-from-empty should call LoadStrict.
func (s *JSONStore) Load(id string) (*models.DebateSession, error) {
	session, err := s.LoadStrict(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

// LoadStrict is like Load but returns ErrNotFound when the file is missing.
func (s *JSONStore) LoadStrict(id string) (*models.DebateSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	safeID := sanitizeID(id)
	path := filepath.Join(s.dir, safeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session models.DebateSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &session, nil
}

// List returns metadata for all saved debates, newest first.
func (s *JSONStore) List() ([]SessionMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read store dir: %w", err)
	}

	var metas []SessionMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var session models.DebateSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		metas = append(metas, SessionMeta{
			ID:         session.ID,
			Topic:      session.Topic,
			Mode:       string(session.Mode),
			Status:     string(session.Status),
			Agents:     len(session.Agents),
			Rounds:     session.Round,
			CreatedAt:  session.CreatedAt,
			HasVerdict: session.Verdict != nil,
		})
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})

	return metas, nil
}

// Delete removes a saved session and its event sidecar.
func (s *JSONStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	safeID := sanitizeID(id)
	sessionPath := filepath.Join(s.dir, safeID+".json")
	eventsPath := filepath.Join(s.dir, safeID+".events.jsonl")

	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(eventsPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// AppendEvent writes a single event line to the per-debate sidecar.
func (s *JSONStore) AppendEvent(ev Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	safeID := sanitizeID(ev.DebateID)
	path := filepath.Join(s.dir, safeID+".events.jsonl")
	payload := ev.Payload
	if payload == nil {
		payload = []byte("{}")
	}
	rec := struct {
		Seq       int64           `json:"seq"`
		EventType string          `json:"event_type"`
		Payload   json.RawMessage `json:"payload"`
		CreatedAt time.Time       `json:"created_at"`
	}{
		Seq:       ev.Seq,
		EventType: ev.EventType,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// EventsAfter returns events for a debate with seq > fromSeq, oldest first.
func (s *JSONStore) EventsAfter(debateID string, fromSeq int64) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	safeID := sanitizeID(debateID)
	path := filepath.Join(s.dir, safeID+".events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []Event
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var rec struct {
			Seq       int64           `json:"seq"`
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.Seq <= fromSeq {
			continue
		}
		out = append(out, Event{
			DebateID:  debateID,
			Seq:       rec.Seq,
			EventType: rec.EventType,
			Payload:   rec.Payload,
		})
	}
	return out, nil
}

func sanitizeID(id string) string {
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "\\", "_")
	id = strings.ReplaceAll(id, "..", "_")
	return id
}