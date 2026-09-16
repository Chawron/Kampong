package debate

import (
	"log"

	"github.com/kampong/debate/internal/models"
	"github.com/kampong/debate/internal/store"
)

// CalibrationRollup folds the just-finished debate's per-agent confidence +
// judge score into the persistent rolling averages, then optionally applies
// the result as a calibration weight on the next debate's arguments.
//
// ApplyCalibrationWeight returns the agent's historical accuracy ratio
// (avg_judge_score / avg_confidence). If an agent is consistently over-
// confident (ratio < threshold), their future arguments should have their
// displayed confidence down-weighted. When no history is available we return
// 1.0 — the agent is treated as perfectly calibrated.
func ApplyCalibrationWeight(cs store.CalibrationStore, agentID string) float64 {
	if cs == nil {
		return 1.0
	}
	avgConf, avgJudge, n, ok := cs.Get(agentID)
	if !ok || n < 3 {
		// Need at least 3 samples before we trust the ratio; otherwise the
		// signal is too noisy to penalise anyone.
		return 1.0
	}
	if avgConf <= 0 {
		return 1.0
	}
	ratio := avgJudge / avgConf
	if ratio < 0.7 {
		// Down-weight: if you've been 30%+ over-confident historically,
		// reduce the influence of your next argument's confidence score.
		return 0.7
	}
	if ratio > 1.3 {
		// Slight boost for consistently under-confident agents so their
		// arguments aren't penalised for honesty.
		return 1.1
	}
	return 1.0
}

// UpdateCalibrationFromSession pulls per-agent scores out of a finished debate
// and persists them into the rolling average. Safe to call when no
// CalibrationStore is configured (it just returns).
func UpdateCalibrationFromSession(cs store.CalibrationStore, session *models.DebateSession) {
	if cs == nil || session == nil {
		return
	}
	verdict := session.GetVerdict()
	if verdict == nil {
		return
	}
	for _, sc := range verdict.Scorecard {
		if sc.AgentID == "" {
			continue
		}
		// Average the agent's confidence across their arguments; fall back
		// to 0.5 if none of their entries had a confidence tag.
		var confSum, confN float64
		for _, entry := range session.GetTranscript() {
			if entry.AgentID != sc.AgentID {
				continue
			}
			if entry.Confidence > 0 {
				confSum += float64(entry.Confidence) / 100.0
				confN++
			}
		}
		avgConf := 0.5
		if confN > 0 {
			avgConf = confSum / confN
		}
		// Judge total is on 0-10; normalise to 0-1.
		judgeScore := sc.Total / 10.0
		if err := cs.Update(sc.AgentID, avgConf, judgeScore); err != nil {
			log.Printf("CALIBRATION STORE UPDATE [%s] agent=%s: %v", session.ID, sc.AgentID, err)
		}
	}
}