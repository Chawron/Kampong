# Kampong — AI Multi-Agent Debate Arena

## System Summary v2 (August 2026)

**Kampong** is a Go-based AI debate platform where multiple expert agents research the web, debate in structured rounds, and deliver judged verdicts — all streamed live with a real-time knowledge graph. It includes a comprehensive medical diagnosis subsystem, human-in-the-loop controls, confidence-weighted judging, social reaction simulation, and post-debate quality analysis.

---

## Codebase Stats

| Category | Files | Lines |
|----------|------:|------:|
| Go (.go) | 53 | 15,709 |
| JavaScript (.js) | 5 | 3,016 |
| CSS (.css) | 2 | 3,088 |
| HTML (.html) | 2 | 610 |
| **Total** | **62** | **22,423** |

**18 Go packages** | **38 API endpoints** (20 regular + 18 medical) | **17 WebSocket events**

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                     FRONTEND (Browser SPA)                            │
│  index.html · medical-input.html                                      │
│  app.js · arena.js · graph.js · ws.js · medical-input.js             │
│  style.css (2,641 lines) · medical.css (447 lines)                   │
│                                                                       │
│  Features: Debate control bar, confidence badges, pause/inject/       │
│  challenge modals, post-debate analysis panel, social reaction grid,  │
│  medical knowledge panel, D3.js knowledge graph, streaming feed      │
└────────────────────────────┬─────────────────────────────────────────┘
                             │ WebSocket + REST API
