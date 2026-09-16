// Package store persists debate sessions. Two implementations:
//
//   - JSONStore: one .json file per session under data/debates/.
//   - SQLiteStore: rows in a single SQLite database file (pure-Go driver,
//   no CGO required).
//
// The Session interface lets the rest of the program stay agnostic to the
// backend. Switch via config.yaml:
//
//	storage:
//	  backend: json   # or "sqlite"
//	  sqlite_path: data/kampong.db
package store

import (
	"errors"
	"time"

	"github.com/kampong/debate/internal/models"
)

// Event is a single WebSocket event captured for replay.
type Event struct {
	DebateID  string
	Seq       int64
	EventType string
	Payload   []byte
}

// Session is the storage interface used by the API and engine.
type Session interface {
	Save(*models.DebateSession) error
	Load(id string) (*models.DebateSession, error)
	List() ([]SessionMeta, error)
	Delete(id string) error
}

// EventStore is an optional extension: stores append-only event streams so
// late-joining clients can replay a debate from an arbitrary sequence number.
type EventStore interface {
	AppendEvent(ev Event) error
	EventsAfter(debateID string, fromSeq int64) ([]Event, error)
}

// CalibrationStore tracks rolling per-agent calibration averages across debates.
type CalibrationStore interface {
	// Get returns the rolling average (avg_confidence, avg_judge_score,
	// debate_count). If no history exists, returns zero values with ok=false.
	Get(agentID string) (avgConfidence, avgJudgeScore float64, debateCount int, ok bool)
	// Update incorporates (confidence, judgeScore) into the rolling average for
	// the given agent.
	Update(agentID string, confidence, judgeScore float64) error
}

// ErrNotFound is returned by Load when a session doesn't exist.
var ErrNotFound = errors.New("session not found")

// SessionMeta is the lightweight listing info for a saved debate.
type SessionMeta struct {
	ID         string    `json:"id"`
	Topic      string    `json:"topic"`
	Mode       string    `json:"mode"`
	Status     string    `json:"status"`
	Agents     int       `json:"agents"`
	Rounds     int       `json:"rounds"`
	CreatedAt  time.Time `json:"created_at"`
	HasVerdict bool      `json:"has_verdict"`
}