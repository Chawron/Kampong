package panel

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/kampong/debate/internal/analyzer"
	"github.com/kampong/debate/internal/config"
	"github.com/kampong/debate/internal/models"
)

// RoleIcons maps role types to display emojis.
var RoleIcons = map[models.AgentRoleType][]string{
	models.RoleCore:         {"🧬", "🔬", "🧪", "📐", "💻", "🏗️", "⚙️", "🔭"},
	models.RoleAdjacent:     {"📜", "🌍", "💰", "📊", "🎨", "🏛️"},
	models.RolePractitioner: {"🐔", "🚗", "👨‍🍳", "🔧", "👨‍🌾", "🏭"},
	models.RoleLayperson:    {"👨‍👩‍👧", "🧑", "👴", "🤔", "👀"},
	models.RoleJudge:        {"⚖️"},
	models.RolePerspective:  {"🔍", "🧠", "📊", "🌐", "💡", "🎯", "📱", "🏥"},
	models.RoleSynthesizer:  {"🧩"},
}

// Composer builds the final agent panel from the analyzer's suggestion.
type Composer struct {
	cfg *config.Config
}

// NewComposer creates a panel composer.
func NewComposer(cfg *config.Config) *Composer {
	return &Composer{cfg: cfg}
}

// Compose takes the analyzer output and produces the final list of agents including the judge.
func (c *Composer) Compose(suggestion *analyzer.PanelSuggestion, topic string) ([]models.Agent, error) {
	var agents []models.Agent
	counts := map[models.AgentRoleType]int{
		models.RoleCore:         0,
		models.RoleAdjacent:     0,
		models.RolePractitioner: 0,
		models.RoleLayperson:    0,
	}

	expectedCounts := map[models.AgentRoleType]int{
		models.RoleCore:         c.cfg.Debate.Panel.CoreCount,
		models.RoleAdjacent:     c.cfg.Debate.Panel.AdjacentCount,
		models.RolePractitioner: c.cfg.Debate.Panel.PractitionerCount,
		models.RoleLayperson:    c.cfg.Debate.Panel.LayCount,
	}

	for _, sa := range suggestion.SuggestedPanel {
		roleType := models.AgentRoleType(sa.Type)
		if _, ok := expectedCounts[roleType]; !ok {
			continue // skip unknown types
		}
		if counts[roleType] >= expectedCounts[roleType] {
			continue // already filled this slot
		}

		icon := pickIcon(roleType, counts[roleType])
		id := uuid.New().String()[:8]

		agent := models.Agent{
			ID:        id,
			Name:      sa.Role,
			Role:      sa.Role,
			Expertise: sa.Expertise,
			RoleType:  roleType,
			Icon:      icon,
		}
		agents = append(agents, agent)
		counts[roleType]++
	}

	// Fill any remaining slots with generic agents if analyzer didn't provide enough
	c.fillRemainingSlots(&agents, counts, expectedCounts, topic, suggestion)

	// Add the judge
	judgeID := uuid.New().String()[:8]
	agents = append(agents, models.Agent{
		ID:        judgeID,
		Name:      "Debate Judge",
		Role:      "Debate Judge",
		Expertise: "Logical analysis, evidence evaluation, impartial judgment",
		RoleType:  models.RoleJudge,
		Icon:      "⚖️",
	})

	// Assign providers to all agents
	c.assignProviders(agents)

	return agents, nil
}

func (c *Composer) fillRemainingSlots(agents *[]models.Agent, counts map[models.AgentRoleType]int, expected map[models.AgentRoleType]int, topic string, suggestion *analyzer.PanelSuggestion) {
	genericNames := map[models.AgentRoleType][]struct{ name, expertise string }{
		models.RoleCore: {
			{"Domain Expert", "Deep expertise in the primary subject matter of: " + topic},
			{"Technical Specialist", "Specialized technical knowledge relevant to: " + topic},
		},
		models.RoleAdjacent: {
			{"Cross-Domain Analyst", "Connecting ideas across fields related to: " + topic},
			{"Systems Thinker", "Understanding how different aspects of " + topic + " interconnect"},
		},
		models.RolePractitioner: {
			{"Field Practitioner", "Hands-on real-world experience with: " + topic},
		},
		models.RoleLayperson: {
			{"Curious Observer", "Common sense perspective on: " + topic},
			{"Everyday Person", "Ordinary life experience related to: " + topic},
		},
	}

	for roleType, needed := range expected {
		for counts[roleType] < needed {
			names := genericNames[roleType]
			idx := counts[roleType] % len(names)
			id := uuid.New().String()[:8]
			icon := pickIcon(roleType, counts[roleType])
			*agents = append(*agents, models.Agent{
				ID:        id,
				Name:      names[idx].name,
				Role:      names[idx].name,
				Expertise: names[idx].expertise,
				RoleType:  roleType,
				Icon:      icon,
			})
			counts[roleType]++
		}
	}
}

func pickIcon(roleType models.AgentRoleType, idx int) string {
	icons, ok := RoleIcons[roleType]
	if !ok || len(icons) == 0 {
		return "👤"
	}
	return icons[idx%len(icons)]
}

