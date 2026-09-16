package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kampong/debate/internal/models"
	_ "modernc.org/sqlite" // pure-Go SQLite driver — no CGO required
)

// SQLiteStore persists sessions, events, and per-agent calibration rolling
// averages in a single SQLite file. The driver is pure Go, so cross-compile
// stays trivial.
//
// Schema:
//   - sessions(id PK, topic, mode, status, created_at, payload BLOB)
//   - events(debate_id, seq, event_type, payload, created_at, PK(debate_id,seq))
//   - calibrations(agent_id PK, debate_count, sum_confidence, sum_judge_score,
//     updated_at)
type SQLiteStore struct {
	db   *sql.DB
	path string
	mu   sync.Mutex // serialises schema-touching ops
}

// Compile-time interface checks.
var (
	_ Session         = (*SQLiteStore)(nil)
	_ EventStore      = (*SQLiteStore)(nil)
	_ CalibrationStore = (*SQLiteStore)(nil)
)

// NewSQLiteStore opens (and initialises) the SQLite database at path.
// Creates parent directories as needed.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, errors.New("sqlite path is empty")
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
	}

	// _journal=WAL + _busy_timeout cut down on "database is locked" errors
	// when an agent thread and the WebSocket broadcast both write at once.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &SQLiteStore{db: db, path: path}
	if err := s.initSchema(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			topic TEXT NOT NULL DEFAULT '',
			mode TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			payload BLOB NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at)`,
		`CREATE TABLE IF NOT EXISTS events (
			debate_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			payload BLOB NOT NULL,
			created_at DATETIME NOT NULL,
			PRIMARY KEY (debate_id, seq)
		)`,
		`CREATE TABLE IF NOT EXISTS calibrations (
			agent_id TEXT PRIMARY KEY,
			debate_count INTEGER NOT NULL DEFAULT 0,
			sum_confidence REAL NOT NULL DEFAULT 0,
			sum_judge_score REAL NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("schema %q: %w", q[:30], err)
		}
	}
	return nil
}

// Close flushes and closes the underlying database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Path returns the on-disk path (handy for "Where is my data?" debugging).
func (s *SQLiteStore) Path() string {
	return s.path
}