┌────────────────────────────▼─────────────────────────────────────────┐
│                        API SERVER (Go)                                │
│  38 endpoints · 17 WS events · config persistence                    │
│                                                                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │  Debate   │ │ Medical  │ │  Human   │ │  Social  │ │  Config  │  │
│  │  Engine   │ │   API    │ │ Control  │ │Simulation│ │ Provider │  │
│  │ +calibra- │ │ 18 end-  │ │ pause/   │ │ 12 per-  │ │ multi-   │  │
│  │  tion +   │ │ points   │ │ resume/  │ │ sonas    │ │ provider │  │
│  │  critique │ │          │ │ inject/  │ │          │ │ + persist│  │
│  │          │ │          │ │ challenge│ │          │ │          │  │
│  └─────┬────┘ └─────┬────┘ └──────────┘ └──────────┘ └──────────┘  │
│        │            │                                                │
│  ┌─────▼────────────▼──────────────────────────────────────────────┐│
│  │           DEBATE ENGINE (6 files, 1,196 lines)                   ││
│  │  engine.go · judge.go · crossexam.go                             ││
│  │  calibration.go · consistency.go · autocritique.go               ││
│  └─────┬───────────────────────────────────────────────────────────┘│
│        │                                                             │
│  ┌─────▼───────────────────────────────────────────────────────────┐│
│  │           AGENT RUNTIME (agent.go + prompts.go)                  ││
│  │  Iterative research (3 rounds) · Confidence scoring              ││
│  │  Diversity-aware stances · Research cache integration            ││
│  └─────┬───────────────────────────────────────────────────────────┘│
│        │                                                             │
│  ┌─────▼──────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐ │
│  │  LLM Layer │  │Knowledge │  │  Search  │  │  Panel   │ │Analyzer│ │
│  │  client +  │  │  Graph   │  │ Tavily / │  │ Composer │ │+ comple│ │
│  │  manager   │  │extractor │  │DuckDuckGo│  │          │ ││xity   │ │
│  │  + vision  │  │+ auto-   │  │+ cache   │  │          │ │       │ │
│  │  + multi   │  │ connect  │  │          │  │          │ │       │ │
│  └────────────┘ └──────────┘ └──────────┘ └──────────┘ └────────┘ │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────────┐│
│  │        MEDICAL SUBSYSTEM (18 files, 5,292 lines)                 ││
│  │  engine · api · prompts · models · redflags · druginteractions   ││
│  │  labinterpreter · imageanalyzer · guidelines · icd · pubmed      ││
│  │  evidence · secondopinion · literature · reportformatter         ││
│  │  disclaimer · differential · riskscore · longitudinal            ││
│  └──────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────┘
```

---

## 6 Pillars — All Implemented

### Pillar 1: Diversity + Confidence ✅

| Feature | Implementation |
|---------|---------------|
| Confidence scoring | Agents output `[Confidence: XX/100]` — parsed and stored per argument |
| Weighted judging | Judge considers confidence calibration when scoring |
| Diversity stances | Core=strong position, Adjacent=challenge, Practitioner=feasibility, Layperson=clarity |
| Calibration tracking | Post-debate: avg confidence vs judge score → calibration ratio per agent |
| Frontend | Color-coded confidence badges (green >70, yellow 40-70, red <40) |

### Pillar 2: Human-in-the-Loop ✅

| Feature | Endpoint | Frontend |
|---------|----------|----------|
| Pause debate | `POST /api/debate/{id}/pause` | ⏸ button in control bar |
| Resume debate | `POST /api/debate/{id}/resume` | ▶ button (toggles) |
| Inject evidence | `POST /api/debate/{id}/inject` | 💉 modal with textarea + agent dropdown |
| Challenge claim | `POST /api/debate/{id}/challenge` | 🎯 modal with claim list + challenge text |
| WebSocket events | `debate_paused`, `debate_resumed`, `user_injection`, `claim_challenged` | Pause banner, injection cards in feed |

### Pillar 3: Evaluation & Calibration ✅

| Feature | Implementation |
|---------|---------------|
| Consistency check | Per-agent keyword overlap across rounds, contradiction detection |
| Auto-critique | Separate LLM call evaluates: quality, strengths, weaknesses, diversity, research depth |
| Post-debate analysis | Broadcast via `post_debate_analysis` event |
| Frontend | Collapsible panel with calibration table, consistency bars, critique card |

### Pillar 4: Social/Public Reaction Layer ✅

| Feature | Implementation |
|---------|---------------|
| 12 personas | concerned_parent, science_journalist, corporate_executive, patient_advocate, conspiracy_skeptic, policy_maker, social_media_influencer, healthcare_worker, elderly_citizen, university_student, insurance_analyst, community_leader |
| Per-persona reaction | Sentiment, amplify, distort, trust score |
| Narrative analysis | Dominant narrative, risk areas, overall sentiment |
| Endpoint | `POST /api/debate/{id}/social-simulation` |
| Frontend | Social grid with persona cards, sentiment badges, narrative summary |

### Pillar 5: Cost & Latency Control ✅

| Feature | Implementation |
|---------|---------------|
| Research cache | Thread-safe, 30min TTL, deduplicates search API calls across agents |
| Complexity assessment | Heuristic (zero-cost) + LLM-based topic difficulty scoring |
| Adaptive depth | Recommends quick/standard/deep/discussion based on complexity |
| Cache stats | Hit count tracking for monitoring |

### Pillar 6: Medical Enhancements ✅

| Feature | Implementation |
|---------|---------------|
| Differential diagnosis | LLM-ranked with probability, supporting/against evidence, next test, urgency |
| Dynamic risk score | Rule-based composite: red flags (30%) + drugs (20%) + labs (25%) + age (10%) + history (15%) |
| Longitudinal memory | Patient visit history stored as JSON, trend analysis via LLM |
| Lab image extraction | Vision model reads structured lab values from report photos |
| Frontend | Probability bars, risk gauge with breakdown, DX cards |

---

## Core Features

### Debate Modes

| Mode | Rounds | Phases | Output |
|------|--------|--------|--------|
| **Quick** | 2 | Opening → Rebuttal → Verdict | Winner + scorecard |
| **Standard** | 3 | Opening → Rebuttal → Closing → Verdict | Winner + scorecard |
| **Deep** | 5 | Opening → Rebuttal → Cross-Exam → Final Rebuttal → Closing → Verdict | Winner + scorecard |
| **Discussion** | 4 | Perspectives → Deep Dive → Cross-Pollination → Synthesis | Insights (no winner) |

### Agent Panel (6 agents + judge)

| Role | Count | Focus |
|------|-------|-------|
| Core Expert | 2 | Deep domain knowledge, strong positions |
| Adjacent Expert | 2 | Cross-disciplinary, challenges assumptions |
| Practitioner | 1 | Real-world feasibility |
| Layperson | 1 | Common sense, demands clarity |
| Judge | 1 | Confidence-weighted final evaluation |

### Medical Subsystem (18 modules)

| Module | Lines | What it does |
|--------|------:|-------------|
| api.go | 854 | Central handler, knowledge gathering, 18 endpoints |
| labinterpreter.go | 611 | 20+ tests with reference ranges, clinical significance |
| druginteractions.go | 330 | 15+ drug pairs, 3 check types |
| secondopinion.go | 340 | 5 independent panels in parallel |
| reportformatter.go | 348 | SOAP, discharge, consultation reports |
| guidelines.go | 368 | 7 conditions (ACC/AHA, ADA, GINA, KDIGO, etc.) |
| imageanalyzer.go | 422 | Vision-based: X-ray, CT, MRI, ECG, lab reports |
| literature.go | 415 | PubMed + ClinicalTrials.gov |
| redflags.go | 254 | 16 emergency rules + vital sign thresholds |
| riskscore.go | 255 | Dynamic composite risk score |
| pubmed.go | 295 | Real NCBI E-utilities integration |
| evidence.go | 205 | Oxford evidence levels 1-5 |
| differential.go | 203 | LLM-ranked differential diagnosis |
| engine.go | 183 | Medical debate orchestration |
| icd.go | 177 | 40+ ICD-10 codes |
| longitudinal.go | 152 | Patient visit history + trend analysis |
| models.go | 157 | Complete medical data model |
| prompts.go | 186 | 8 medical agent role prompts |
| disclaimer.go | 97 | Informed consent, disclaimers |

---

## All API Endpoints (38 total)

### Regular (20)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/debate/start` | Start debate |
| GET | `/api/debate/{id}/status` | Current status |
| GET | `/api/debate/{id}/history` | Full transcript |
| GET | `/api/debate/{id}/graph` | Knowledge graph |
| GET | `/api/debate/{id}/report` | Download report |
| POST | `/api/debate/{id}/cancel` | Cancel debate |
| POST | `/api/debate/{id}/pause` | **Pause (Pillar 2)** |
| POST | `/api/debate/{id}/resume` | **Resume (Pillar 2)** |
| POST | `/api/debate/{id}/inject` | **Inject evidence (Pillar 2)** |
| POST | `/api/debate/{id}/challenge` | **Challenge claim (Pillar 2)** |
| POST | `/api/debate/{id}/social-simulation` | **Social simulation (Pillar 4)** |
| GET | `/api/debates` | List all debates |
| DELETE | `/api/debate/{id}` | Delete debate |
| GET | `/api/config` | Get config |
| PUT | `/api/config` | Update config (persisted) |
| POST | `/api/upload` | File upload + vision |
| GET | `/api/providers` | List providers |
| POST | `/api/providers` | Add provider |
| DELETE | `/api/providers/{name}` | Remove provider |
| GET | `/ws/debate/{id}` | WebSocket |

