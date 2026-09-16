# Kampong — AI Expert Debate Arena

**Kampong** is an AI-powered debate and discussion system. It assembles a panel of expert AI agents, has them research the web, debate each other in structured rounds, and delivers a judge's verdict or a synthesized set of insights — all streamed live to a browser with a real-time knowledge graph.

## How It Works

1. **Analyze** — An LLM analyzes your topic and suggests which experts should participate.
2. **Compose** — The system builds a balanced panel: core domain experts, adjacent-field thinkers, practitioners, and laypersons.
3. **Research** — Each agent performs iterative web search (DuckDuckGo or Tavily) before speaking, refining queries across up to 3 research rounds.
4. **Debate** — Agents speak in randomized order across multiple rounds (opening → rebuttal → cross-examination → closing), with every token streamed live.
5. **Verdict** — A judge LLM evaluates evidence quality, logical coherence, and novelty. In discussion mode, a synthesizer produces themes, insights, and open questions instead of picking a winner.
6. **Knowledge Graph** — Claims, concepts, and evidence are extracted from every argument (LLM + NLP) and auto-connected, visualized as an interactive D3.js force graph.

## Features

- **4 debate modes:** Quick (2 rounds), Standard (3), Deep (5 with cross-examination), Discussion (4 rounds, synthesis instead of winner)
- **Multi-agent panel:** Configurable mix of core experts, adjacent-field thinkers, practitioners, and laypersons
- **Agentic web research:** Iterative search with LLM-guided follow-up queries and deduplication
- **Real-time streaming:** Every token appears in the browser as the LLM generates it via WebSocket
- **Interactive knowledge graph:** D3.js force-directed graph with supports/contradicts/cites edges, filters, search, and node details
- **Session persistence:** Debates auto-save as JSON; view and replay past sessions from the dashboard
- **Exportable reports:** Download full debate transcripts as Markdown or printable HTML
- **Configurable scoring:** Adjust evidence/logic/novelty weights for the judge
- **Single binary:** Go compiles to one executable; just bring your API key

## Quick Start

### Prerequisites

- Go 1.25+
- An OpenAI-compatible API key (OpenAI, Groq, Together, Anthropic via proxy, etc.)

### Run

```powershell
# 1. Set your API key
$env:KAMPONG_LLM_KEY = "sk-..."

# 2. (Optional) Set a Tavily key for better search
$env:KAMPONG_TAVILY_KEY = "tvly-..."

# 3. Run
go run ./cmd/server
```

Open **http://localhost:9090** in your browser. Type a topic, pick a mode, and press **Start Debate**.

### Build a standalone binary

```powershell
.\build.ps1
# Or:
go build -o kampong.exe ./cmd/server
```

The binary is self-contained — just run it alongside `config.yaml` and the `web/` directory.

## Configuration

Edit `config.yaml` or set environment variables:

| Setting | Env Var | Default | Description |
|---------|---------|---------|-------------|
| `llm.api_key` | `KAMPONG_LLM_KEY` | — | OpenAI-compatible API key (required) |
| `llm.base_url` | — | `https://api.openai.com/v1` | API endpoint |
| `llm.model` | — | `gpt-4o` | Primary model for agents and judge |
| `llm.fast_model` | — | `gpt-4o-mini` | Model for lighter tasks (research planning) |
| `search.tavily_api_key` | `KAMPONG_TAVILY_KEY` | — | Tavily API key (falls back to free DuckDuckGo) |
| `debate.default_mode` | — | `deep` | Default debate mode |
| `debate.panel.total_agents` | — | `6` | Total agents including judge |
| `server.port` | — | `9090` | HTTP server port |

### Panel composition

| Option | Default | Description |
|--------|---------|-------------|
| `core_count` | 2 | Deep subject-matter experts |
| `adjacent_count` | 2 | Cross-domain thinkers |
| `practitioner_count` | 1 | Real-world practitioners |
| `lay_count` | 1 | Common-sense / outsider perspective |