// Save persists the session as a JSON blob plus searchable columns.
func (s *SQLiteStore) Save(session *models.DebateSession) error {
	if session == nil {
		return errors.New("nil session")
	}
	// Snapshot under the session lock so a concurrent engine goroutine
	// (post-debate analysis, social simulation) can't race the marshal.
	snap := session.Snapshot()
	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	_, err = s.db.ExecContext(context.Background(),
		`INSERT INTO sessions(id, topic, mode, status, created_at, payload)
		 VALUES(?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   topic=excluded.topic,
		   mode=excluded.mode,
		   status=excluded.status,
		   payload=excluded.payload`,
		session.ID, session.Topic, string(snap.Mode), string(snap.Status),
		snap.CreatedAt, payload,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// Load reads a session. Returns ErrNotFound when the row does not exist.
func (s *SQLiteStore) Load(id string) (*models.DebateSession, error) {
	var payload []byte
	err := s.db.QueryRowContext(context.Background(),
		`SELECT payload FROM sessions WHERE id = ?`, id,
	).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("select session: %w", err)
	}
	var session models.DebateSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

// List returns metadata for all sessions, newest first.
func (s *SQLiteStore) List() ([]SessionMeta, error) {
	rows, err := s.db.QueryContext(context.Background(),
		`SELECT id, topic, mode, status, created_at, payload
		 FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var metas []SessionMeta
	for rows.Next() {
		var (
			id, topic, mode, status string
			createdAt               time.Time
			payload                 []byte
		)
		if err := rows.Scan(&id, &topic, &mode, &status, &createdAt, &payload); err != nil {
			return nil, err
		}

		// Peek into the payload for agent count + verdict presence rather than
		// duplicating those into separate columns.
		agentCount, hasVerdict := 0, false
		var probe struct {
			Agents  []json.RawMessage `json:"agents"`
			Verdict json.RawMessage   `json:"verdict"`
			Round   int               `json:"round"`
		}
		if err := json.Unmarshal(payload, &probe); err == nil {
			agentCount = len(probe.Agents)
			hasVerdict = len(probe.Verdict) > 0
		}

		metas = append(metas, SessionMeta{
			ID:         id,
			Topic:      topic,
			Mode:       mode,
			Status:     status,
			Agents:     agentCount,
			Rounds:     probe.Round,
			CreatedAt:  createdAt,
			HasVerdict: hasVerdict,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return metas, nil
}

// Delete removes a session and its events.
func (s *SQLiteStore) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM events WHERE debate_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// AppendEvent inserts an event. Seq must be monotonic per debate but this
// method does not enforce that — the WS hub owns sequencing.
func (s *SQLiteStore) AppendEvent(ev Event) error {
	payload := ev.Payload
	if payload == nil {
		payload = []byte("{}")
	}
	_, err := s.db.ExecContext(context.Background(),
		`INSERT OR REPLACE INTO events(debate_id, seq, event_type, payload, created_at)
		 VALUES(?, ?, ?, ?, ?)`,
		ev.DebateID, ev.Seq, ev.EventType, payload, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

// EventsAfter returns events for a debate with seq > fromSeq, ordered by seq.
func (s *SQLiteStore) EventsAfter(debateID string, fromSeq int64) ([]Event, error) {
	rows, err := s.db.QueryContext(context.Background(),
		`SELECT seq, event_type, payload
		 FROM events
		 WHERE debate_id = ? AND seq > ?
		 ORDER BY seq ASC`,
		debateID, fromSeq,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.Seq, &ev.EventType, &ev.Payload); err != nil {
			return nil, err
		}
		ev.DebateID = debateID
		out = append(out, ev)
	}
	return out, rows.Err()
}

// Get returns rolling averages for the given agent. ok=false when no
// history has been recorded.
func (s *SQLiteStore) Get(agentID string) (float64, float64, int, bool) {
	var (
		debateCount int
		sumConf     float64
		sumJudge    float64
	)
	err := s.db.QueryRowContext(context.Background(),
		`SELECT debate_count, sum_confidence, sum_judge_score
		 FROM calibrations WHERE agent_id = ?`, agentID,
	).Scan(&debateCount, &sumConf, &sumJudge)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, 0, false
		}
		return 0, 0, 0, false
	}
	avgConf := sumConf / float64(debateCount)
	avgJudge := sumJudge / float64(debateCount)
	return avgConf, avgJudge, debateCount, true
}

// Update folds a new (confidence, judgeScore) sample into the rolling average.
func (s *SQLiteStore) Update(agentID string, confidence, judgeScore float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO calibrations(agent_id, debate_count, sum_confidence, sum_judge_score, updated_at)
		 VALUES(?, 1, ?, ?, ?)
		 ON CONFLICT(agent_id) DO UPDATE SET
		   debate_count = debate_count + 1,
		   sum_confidence = sum_confidence + excluded.sum_confidence,
		   sum_judge_score = sum_judge_score + excluded.sum_judge_score,
		   updated_at = excluded.updated_at`,
		agentID, confidence, judgeScore, time.Now().UTC(),
	)
	return err
}

// ImportJSON imports existing JSON files from dir. Runs once at startup to
// preserve data/debates/*.json across the migration. Safe to call repeatedly:
// it skips sessions that are already present.
func (s *SQLiteStore) ImportJSON(dir string) (imported, skipped int, err error) {
	if dir == "" {
		return 0, 0, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	for _, e := range entries {
		if e.IsDir() || !endsWithJSON(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			skipped++
			continue
		}
		var session models.DebateSession
		if err := json.Unmarshal(data, &session); err != nil {
			skipped++
			continue
		}
		// Skip if a row already exists for this id.
		var exists int
		err = s.db.QueryRow(`SELECT 1 FROM sessions WHERE id = ?`, session.ID).Scan(&exists)
		if err == nil {
			skipped++
			continue
		}
		if err := s.Save(&session); err != nil {
			skipped++
			continue
		}
		imported++
	}
	return imported, skipped, nil
}

func endsWithJSON(name string) bool {
	return len(name) >= 5 && name[len(name)-5:] == ".json"
}