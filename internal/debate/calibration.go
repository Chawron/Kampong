package debate

import (
	"regexp"
	"strconv"

	"github.com/kampong/debate/internal/models"
)

// confidenceRe matches [Confidence: XX/100] in agent arguments.
var confidenceRe = regexp.MustCompile(`\[Confidence:\s*(\d{1,3})/100\]`)

// ExtractConfidence parses the confidence score from an agent's argument text.
// Returns 0 if no confidence tag is found.
func ExtractConfidence(text string) int {
	m := confidenceRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	v, err := strconv.Atoi(m[1])
	if err != nil || v < 0 || v > 100 {
		return 0
	}
	return v
}

// ComputeCalibrations analyzes the transcript and verdict to compute per-agent calibration.
func ComputeCalibrations(transcript []models.TranscriptEntry, verdict *models.Verdict) []models.AgentCalibration {
	if verdict == nil || len(verdict.Scorecard) == 0 {
		return nil
	}

	type scoreInfo struct {
		name  string
		total float64
	}
	scoreByAgent := make(map[string]scoreInfo)
	for _, s := range verdict.Scorecard {
		scoreByAgent[s.AgentID] = scoreInfo{name: s.AgentName, total: s.Total}
	}

	type accum struct {
		name      string
		totalConf int
		confCount int
	}
	agentAccum := make(map[string]*accum)

	for _, entry := range transcript {
		if entry.IsUserInject || entry.IsChallenge {
			continue
		}
		conf := ExtractConfidence(entry.Text)
		if conf == 0 {
			conf = entry.Confidence // fallback to pre-extracted confidence
		}
		if conf == 0 {
			continue
		}
		a, ok := agentAccum[entry.AgentID]
		if !ok {
			a = &accum{name: entry.AgentName}
			agentAccum[entry.AgentID] = a
		}
		a.totalConf += conf
		a.confCount++
	}

	var results []models.AgentCalibration
	for agentID, acc := range agentAccum {
		si, hasScore := scoreByAgent[agentID]
		if !hasScore || acc.confCount == 0 {
			continue
		}
		avgConf := float64(acc.totalConf) / float64(acc.confCount)
		judgeScoreNorm := si.total * 10.0
		var ratio float64
		if avgConf > 0 {
			ratio = judgeScoreNorm / avgConf
		}
		results = append(results, models.AgentCalibration{
			AgentID:          agentID,
			AgentName:        acc.name,
			TotalArguments:   acc.confCount,
			AvgConfidence:    avgConf,
			JudgeAvgScore:    si.total,
			CalibrationRatio: ratio,
		})
	}

	return results
}