### Scoring weights

| Option | Default | Description |
|--------|---------|-------------|
| `evidence_weight` | 0.40 | Weight of cited sources and data quality |
| `logic_weight` | 0.35 | Weight of reasoning and internal consistency |
| `novelty_weight` | 0.25 | Weight of original contributions vs rehashing |

## Architecture

```
cmd/server/main.go          Entry point — wires everything, starts HTTP server
internal/
├── agent/                   Agent runtime: prompt templates, multi-round research planner
├── analyzer/                Topic analyzer: suggests panel composition from the topic
├── api/                     HTTP API: chi router, REST endpoints, WebSocket handler
├── config/                  YAML config loading with env var overrides
├── debate/                  Core engine: round execution, judge evaluation, cross-examination
├── graph/                   Knowledge graph: LLM extractor, NLP concept extraction, auto-connect, store
├── llm/                     OpenAI-compatible client with streaming SSE support
├── models/                  Data types: Agent, Session, Verdict, GraphNode/Edge, etc.
├── panel/                   Panel composer: builds the agent roster from analyzer output
├── report/                  Markdown and HTML report generation
├── search/                  Web search via DuckDuckGo (free) or Tavily API
├── store/                   JSON file-based session persistence
└── ws/                      WebSocket hub: per-debate pub/sub broadcasting
web/
├── index.html               Single-page app with landing dashboard and arena views
├── css/style.css             Full styling
└── js/
    ├── app.js                App controller: debate lifecycle, settings, history
    ├── arena.js              Arena UI: feed rendering, scroll management, phase banners
    ├── graph.js              D3.js force graph: interactive knowledge graph visualization
    └── ws.js                 WebSocket client: event routing to UI handlers
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/debate/start` | Start a new debate (`{"topic":"...", "mode":"deep"}`) |
| `GET` | `/api/debate/{id}/status` | Current debate status and panel |
| `GET` | `/api/debate/{id}/history` | Full transcript |
| `GET` | `/api/debate/{id}/graph` | Knowledge graph nodes and edges |
| `GET` | `/api/debate/{id}/report` | Download report (`?format=md` or `html`) |
| `POST` | `/api/debate/{id}/cancel` | Cancel a running debate |
| `GET` | `/api/debates` | List all past debates |
| `DELETE` | `/api/debate/{id}` | Delete a saved debate |
| `GET` | `/api/config` | Current configuration |
| `PUT` | `/api/config` | Update config at runtime |
| `GET` | `/ws/debate/{id}` | WebSocket for live debate events |

### WebSocket Events

| Event | Direction | Description |
|-------|-----------|-------------|
| `phase_change` | server → client | New debate phase (analyzing, opening, rebuttal, verdict, etc.) |
| `panel_ready` | server → client | Agent panel composed and ready |
| `agent_searching` | server → client | Agent is searching the web |
| `agent_thinking` | server → client | Search results received, about to speak |
| `research_round` | server → client | Each round of iterative research with findings |
| `agent_speaking` | server → client | Streaming token from agent's argument |
| `agent_done` | server → client | Agent finished their argument |
| `graph_update` | server → client | New nodes/edges extracted from the argument |
| `verdict` | server → client | Judge's final evaluation with scores |
| `synthesis` | server → client | Synthesizer's discussion insights |
| `error` | server → client | Error with code and message |

## Design Decisions

- **Single binary** — Go compiles everything including the static frontend into one executable. No Node.js, no npm, no Docker required.
- **DuckDuckGo fallback** — Web search works out of the box with zero API keys via DuckDuckGo. Tavily available for higher-quality results.
- **Knowledge graph as a sidecar** — The graph extractor runs after each argument but the debate engine doesn't block on it. If extraction fails, the debate continues.
- **Reconnection support** — WebSocket handler replays the full debate state (phase, panel, transcript, graph, verdict) when a client reconnects.
- **No database** — Sessions persist as JSON files under `data/debates/`. No external database dependency.

## License

MIT
