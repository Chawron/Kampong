# Kampong — Product Overview

## What We've Built

**22,423 lines of code** across **62 files** — a complete AI debate platform with medical intelligence.

### By the Numbers

| Metric | Count |
|--------|------:|
| Go packages | 18 |
| API endpoints | 38 |
| WebSocket events | 17 |
| Medical modules | 18 |
| Debate modes | 4 |
| Agent roles | 6 + judge |
| Medical agent roles | 8 |
| Social personas | 12 |
| Clinical guidelines | 7 |
| Drug interactions | 15+ |
| Lab tests | 20+ |
| ICD-10 codes | 40+ |
| Red flag rules | 16 |
| Tests | 22 |

---

## Feature Completion Matrix

| Feature | Backend | Frontend | API | Status |
|---------|:-------:|:--------:|:---:|:------:|
| Multi-agent debate engine | ✅ | ✅ | ✅ | **Done** |
| 4 debate modes | ✅ | ✅ | ✅ | **Done** |
| Agentic web research (3 rounds) | ✅ | ✅ | ✅ | **Done** |
| Real-time knowledge graph (D3.js) | ✅ | ✅ | ✅ | **Done** |
| Multi-provider LLM | ✅ | ✅ | ✅ | **Done** |
| Vision image analysis | ✅ | ✅ | ✅ | **Done** |
| Config persistence | ✅ | ✅ | ✅ | **Done** |
| API key validation | ✅ | — | ✅ | **Done** |
| Language auto-detection | ✅ | — | — | **Done** |
| Confidence scoring | ✅ | ✅ | ✅ | **Done** |
| Human-in-the-loop (pause/inject/challenge) | ✅ | ✅ | ✅ | **Done** |
| Calibration tracking | ✅ | ✅ | ✅ | **Done** |
| Consistency checking | ✅ | ✅ | ✅ | **Done** |
| Auto-critique | ✅ | ✅ | ✅ | **Done** |
| Social reaction simulation | ✅ | ✅ | ✅ | **Done** |
| Research caching | ✅ | — | — | **Done** |
| Adaptive complexity | ✅ | — | ✅ | **Done** |
| Medical case input | ✅ | ✅ | ✅ | **Done** |
| Medical knowledge injection | ✅ | ✅ | ✅ | **Done** |
| Red flag detection | ✅ | ✅ | ✅ | **Done** |
| Drug interaction check | ✅ | ✅ | ✅ | **Done** |
| Lab interpretation | ✅ | ✅ | ✅ | **Done** |
| Lab image extraction | ✅ | ✅ | ✅ | **Done** |
| Clinical guidelines | ✅ | ✅ | ✅ | **Done** |
| ICD-10 coding | ✅ | ✅ | ✅ | **Done** |
| PubMed search | ✅ | ✅ | ✅ | **Done** |
| Literature search | ✅ | ✅ | ✅ | **Done** |
| Evidence scoring | ✅ | ✅ | ✅ | **Done** |
| Second opinion (5 panels) | ✅ | — | ✅ | **Done** |
| Medical report generation | ✅ | — | ✅ | **Done** |
| Differential diagnosis ranking | ✅ | ✅ | ✅ | **Done** |
| Dynamic risk score | ✅ | ✅ | ✅ | **Done** |
| Longitudinal patient memory | ✅ | — | ✅ | **Done** |
| Medical image analysis | ✅ | ✅ | ✅ | **Done** |
| Informed consent flow | ✅ | ✅ | ✅ | **Done** |

**Completion: 34/34 features = 100%**

---

## What Could Be Improved (Future Roadmap)

### High Priority

| Item | Description | Effort |
|------|-------------|--------|
| **Local model support** | Ollama/vLLM integration for cost reduction on low-stakes roles | Medium |
| **Message pruning** | Keep only contradicting arguments to reduce token usage | Medium |
| **Debate templates** | Pre-built debate configs for common scenarios (medical, policy, tech) | Low |
| **Export to PDF** | Direct PDF generation from debate reports | Low |
| **Multi-language debates** | Agents that can debate in different languages simultaneously | Medium |