// ComposeDiscussion builds a discussion panel from the analyzer's discussion suggestion.
func (c *Composer) ComposeDiscussion(suggestion *analyzer.DiscussionSuggestion, topic string) ([]models.Agent, error) {
	var agents []models.Agent

	for _, p := range suggestion.Perspectives {
		roleType := models.AgentRoleType(p.Type)
		if roleType != models.RolePerspective && roleType != models.RolePractitioner && roleType != models.RoleLayperson {
			roleType = models.RolePerspective
		}

		icon := pickIcon(roleType, len(agents))
		id := uuid.New().String()[:8]

		agent := models.Agent{
			ID:        id,
			Name:      p.Angle,
			Role:      p.Angle,
			Expertise: p.Expertise,
			RoleType:  roleType,
			Icon:      icon,
		}
		agents = append(agents, agent)
	}

	// Fill remaining slots if analyzer didn't provide enough
	totalNeeded := c.cfg.Debate.Panel.TotalAgents
	for len(agents) < totalNeeded {
		id := uuid.New().String()[:8]
		icon := pickIcon(models.RolePerspective, len(agents))
		agents = append(agents, models.Agent{
			ID:        id,
			Name:      fmt.Sprintf("Perspective %d", len(agents)+1),
			Role:      fmt.Sprintf("Perspective %d", len(agents)+1),
			Expertise: "Broad knowledge relevant to: " + topic,
			RoleType:  models.RolePerspective,
			Icon:      icon,
		})
	}

	// Add the synthesizer (replaces judge in discussion mode)
	synthID := uuid.New().String()[:8]
	agents = append(agents, models.Agent{
		ID:        synthID,
		Name:      "Discussion Synthesizer",
		Role:      "Discussion Synthesizer",
		Expertise: "Connecting ideas, identifying patterns, summarizing insights across perspectives",
		RoleType:  models.RoleSynthesizer,
		Icon:      "🧩",
	})

	// Assign providers to all agents
	c.assignProviders(agents)

	return agents, nil
}

// assignProviders assigns LLM providers to agents using a vendor-aware policy.
//
// The policy:
//  1. Bucket each agent by role (core+judge, adjacent, practitioner+layperson).
//  2. Greedy pick from enabled providers whose `Family` keeps the bucket's
//     per-family count ≤ 2 (so no family dominates a bucket).
//  3. If no provider satisfies (2), pick the least-used family — fall back to
//     a round-robin within that family so we still use something.
//
// The Agent.ProviderName field already exists; we also set a derived Family on
// the agent for downstream consumers (UI badges, vendor audit).
func (c *Composer) assignProviders(agents []models.Agent) {
	if len(c.cfg.Providers) == 0 {
		return
	}

	var enabled []config.ProviderConfig
	for _, p := range c.cfg.Providers {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	if len(enabled) == 0 {
		return
	}

	// Build provider index keyed by family so we can pick deterministically.
	byFamily := make(map[string][]config.ProviderConfig)
	for _, p := range enabled {
		fam := p.ResolvedFamily()
		byFamily[fam] = append(byFamily[fam], p)
	}

	// Track per-family usage per bucket so no family dominates a bucket.
	usage := make(map[string]map[string]int)

	for i := range agents {
		bucket := bucketForRole(agents[i].RoleType)
		chosen := pickProvider(byFamily, bucket, i, usage)
		agents[i].ProviderName = chosen.Name
		agents[i].ProviderFamily = chosen.ResolvedFamily()
		usage[bucket][chosen.ResolvedFamily()]++
	}
}

// bucketForRole groups roles into the three diversity buckets.
func bucketForRole(r models.AgentRoleType) string {
	switch r {
	case models.RoleCore, models.RoleJudge:
		return "core"
	case models.RoleAdjacent:
		return "adjacent"
	case models.RolePractitioner, models.RoleLayperson, models.RolePerspective, models.RoleSynthesizer:
		return "support"
	default:
		return "core"
	}
}

// pickProvider returns a provider that, ideally, comes from a family that has
// been used ≤2 times in this bucket. Falls back to the least-used family if no
// provider can keep diversity.
//
// usage maps bucket -> family -> count and is shared across all calls in a
// single assignProviders pass (each bucket keeps its own family counters).
func pickProvider(byFamily map[string][]config.ProviderConfig, bucket string, seedIdx int, usage map[string]map[string]int) config.ProviderConfig {
	// Order families deterministically: stable but interleaved per bucket.
	families := sortedFamilies(byFamily)

	if usage[bucket] == nil {
		usage[bucket] = make(map[string]int)
	}
	bucketUsage := usage[bucket]

	for _, fam := range families {
		providers := byFamily[fam]
		if len(providers) == 0 {
			continue
		}
		if bucketUsage[fam] < 2 {
			// Cycle through providers in this family deterministically.
			p := providers[seedIdx%len(providers)]
			return p
		}
	}

	// Fallback: pick the family with the lowest usage (still deterministic).
	least := families[0]
	for _, fam := range families {
		if len(byFamily[fam]) == 0 {
			continue
		}
		if bucketUsage[fam] < bucketUsage[least] {
			least = fam
		}
	}
	if providers := byFamily[least]; len(providers) > 0 {
		return providers[seedIdx%len(providers)]
	}
	// Shouldn't happen — but return the first enabled provider as a safety net.
	all := providersFromValues(byFamily)
	return all[seedIdx%len(all)]
}

func sortedFamilies(byFamily map[string][]config.ProviderConfig) []string {
	out := make([]string, 0, len(byFamily))
	for k := range byFamily {
		out = append(out, k)
	}
	// Stable alphabetical so bucket ordering doesn't depend on map iteration.
	sortStrings(out)
	return out
}

func providersFromValues(byFamily map[string][]config.ProviderConfig) []config.ProviderConfig {
	var out []config.ProviderConfig
	for _, v := range byFamily {
		out = append(out, v...)
	}
	return out
}

// sortStrings is a tiny insertion sort — keeps us off importing sort for one call.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
