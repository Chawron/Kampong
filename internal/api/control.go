package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kampong/debate/internal/models"
	"github.com/kampong/debate/internal/ws"
)

// handlePauseDebate — POST /api/debate/{id}/pause
// Pauses a running debate and broadcasts "debate_paused" via WebSocket.
func (s *Server) handlePauseDebate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	if session.IsPaused() {
		http.Error(w, `{"error":"debate is already paused"}`, http.StatusConflict)
		return
	}

	// Parse optional reason from body
	var body struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Reason == "" {
		body.Reason = "Paused by user"
	}

	session.Pause(body.Reason)

	log.Printf("CONTROL [%s]: Debate paused — %s", id, body.Reason)

	s.hub.Broadcast(id, ws.Event{
		Event:    "debate_paused",
		DebateID: id,
		Data: map[string]interface{}{
			"reason": body.Reason,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "paused",
		"reason": body.Reason,
	})
}

// handleResumeDebate — POST /api/debate/{id}/resume
// Resumes a paused debate and broadcasts "debate_resumed" via WebSocket.
func (s *Server) handleResumeDebate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	if !session.IsPaused() {
		http.Error(w, `{"error":"debate is not paused"}`, http.StatusConflict)
		return
	}

	restored := session.Resume()

	log.Printf("CONTROL [%s]: Debate resumed — restoring to %s", id, restored)

	s.hub.Broadcast(id, ws.Event{
		Event:    "debate_resumed",
		DebateID: id,
		Data:     map[string]interface{}{},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "resumed",
		"restored_phase": string(restored),
	})
}

// handleInjectEvidence — POST /api/debate/{id}/inject
// Injects user evidence, question, or redirect into the debate transcript.
// Body: { "type": "evidence|question|redirect", "target_agent_id": "...", "text": "..." }
func (s *Server) handleInjectEvidence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	var body struct {
		Type          string `json:"type"`
		TargetAgentID string `json:"target_agent_id"`
		Text          string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.Text == "" {
		http.Error(w, `{"error":"text is required"}`, http.StatusBadRequest)
		return
	}
	if body.Type == "" {
		body.Type = "evidence"
	}

	// Validate type
	switch body.Type {
	case "evidence", "question", "redirect":
	default:
		http.Error(w, `{"error":"type must be evidence, question, or redirect"}`, http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	injection := models.UserInjection{
		ID:            uuid.New().String(),
		DebateID:      id,
		Type:          body.Type,
		TargetAgentID: body.TargetAgentID,
		Text:          body.Text,
		Timestamp:     now,
	}

	// Add injection record to session
	session.AddInjection(injection)

	// Also append as a transcript entry so it appears in the debate flow
	entry := models.TranscriptEntry{
		ID:            injection.ID,
		AgentID:       "user",
		AgentName:     "User",
		AgentRole:     "human",
		Round:         session.Round,
		Phase:         session.GetStatus(),
		Text:          body.Text,
		Timestamp:     now,
		TargetAgentID: body.TargetAgentID,
		IsUserInject:  true,
	}
	session.AppendTranscript(entry)

	log.Printf("CONTROL [%s]: User injection (%s) — %s", id, body.Type, truncate(body.Text, 80))

	s.hub.Broadcast(id, ws.Event{
		Event:    "user_injection",
		DebateID: id,
		Data: map[string]interface{}{
			"type":            body.Type,
			"text":            body.Text,
			"target_agent_id": body.TargetAgentID,
			"timestamp":       now.Format(time.RFC3339),
			"injection_id":    injection.ID,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "injected",
		"injection_id": injection.ID,
	})
}

// handleChallengeClaim — POST /api/debate/{id}/challenge
// Challenges a specific transcript entry/claim.
// Body: { "transcript_entry_id": "...", "challenge_text": "..." }
func (s *Server) handleChallengeClaim(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	session := s.getSession(id)
	if session == nil {
		http.Error(w, `{"error":"debate not found"}`, http.StatusNotFound)
		return
	}

	var body struct {
		TranscriptEntryID string `json:"transcript_entry_id"`
		ChallengeText     string `json:"challenge_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if body.TranscriptEntryID == "" {
		http.Error(w, `{"error":"transcript_entry_id is required"}`, http.StatusBadRequest)
		return
	}
	if body.ChallengeText == "" {
		http.Error(w, `{"error":"challenge_text is required"}`, http.StatusBadRequest)
		return
	}

	// Verify the target entry exists
	transcript := session.GetTranscript()
	found := false
	var challengedAgent string
	for _, e := range transcript {
		if e.ID == body.TranscriptEntryID {
			found = true
			challengedAgent = e.AgentName
			break
		}
	}
	if !found {
		http.Error(w, `{"error":"transcript entry not found"}`, http.StatusNotFound)
		return
	}

	now := time.Now().UTC()
	challengeID := uuid.New().String()

	// Append challenge as a transcript entry
	entry := models.TranscriptEntry{
		ID:            challengeID,
		AgentID:       "user",
		AgentName:     "User",
		AgentRole:     "human",
		Round:         session.Round,
		Phase:         session.GetStatus(),
		Text:          body.ChallengeText,
		Timestamp:     now,
		TargetAgentID: body.TranscriptEntryID, // points to the challenged entry
		IsChallenge:   true,
	}
	session.AppendTranscript(entry)

	log.Printf("CONTROL [%s]: Claim challenged (entry %s by %s)", id, body.TranscriptEntryID, challengedAgent)

	s.hub.Broadcast(id, ws.Event{
		Event:    "claim_challenged",
		DebateID: id,
		Data: map[string]interface{}{
			"entry_id":        body.TranscriptEntryID,
			"challenge_id":    challengeID,
			"challenge_text":  body.ChallengeText,
			"challenged_agent": challengedAgent,
			"timestamp":       now.Format(time.RFC3339),
		},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "challenged",
		"challenge_id": challengeID,
	})
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