### Medical (18)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/medical/debate/start` | Start medical debate |
| GET | `/api/medical/consent-text` | Consent text |
| POST | `/api/medical/consent` | Record consent |
| GET | `/api/medical/case` | Get case data |
| POST | `/api/medical/redflag-check` | Red flag detection |
| POST | `/api/medical/drug-interaction-check` | Drug interactions |
| POST | `/api/medical/lab-interpretation` | Lab interpretation |
| POST | `/api/medical/image-analysis` | Image analysis |
| POST | `/api/medical/extract-lab-image` | **Lab extraction from image** |
| POST | `/api/medical/guideline-match` | Clinical guidelines |
| POST | `/api/medical/icd-code-match` | ICD-10 codes |
| POST | `/api/medical/second-opinion` | Multi-panel second opinion |
| POST | `/api/medical/literature-search` | PubMed + trials |
| POST | `/api/medical/generate-report` | Medical reports |
| POST | `/api/medical/differential-ranking` | **Differential diagnosis (Pillar 6)** |
| POST | `/api/medical/risk-score` | **Dynamic risk score (Pillar 6)** |
| POST | `/api/medical/longitudinal/save` | **Save patient visit (Pillar 6)** |
| GET | `/api/medical/longitudinal/load` | **Load patient history (Pillar 6)** |

---

## WebSocket Events (17 total)

| Event | Source | Description |
|-------|--------|-------------|
| `phase_change` | engine | Debate phase transition |
| `panel_ready` | router | Agent panel assembled |
| `agent_searching` | engine | Agent started research |
| `research_round` | engine | Search results from a round |
| `agent_thinking` | engine | Agent has results, preparing |
| `agent_speaking` | engine | Token-by-token streaming |
| `agent_done` | engine | Agent finished (includes confidence) |
| `graph_update` | engine | New graph nodes/edges |
| `verdict` | judge | Final verdict |
| `synthesis` | judge | Discussion synthesis |
| `debate_paused` | **control** | **Debate paused (Pillar 2)** |
| `debate_resumed` | **control** | **Debate resumed (Pillar 2)** |
| `user_injection` | **control** | **User injected evidence (Pillar 2)** |
| `claim_challenged` | **control** | **User challenged claim (Pillar 2)** |
| `post_debate_analysis` | **engine** | **Calibration + consistency + critique (Pillar 3)** |
| `social_simulation` | **router** | **Public reaction results (Pillar 4)** |
| `error` | router/engine | Error notification |

---

## Language Support

All prompts include language matching — agents, judge, and synthesizer respond in the same language as the debate topic (Thai, English, or any other).

---

## Deployment

- **Single binary:** `kampong.exe` (Windows) or `kampong` (Linux/macOS)
- **Default port:** 9090
- **Data:** `data/debates/` (sessions), `data/uploads/` (files), `data/longitudinal/` (patients)
- **Config:** `config.yaml` (persisted on updates)
- **No database required**
- **Graceful shutdown** on SIGINT/SIGTERM

---

## Tests

| Package | Tests | Status |
|---------|------:|--------|
| internal/graph | 6 | ✅ |
| internal/models | 6 | ✅ |
| internal/store | 4 | ✅ |
| internal/ws | 6 | ✅ |
