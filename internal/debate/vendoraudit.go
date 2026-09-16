package debate

import (
	"fmt"
	"strings"

	"github.com/kampong/debate/internal/models"
)

// ComputeVendorAudit summarises the debate from a vendor-diversity angle.
//
// Inputs:
//   - agents:  the panel as composed (ProviderFamily populated by Composer)
//   - scorecard: from the verdict (one AgentScore per speaker)
//   - consolidatedClaims: from the verdict (which agents supported/opposed what)
//
// Output flags three things:
//  1. How many agents per vendor family.
//  2. For each consolidated claim, the vendor families on each side.
//  3. Plain-English warnings about dominance / one-sidedness.
func ComputeVendorAudit(
	agents []models.Agent,
	scorecard []models.AgentScore,
	consolidatedClaims []models.ConsolidatedClaim,
) models.VendorAudit {
	audit := models.VendorAudit{
		VendorDistribution: map[string]int{},
		FamilyScorecard:    map[string]float64{},
	}

	// Map agent_id → family for cheap lookups.
	familyByID := make(map[string]string)
	for _, a := range agents {
		fam := a.ProviderFamily
		if fam == "" {
			fam = "unknown"
		}
		familyByID[a.ID] = fam
		audit.VendorDistribution[fam]++
	}

	// Average judge score per family.
	type famAcc struct {
		sum   float64
		count int
	}
	famScore := make(map[string]*famAcc)
	for _, s := range scorecard {
		fam := familyByID[s.AgentID]
		if fam == "" {
			fam = "unknown"
		}
		acc, ok := famScore[fam]
		if !ok {
			acc = &famAcc{}
			famScore[fam] = acc
		}
		acc.sum += s.Total
		acc.count++
	}
	for fam, acc := range famScore {
		if acc.count > 0 {
			audit.FamilyScorecard[fam] = acc.sum / float64(acc.count)
		}
	}

	// For each consolidated claim, list the families on each side.
	for _, c := range consolidatedClaims {
		split := models.ClaimVendorSplit{Claim: c.Claim}
		seen := map[string]bool{}
		for _, name := range c.Support {
			id := lookupAgentIDByName(agents, name)
			fam := familyByID[id]
			if fam == "" {
				continue
			}
			if !seen["s:"+fam] {
				split.SupportFamilies = append(split.SupportFamilies, fam)
				seen["s:"+fam] = true
			}
		}
		seen = map[string]bool{}
		for _, name := range c.Oppose {
			id := lookupAgentIDByName(agents, name)
			fam := familyByID[id]
			if fam == "" {
				continue
			}
			if !seen["o:"+fam] {
				split.OpposeFamilies = append(split.OpposeFamilies, fam)
				seen["o:"+fam] = true
			}
		}
		audit.InterVendorClaims = append(audit.InterVendorClaims, split)
	}

	// Warnings.
	if len(audit.VendorDistribution) <= 1 {
		audit.BiasWarnings = append(audit.BiasWarnings,
			fmt.Sprintf("Only one vendor family represented (%s). Debate may inherit that vendor's biases.",
				onlyFamily(audit.VendorDistribution)))
	}
	// Did the winner's family have all the supporting evidence on the winning claim?
	if winnerFam := winningFamily(scorecard, familyByID); winnerFam != "" {
		mono := countMonochromeClaims(audit.InterVendorClaims, winnerFam)
		if mono > 0 && len(audit.InterVendorClaims) > 0 && mono == len(audit.InterVendorClaims) {
			audit.BiasWarnings = append(audit.BiasWarnings,
				fmt.Sprintf("All consolidated claims were supported only by family %q. Cross-vendor review recommended.",
					winnerFam))
		}
	}
	// Dominance check.
	for fam, n := range audit.VendorDistribution {
		if n >= 4 && len(agents) > 0 && float64(n)/float64(len(agents)) >= 0.6 {
			audit.BiasWarnings = append(audit.BiasWarnings,
				fmt.Sprintf("Family %q supplied %d of %d agents (%.0f%%). Diversity is limited.",
					fam, n, len(agents), 100*float64(n)/float64(len(agents))))
		}
	}

	return audit
}

func lookupAgentIDByName(agents []models.Agent, name string) string {
	for _, a := range agents {
		if strings.EqualFold(a.Name, name) {
			return a.ID
		}
	}
	return ""
}

func onlyFamily(m map[string]int) string {
	for k := range m {
		return k
	}
	return ""
}

func winningFamily(scorecard []models.AgentScore, familyByID map[string]string) string {
	if len(scorecard) == 0 {
		return ""
	}
	best := scorecard[0]
	for _, s := range scorecard[1:] {
		if s.Total > best.Total {
			best = s
		}
	}
	return familyByID[best.AgentID]
}

func countMonochromeClaims(splits []models.ClaimVendorSplit, fam string) int {
	count := 0
	for _, s := range splits {
		if len(s.SupportFamilies) == 1 && s.SupportFamilies[0] == fam && len(s.OpposeFamilies) == 0 {
			count++
		}
	}
	return count
}