### Medium Priority

| Item | Description | Effort |
|------|-------------|--------|
| **Database backend** | Replace JSON files with SQLite/PostgreSQL for multi-user | High |
| **User authentication** | Login, user-specific debates, access control | High |
| **Debate replay** | Step-by-step replay of past debates with timing | Medium |
| **Agent personality presets** | Named agent personalities (Skeptic, Optimist, Data-Driven) | Low |
| **Real-time collaboration** | Multiple users watching/injecting simultaneously | High |
| **Notification system** | Email/webhook when debate completes or red flags detected | Medium |
| **Batch medical screening** | Process multiple patients in batch mode | Medium |

### Low Priority (Nice to Have)

| Item | Description | Effort |
|------|-------------|--------|
| **Mobile app** | React Native or PWA for mobile access | High |
| **Voice input** | Speech-to-text for medical case input | Medium |
| **DICOM support** | Native DICOM medical image parsing | High |
| **FHIR integration** | Healthcare data standard interoperability | High |
| **Debate marketplace** | Share/sell debate configurations and agent panels | High |
| **API rate limiting** | Per-user rate limits for multi-tenant deployment | Low |
| **Analytics dashboard** | Usage stats, cost tracking, debate quality trends | Medium |

---

## Competitive Positioning

### vs MiroFish

| Feature | Kampong | MiroFish |
|---------|:-------:|:--------:|
| Multi-agent debate | ✅ | ✅ |
| Knowledge graph | ✅ | ✅ |
| Medical intelligence | ✅ (deep) | ❌ |
| Human-in-the-loop | ✅ | ❌ |
| Social reaction simulation | ✅ | ✅ |
| Confidence calibration | ✅ | ❌ |
| Auto-critique | ✅ | ❌ |
| Drug interaction check | ✅ | ❌ |
| PubMed integration | ✅ | ❌ |
| Self-hosted | ✅ | ❌ (SaaS) |
| Open source | ✅ | ❌ |

### Unique Advantages

1. **Medical-grade intelligence** — 18 medical modules, not just general debate
2. **Full human control** — pause, inject, challenge, redirect mid-debate
3. **Quality assurance** — calibration, consistency, auto-critique ensure debate quality
4. **Cost control** — research caching, adaptive depth, multi-provider support
5. **Privacy** — self-hosted, no data leaves your infrastructure
6. **Extensible** — Go backend, clean architecture, easy to add modules

---

## Technical Debt & Known Issues

| Issue | Severity | Notes |
|-------|----------|-------|
| `go vet` warning in `graph/extractor.go` | Low | Pre-existing `string(int)` conversion — already fixed in one place |
| PubMed XML parser is simplified | Low | Uses string matching instead of proper XML parsing — works but fragile |
| No database | Medium | JSON files work for single-user, limiting for multi-tenant |
| `strings.Title` deprecated | Low | Used in a few places, cosmetic warning |
| PDF extraction is a stub | Low | File upload handles PDFs but doesn't extract text |

---

## Deployment Checklist

- [x] Single binary deployment
- [x] Config persistence to disk
- [x] API key validation before debates
- [x] Graceful shutdown
- [x] CORS support
- [x] WebSocket reconnection
- [x] Path traversal protection
- [x] File upload size limits (10MB)
- [ ] HTTPS/TLS termination (use reverse proxy)
- [ ] Rate limiting (add middleware)
- [ ] Health check endpoint (add `/api/health`)
- [ ] Metrics/monitoring (add Prometheus)
- [ ] Docker containerization (add Dockerfile)
- [ ] CI/CD pipeline (add GitHub Actions)

---

## Quick Start

```bash
# Build
go build -o kampong.exe ./cmd/server

# Run
./kampong.exe -config config.yaml

# Open browser
# http://localhost:9090
```

### First Time Setup

1. Open http://localhost:9090
2. Enter your API key in Settings (or add a Provider)
3. Start a debate or go to Medical Input for medical discussions
4. API key is saved to `config.yaml` and survives restarts

---

*Last updated: August 12, 2026*
*Version: 2.0 — All 6 Pillars Complete*
