// arena.js — Debate arena UI management
let currentAgents = [];
let streamingBubble = null;
let streamingAgentId = null;
let streamingText = '';
let activeAgentId = null;
let currentThinkingCard = null;  // the active agent's thinking card
let autoScrollEnabled = true;  // User can toggle this
let userScrolledUp = false;    // Track if user intentionally scrolled up
let programmaticScroll = false; // Flag to distinguish programmatic vs user scroll
let scrollDebounceTimer = null; // Debounce timer for streaming scroll calls

// ── Auto-scroll helper ──
function scrollFeed(force = false) {
  const feed = document.getElementById('debate-feed');
  if (!feed) return;

  // If forced (e.g., new agent card), always scroll smoothly
  if (force) {
    programmaticScroll = true;
    feed.scrollTop = feed.scrollHeight;
    setTimeout(() => { programmaticScroll = false; }, 100);
    hideNewMessageIndicator();
    return;
  }

  // Only auto-scroll if enabled and user hasn't scrolled up intentionally
  if (!autoScrollEnabled || userScrolledUp) {
    showNewMessageIndicator();
    return;
  }

  programmaticScroll = true;
  feed.scrollTop = feed.scrollHeight;
  setTimeout(() => { programmaticScroll = false; }, 50);
  hideNewMessageIndicator();
}

// Debounced scroll for streaming (called on every token)
function scrollFeedDebounced() {
  if (scrollDebounceTimer) return;
  scrollDebounceTimer = setTimeout(() => {
    scrollDebounceTimer = null;
    scrollFeed(false);
  }, 150);
}

// Show/hide new message indicator
function showNewMessageIndicator() {
  const indicator = document.getElementById('new-message-indicator');
  if (indicator && !autoScrollEnabled) {
    indicator.classList.remove('hidden');
  }
}

function hideNewMessageIndicator() {
  const indicator = document.getElementById('new-message-indicator');
  if (indicator) {
    indicator.classList.add('hidden');
  }
}

// Track user scroll behavior
function setupScrollTracking() {
  const feed = document.getElementById('debate-feed');
  if (!feed) return;

  // Reset state
  userScrolledUp = false;
  programmaticScroll = false;

  feed.addEventListener('scroll', () => {
    // Ignore scroll events caused by our own scrollFeed() calls
    if (programmaticScroll) return;

    const distFromBottom = feed.scrollHeight - feed.scrollTop - feed.clientHeight;

    if (distFromBottom > 200) {
      userScrolledUp = true;
      showNewMessageIndicator();
    } else {
      userScrolledUp = false;
      hideNewMessageIndicator();
    }
  });
}

// Toggle auto-scroll from UI
function toggleAutoScroll() {
  autoScrollEnabled = !autoScrollEnabled;
  const btn = document.getElementById('autoscroll-toggle');
  if (btn) {
    btn.classList.toggle('active', autoScrollEnabled);
    btn.title = autoScrollEnabled ? 'Auto-scroll ON' : 'Auto-scroll OFF';
  }
  if (autoScrollEnabled) {
    userScrolledUp = false;
    hideNewMessageIndicator();
    scrollFeed(true);
  }
}

// ── Phase change ──
function onPhaseChange(data) {
  const indicator = document.getElementById('phase-indicator');
  const roundCounter = document.getElementById('round-counter');

  if (indicator) {
    const phase = data.phase || '';
    // Format phase name nicely
    const phaseName = phase.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
    indicator.textContent = phaseName;
  }
  if (roundCounter && data.round && data.total_rounds) {
    roundCounter.textContent = `Step ${data.round}/${data.total_rounds}`;
  }

  // Update progress bar
  updateProgress(data);

  if (data.message) {
    addPhaseBanner(data.message, data.phase);
  }
}

function updateProgress(data) {
  const progressFill = document.getElementById('debate-progress-fill');
  const progressText = document.getElementById('debate-progress-text');
  
  if (!progressFill || !progressText) return;
  
  const totalRounds = data.total_rounds || 5;
  const currentRound = data.round || 0;
  const phase = data.phase || ''
  
  // Calculate progress based on round and phase
  let progress = 0;
  if (phase === 'verdict') {
    progress = 100;
  } else if (currentRound > 0) {
    // Each round is worth roughly (100 / totalRounds) percent
    // Add partial progress within a round
    progress = Math.min((currentRound / totalRounds) * 100, 95);
  } else {
    // Still in initialization
    progress = 5;
  }
  
  progressFill.style.width = `${progress}%`;
  progressText.textContent = `${Math.round(progress)}%`;
}

function addPhaseBanner(message, phase) {
  const feed = document.getElementById('feed-content');
  if (!feed) return;
  const div = document.createElement('div');
  div.className = 'phase-banner';
  div.innerHTML = `<span class="phase-banner-icon">${phaseIcon(phase)}</span><span>${esc(message)}</span>`;
  feed.appendChild(div);
  scrollFeed();
}

function phaseIcon(phase) {
  if (!phase) return '📢';
  if (phase.includes('opening')) return '🎤';
  if (phase.includes('rebuttal')) return '⚔️';
  if (phase.includes('cross')) return '❓';
  if (phase.includes('closing')) return '🏁';
  if (phase.includes('verdict')) return '⚖️';
  if (phase.includes('perspective')) return '🔍';
  if (phase.includes('deep_dive')) return '🤿';
  if (phase.includes('cross_pollination')) return '🌸';
  if (phase.includes('synthesis')) return '🧩';
  if (phase.includes('paused')) return '⏸';
  if (phase.includes('resumed')) return '▶';
  return '📢';
}

// ── Panel ready ──
function onPanelReady(agents) {
  currentAgents = agents;
  const panelList = document.getElementById('panel-list');
  if (!panelList) return;

  panelList.innerHTML = '';
  for (const a of agents) {
    const div = document.createElement('div');
    div.className = 'panel-agent';
    div.id = 'panel-agent-' + a.id;

    // Determine role badge color
    let roleColor = 'var(--text-muted)';
    let roleLabel = a.role_type || 'agent';
    if (a.role_type === 'core') {
      roleColor = 'var(--accent)';
      roleLabel = 'Core';
    } else if (a.role_type === 'adjacent') {
      roleColor = 'var(--purple)';
      roleLabel = 'Adjacent';
    } else if (a.role_type === 'practitioner') {
      roleColor = 'var(--green)';
      roleLabel = 'Practitioner';
    } else if (a.role_type === 'layperson') {
      roleColor = 'var(--orange)';
      roleLabel = 'Layperson';
    } else if (a.role_type === 'judge' || a.role_type === 'synthesizer') {
      roleColor = 'var(--text-muted)';
      roleLabel = a.role_type === 'judge' ? 'Judge' : 'Synthesizer';
    }

    div.innerHTML = `
      <span class="agent-icon">${a.icon || '👤'}</span>
      <div class="agent-info">
        <div class="agent-name">${esc(a.name)}</div>
        <div class="agent-role-badge" style="background:${roleColor}20; color:${roleColor}">${roleLabel}</div>
        ${a.provider_name ? `<div class="agent-provider" title="Using ${esc(a.provider_name)}">⚡ ${esc(a.provider_name)}</div>` : ''}
      </div>
    `;
    panelList.appendChild(div);
  }
}

// ── Agent searching (new: animated progress card) ──
function onAgentSearching(data) {
  const agent = currentAgents.find(a => a.id === data.agent_id);
  const feed = document.getElementById('feed-content');
  if (!feed) return;

  const card = document.createElement('div');
  card.className = 'agent-turn-card';
  card.id = 'turn-' + data.agent_id;
  
  // Add role data attribute for color coding
  if (agent && agent.role) {
    const role = agent.role.toLowerCase();
    if (role.includes('pro') || role.includes('support')) {
      card.dataset.agentRole = 'pro';
    } else if (role.includes('con') || role.includes('against')) {
      card.dataset.agentRole = 'con';
    } else {
      card.dataset.agentRole = 'neutral';
    }
  }

  const query = data.query || 'gathering research...';
  card.innerHTML = `
    <div class="agent-card-header">
      <span class="agent-card-icon">${agent ? agent.icon : '🤖'}</span>
      <span class="agent-card-name">${esc(agent ? agent.name : 'Unknown')}</span>
      <span class="agent-card-role">${esc(agent ? agent.expertise : '')}</span>
    </div>
    <div class="agent-card-search">
      <span class="search-spinner"></span>
      <span class="search-query">🔍 <em>"${esc(query)}"</em></span>
    </div>
  `;

  currentThinkingCard = card;
  feed.appendChild(card);
  scrollFeed(true);  // Force scroll to new card
}

// ── Research round (agentic multi-step research progress) ──
function onResearchRound(data) {
  if (!currentThinkingCard) return;

  const round = data.round || 1;
  const queries = data.queries || [];
  const results = data.results || [];
  const findings = data.findings || [];
  const reasoning = data.reasoning || '';

  // Find or create the research progress section in the agent card
  let researchSection = currentThinkingCard.querySelector('.research-progress');
  if (!researchSection) {
    researchSection = document.createElement('div');
    researchSection.className = 'research-progress';
    // Insert after the search section
    const searchEl = currentThinkingCard.querySelector('.agent-card-search');
    if (searchEl) {
      searchEl.after(researchSection);
    } else {
      currentThinkingCard.appendChild(researchSection);
    }
  }

  // Build round display
  let roundHtml = `<div class="research-round">`;
  roundHtml += `<div class="research-round-header">`;
  roundHtml += `<span class="research-round-badge">🔬 Research Round ${round}</span>`;
  if (reasoning) {
    roundHtml += `<span class="research-reasoning">${esc(reasoning)}</span>`;
  }
  roundHtml += `</div>`;

  // Show queries
  if (queries.length > 0) {
    roundHtml += `<div class="research-queries">`;
    roundHtml += queries.map(q => `<span class="research-query">🔍 <em>"${esc(q)}"</em></span>`).join('');
    roundHtml += `</div>`;
  }

  // Show results count
  if (results.length > 0) {
    roundHtml += `<div class="research-results-count">📚 Found <strong>${results.length}</strong> new source${results.length !== 1 ? 's' : ''}</div>`;
  }

  // Show key findings
  if (findings.length > 0) {
    roundHtml += `<div class="research-findings">`;
    roundHtml += findings.map(f => `<div class="research-finding">💡 ${esc(f)}</div>`).join('');
    roundHtml += `</div>`;
  }

  roundHtml += `</div>`;
  researchSection.innerHTML += roundHtml;

  scrollFeed();
}

// ── Agent thinking (research results integrated into agent card) ──
function onAgentThinking(data) {
  if (!currentThinkingCard) return;

  const results = data.search_results || [];
  const resultCount = results.length;

  // Replace search spinner with results
  const searchEl = currentThinkingCard.querySelector('.agent-card-search');
  if (searchEl) {
    const query = data.search_query || '';
    let resultsHtml = '';
    if (resultCount > 0) {
      resultsHtml = `<div class="search-summary">📚 Found <strong>${resultCount}</strong> source${resultCount !== 1 ? 's' : ''}</div>`;
      resultsHtml += '<div class="search-results-list">';
      resultsHtml += results.map((r, i) => {
        const snip = (r.snippet || '').substring(0, 120);
        return `<div class="sr-item" title="${esc(r.snippet || '')}">
          <span class="sr-index">${i + 1}</span>
          <div class="sr-body">
            <div class="sr-title">${esc(r.title || 'Untitled')}</div>
            <div class="sr-snippet">${esc(snip)}${snip.length >= 120 ? '…' : ''}</div>
            ${r.url ? `<a class="sr-url" href="${esc(safeUrl(r.url))}" target="_blank" onclick="event.stopPropagation()">↗ source</a>` : ''}
          </div>
        </div>`;
      }).join('');
      resultsHtml += '</div>';
    } else {
      resultsHtml = '<div class="search-summary empty">📭 No fresh results — relying on expertise</div>';
    }
    searchEl.innerHTML = resultsHtml;
  }

  scrollFeed();
}

// ── Agent speaking (speech bubble inside agent card) ──
function onAgentSpeaking(data) {
  // Highlight speaking agent in sidebar
  if (activeAgentId !== data.agent_id) {
    if (activeAgentId) {
      const prev = document.getElementById('panel-agent-' + activeAgentId);
      if (prev) prev.classList.remove('speaking');
    }
    activeAgentId = data.agent_id;
    const curr = document.getElementById('panel-agent-' + activeAgentId);
    if (curr) curr.classList.add('speaking');
  }

  // Add active-speaking class to the card
  if (currentThinkingCard) {
    currentThinkingCard.classList.add('active-speaking');
  }

  // Create or update streaming bubble inside the agent card
  if (!streamingBubble || streamingAgentId !== data.agent_id) {
    // Finalize previous streaming bubble
    if (streamingBubble) {
      streamingBubble.classList.remove('streaming');
      const cursor = streamingBubble.querySelector('.typing-cursor');
      if (cursor) cursor.remove();
    }

    streamingAgentId = data.agent_id;
    streamingText = '';

    // Remove old speech bubble if any
    const oldBubble = currentThinkingCard ? currentThinkingCard.querySelector('.speech-bubble') : null;
    if (oldBubble) oldBubble.remove();

    const container = currentThinkingCard || document.getElementById('feed-content');
    streamingBubble = document.createElement('div');
    streamingBubble.className = 'speech-bubble streaming';
    streamingBubble.innerHTML = '<div class="speech-content"></div>';
    container.appendChild(streamingBubble);
    scrollFeed();
  }

  streamingText += data.token || '';
  const content = streamingBubble.querySelector('.speech-content');
  content.textContent = streamingText;

  // Typing cursor
  let cursor = streamingBubble.querySelector('.typing-cursor');
  if (!cursor) {
    cursor = document.createElement('span');
    cursor.className = 'typing-cursor';
    content.appendChild(cursor);
  }

  // Use debounced scroll during streaming to avoid smooth-scroll pileup
  scrollFeedDebounced();
}

// ── Agent done ──
function onAgentDone(data) {
  // Finalize streaming bubble
  if (streamingBubble) {
    streamingBubble.classList.remove('streaming');
    const cursor = streamingBubble.querySelector('.typing-cursor');
    if (cursor) cursor.remove();
    streamingBubble = null;
    streamingAgentId = null;
    streamingText = '';
  }

  // Remove active-speaking class from the card
  if (currentThinkingCard) {
    currentThinkingCard.classList.remove('active-speaking');
  }

  // Add search count badge to the agent card
  if (currentThinkingCard && data.search_count > 0) {
    const header = currentThinkingCard.querySelector('.agent-card-header');
    if (header) {
      let badge = header.querySelector('.search-count-badge');
      if (!badge) {
        badge = document.createElement('span');
        badge.className = 'search-count-badge';
        header.appendChild(badge);
      }
      badge.textContent = `${data.search_count} source${data.search_count !== 1 ? 's' : ''}`;
    }
  }

  // Add confidence badge to the agent card
  if (currentThinkingCard && data.confidence != null) {
    const header = currentThinkingCard.querySelector('.agent-card-header');
    if (header) {
      const confBadge = document.createElement('span');
      const conf = Math.round(data.confidence);
      let confColor = 'var(--red)';
      if (conf > 70) confColor = 'var(--green)';
      else if (conf >= 40) confColor = 'var(--orange)';
      confBadge.className = 'confidence-badge';
      confBadge.style.background = confColor + '20';
      confBadge.style.color = confColor;
      confBadge.textContent = `🎯 ${conf}/100`;
      header.appendChild(confBadge);
    }
  }

  // Add vendor-family chip (Phase 2: vendor diversity is visible).
  if (currentThinkingCard && data.vendor_family) {
    const header = currentThinkingCard.querySelector('.agent-card-header');
    if (header && !header.querySelector('.vendor-chip')) {
      const chip = document.createElement('span');
      chip.className = 'vendor-chip';
      const fam = String(data.vendor_family);
      const prov = data.provider_name ? ` · ${data.provider_name}` : '';
      chip.textContent = `${fam}${prov}`;
      chip.title = `Model family: ${fam}`;
      header.appendChild(chip);
    }
  }

  currentThinkingCard = null;

  // Remove speaking highlight
  if (activeAgentId === data.agent_id) {
    const prev = document.getElementById('panel-agent-' + activeAgentId);
    if (prev) prev.classList.remove('speaking');
    activeAgentId = null;
  }

  scrollFeed();
}

// ── Graph update ──
function onGraphUpdate(data) {
  if (data.new_nodes || data.new_edges) {
    addGraphData(data.new_nodes || [], data.new_edges || []);
  }
}

// ── Verdict ──
function onVerdict(data) {
  const overlay = document.getElementById('verdict-overlay');
  overlay.classList.remove('hidden');

  const winnerDiv = document.getElementById('verdict-winner');
  const winner = currentAgents.find(a => a.id === data.winner_agent_id);
  winnerDiv.innerHTML = `
    <span class="winner-icon">${winner ? winner.icon : '🏆'}</span>
    ${esc(winner ? winner.name : 'Unknown')} — ${esc(winner ? winner.role : '')}
  `;

  const scorecardDiv = document.getElementById('verdict-scorecard');
  scorecardDiv.innerHTML = '<h3>Scorecard</h3>';
  const scores = data.scorecard || [];
  scores.sort((a, b) => b.total - a.total);
  for (const s of scores) {
    scorecardDiv.innerHTML += `
      <div class="score-row">
        <span class="score-name">${esc(s.agent_name || 'Unknown')}</span>
        <span class="score-bars">
          <div class="score-bar-bg"><div class="score-bar-fill" style="width:${s.evidence*10}%;background:#60a5fa"></div></div>
          <div class="score-bar-bg"><div class="score-bar-fill" style="width:${s.logic*10}%;background:#4ade80"></div></div>
          <div class="score-bar-bg"><div class="score-bar-fill" style="width:${s.novelty*10}%;background:#fb923c"></div></div>
        </span>
        <span class="score-total">${s.total ? s.total.toFixed(1) : '—'}</span>
      </div>
    `;
  }
  scorecardDiv.innerHTML += `
    <div style="display:flex;gap:1rem;font-size:0.7rem;color:var(--text-dim);margin-top:0.3rem;padding-left:140px">
      <span style="color:#60a5fa">■ Evidence</span>
      <span style="color:#4ade80">■ Logic</span>
      <span style="color:#fb923c">■ Novelty</span>
    </div>
  `;

  // Consolidated Claims
  const claimsDiv = document.getElementById('verdict-consolidated-claims');
  if (claimsDiv && data.consolidated_claims && data.consolidated_claims.length > 0) {
    claimsDiv.innerHTML = '<h3>📋 Consolidated Findings</h3>';
    claimsDiv.innerHTML += data.consolidated_claims.map(c => {
      const statusColors = {
        'established': 'var(--green)',
        'contested': 'var(--orange)',
        'disputed': 'var(--red)',
        'unresolved': 'var(--text-dim)'
      };
      const statusIcons = {
        'established': '✅',
        'contested': '⚔️',
        'disputed': '❌',
        'unresolved': '❓'
      };
      const color = statusColors[c.status] || 'var(--text-dim)';
      const icon = statusIcons[c.status] || '•';
      let html = `<div class="consolidated-claim">`;
      html += `<div class="claim-header"><span class="claim-status" style="color:${color}">${icon} ${esc(c.status)}</span></div>`;
      html += `<div class="claim-text">${esc(c.claim)}</div>`;
      if (c.support && c.support.length > 0) {
        html += `<div class="claim-actors"><span class="claim-support">Supported by:</span> ${c.support.map(a => esc(a)).join(', ')}</div>`;
      }
      if (c.oppose && c.oppose.length > 0) {
        html += `<div class="claim-actors"><span class="claim-oppose">Opposed by:</span> ${c.oppose.map(a => esc(a)).join(', ')}</div>`;
      }
      if (c.evidence) {
        html += `<div class="claim-evidence">${esc(c.evidence)}</div>`;
      }
      html += `</div>`;
      return html;
    }).join('');
  }

  // Consensus Points
  const consensusDiv = document.getElementById('verdict-consensus');
  if (consensusDiv && data.consensus_points && data.consensus_points.length > 0) {
    consensusDiv.innerHTML = '<h3>🤝 Consensus</h3><ul>' +
      data.consensus_points.map(p => `<li>${esc(p)}</li>`).join('') + '</ul>';
  }

  // Unresolved Questions
  const unresolvedDiv = document.getElementById('verdict-unresolved');
  if (unresolvedDiv && data.unresolved_questions && data.unresolved_questions.length > 0) {
    unresolvedDiv.innerHTML = '<h3>❓ Unresolved Questions</h3><ul>' +
      data.unresolved_questions.map(q => `<li>${esc(q)}</li>`).join('') + '</ul>';
  }

  // Synthesis
  const synthesisDiv = document.getElementById('verdict-synthesis');
  if (synthesisDiv && data.synthesis) {
    synthesisDiv.innerHTML = '<h3>📖 Debate Synthesis</h3><div class="synthesis-text">' +
      esc(data.synthesis).split('\n').filter(p => p.trim()).map(p => `<p>${esc(p)}</p>`).join('') + '</div>';
  }

  const turningDiv = document.getElementById('verdict-turning-points');
  turningDiv.innerHTML = '<h3>Key Turning Points</h3><ul>' +
    (data.turning_points || []).map(t => `<li>${esc(t)}</li>`).join('') +
    '</ul>';

  const reasoningDiv = document.getElementById('verdict-reasoning');
  reasoningDiv.innerHTML = '<h3>Judge\'s Reasoning</h3><p>' + esc(data.reasoning || 'No reasoning provided.') + '</p>';
}

// ── Error ──
function onError(data) {
  const feed = document.getElementById('feed-content');
  if (!feed) return;
  const div = document.createElement('div');
  div.className = 'error-banner';
  div.innerHTML = `<span>⚠️</span> ${esc(data.message || 'Unknown error')}`;
  feed.appendChild(div);
  scrollFeed();
}

function closeVerdict() {
  document.getElementById('verdict-overlay').classList.add('hidden');
}

// ── Export Report ──
function exportReport(format) {
  // Get the current debate ID from the WebSocket connection
  const debateId = wsDebateId;
  if (!debateId) {
    alert('No active debate to export.');
    return;
  }

  if (format === 'md') {
    // Download markdown file
    window.open(`/api/debate/${debateId}/report?format=md`, '_blank');
  } else if (format === 'html') {
    // Open HTML in new window for printing/PDF
    const printWindow = window.open(`/api/debate/${debateId}/report?format=html`, '_blank');
    if (printWindow) {
      printWindow.onload = () => {
        setTimeout(() => printWindow.print(), 500);
      };
    }
  }
}

// ── Discussion Synthesis ──
function onSynthesis(data) {
  const overlay = document.getElementById('verdict-overlay');
  overlay.classList.remove('hidden');

  // Change title for discussion mode
  document.getElementById('verdict-title').textContent = '🧩 Discussion Synthesis';

  // Hide debate-specific sections
  const winnerDiv = document.getElementById('verdict-winner');
  winnerDiv.style.display = 'none';
  const scorecardDiv = document.getElementById('verdict-scorecard');
  scorecardDiv.style.display = 'none';
  document.getElementById('verdict-turning-points').innerHTML = '';
  document.getElementById('verdict-reasoning').innerHTML = '';
  document.getElementById('verdict-consolidated-claims').innerHTML = '';
  document.getElementById('verdict-consensus').innerHTML = '';
  document.getElementById('verdict-unresolved').innerHTML = '';
  document.getElementById('verdict-synthesis').innerHTML = '';

  // Key Insights
  const insightsDiv = document.getElementById('synthesis-insights');
  if (data.key_insights && data.key_insights.length > 0) {
    insightsDiv.innerHTML = '<h3>💡 Key Insights</h3>';
    insightsDiv.innerHTML += data.key_insights.map(ins => `
      <div class="insight-card">
        <div class="insight-theme">${esc(ins.theme)}</div>
        <div class="insight-finding">${esc(ins.finding)}</div>
        ${ins.support_by && ins.support_by.length > 0 ? `<div class="insight-by">Supported by: ${ins.support_by.map(a => esc(a)).join(', ')}</div>` : ''}
        ${ins.evidence ? `<div class="insight-evidence">${esc(ins.evidence)}</div>` : ''}
      </div>
    `).join('');
  }

  // Common Themes
  const themesDiv = document.getElementById('synthesis-themes');
  if (data.common_themes && data.common_themes.length > 0) {
    themesDiv.innerHTML = '<h3>🤝 Common Themes</h3><ul>' +
      data.common_themes.map(t => `<li>${esc(t)}</li>`).join('') + '</ul>';
  }

  // Surprising Findings
  const surprisingDiv = document.getElementById('synthesis-surprising');
  if (data.surprising_findings && data.surprising_findings.length > 0) {
    surprisingDiv.innerHTML = '<h3>😮 Surprising Findings</h3><ul>' +
      data.surprising_findings.map(f => `<li>${esc(f)}</li>`).join('') + '</ul>';
  }

  // Open Questions
  const questionsDiv = document.getElementById('synthesis-questions');
  if (data.open_questions && data.open_questions.length > 0) {
    questionsDiv.innerHTML = '<h3>❓ Open Questions</h3><ul>' +
      data.open_questions.map(q => `<li>${esc(q)}</li>`).join('') + '</ul>';
  }

  // Overview
  const overviewDiv = document.getElementById('synthesis-overview');
  if (data.overview) {
    overviewDiv.innerHTML = '<h3>📖 Overview</h3><div class="synthesis-text">' +
      data.overview.split('\n').filter(p => p.trim()).map(p => `<p>${esc(p)}</p>`).join('') + '</div>';
  }
}

// ── Debate Paused ──
function onDebatePaused(data) {
  const banner = document.getElementById('pause-banner');
  if (banner) banner.classList.remove('hidden');
  const icon = document.getElementById('pause-resume-icon');
  if (icon) icon.textContent = '▶';
  const btn = document.getElementById('btn-pause-resume');
  if (btn) btn.title = 'Resume debate';
  addPhaseBanner(data.message || 'Debate paused by user', 'paused');
}

// ── Debate Resumed ──
function onDebateResumed(data) {
  const banner = document.getElementById('pause-banner');
  if (banner) banner.classList.add('hidden');
  const icon = document.getElementById('pause-resume-icon');
  if (icon) icon.textContent = '⏸';
  const btn = document.getElementById('btn-pause-resume');
  if (btn) btn.title = 'Pause debate';
  addPhaseBanner(data.message || 'Debate resumed', 'resumed');
}

// ── User Injection ──
function onUserInjection(data) {
  const feed = document.getElementById('feed-content');
  if (!feed) return;
  const card = document.createElement('div');
  card.className = 'injection-card';
  const agent = data.target_agent_id
    ? currentAgents.find(a => a.id === data.target_agent_id)
    : null;
  const targetLabel = agent ? agent.name : 'All agents';
  card.innerHTML = `
    <div class="injection-header">
      <span class="injection-icon">💉</span>
      <span class="injection-label">User Evidence Injected</span>
      <span class="injection-target">→ ${esc(targetLabel)}</span>
    </div>
    <div class="injection-body">${esc(data.text || '')}</div>
  `;
  feed.appendChild(card);
  scrollFeed(true);
}

// ── Claim Challenged ──
function onClaimChallenged(data) {
  const feed = document.getElementById('feed-content');
  if (!feed) return;
  const card = document.createElement('div');
  card.className = 'challenge-card';
  card.innerHTML = `
    <div class="challenge-header">
      <span class="challenge-icon">🎯</span>
      <span class="challenge-label">Claim Challenged</span>
    </div>
    <div class="challenge-claim-text">"${esc(data.claim_text || data.transcript_entry_id || '')}"</div>
    <div class="challenge-reason">${esc(data.challenge_text || '')}</div>
  `;
  feed.appendChild(card);
  scrollFeed(true);
}

// ── Post-Debate Analysis ──
function onPostDebateAnalysis(data) {
  // Calibration table
  const calibDiv = document.getElementById('analysis-calibration');
  if (calibDiv && data.calibration && data.calibration.length > 0) {
    let html = '<h4 class="analysis-subtitle">🎯 Calibration</h4>';
    html += '<table class="calibration-table"><thead><tr><th>Agent</th><th>Avg Confidence</th><th>Judge Score</th><th>Ratio</th></tr></thead><tbody>';
    for (const c of data.calibration) {
      const ratio = c.avg_confidence > 0 ? (c.judge_score / c.avg_confidence).toFixed(2) : '—';
      html += `<tr>
        <td>${esc(c.agent_name)}</td>
        <td>${Math.round(c.avg_confidence)}</td>
        <td>${c.judge_score ? c.judge_score.toFixed(1) : '—'}</td>
        <td>${ratio}</td>
      </tr>`;
    }
    html += '</tbody></table>';
    calibDiv.innerHTML = html;
  }

  // Consistency
  const consisDiv = document.getElementById('analysis-consistency');
  if (consisDiv && data.consistency && data.consistency.length > 0) {
    let html = '<h4 class="analysis-subtitle">🔄 Consistency</h4>';
    for (const s of data.consistency) {
      const pct = Math.round((s.consistency || 0) * 100);
      let barColor = 'var(--green)';
      if (pct < 50) barColor = 'var(--red)';
      else if (pct < 75) barColor = 'var(--orange)';
      html += `<div class="consistency-item">
        <div class="consistency-label">
          <span>${esc(s.agent_name)}</span>
          <span class="consistency-pct">${pct}%</span>
        </div>
        <div class="consistency-bar">
          <div class="consistency-bar-fill" style="width:${pct}%;background:${barColor}"></div>
        </div>
        ${s.contradictions && s.contradictions.length > 0
          ? `<div class="consistency-details">${s.contradictions.map(c => `<span class="contradiction-item">⚠ ${esc(c)}</span>`).join('')}</div>`
          : ''}
      </div>`;
    }
    consisDiv.innerHTML = html;
  }

  // Auto-Critique
  const critDiv = document.getElementById('analysis-critique');
  if (critDiv && data.critique) {
    const cr = data.critique;
    let qualityColor = 'var(--green)';
    if (cr.quality === 'poor') qualityColor = 'var(--red)';
    else if (cr.quality === 'fair') qualityColor = 'var(--orange)';
    else if (cr.quality === 'good') qualityColor = 'var(--blue)';
    let html = '<h4 class="analysis-subtitle">📝 Auto-Critique</h4>';
    html += `<div class="critique-card">
      <div class="critique-header">
        <span class="quality-badge" style="background:${qualityColor}20;color:${qualityColor}">${esc(cr.quality || 'N/A').toUpperCase()}</span>
        ${cr.diversity_score != null ? `<span class="diversity-score">Diversity: ${Math.round(cr.diversity_score * 100)}%</span>` : ''}
      </div>`;
    if (cr.strengths && cr.strengths.length > 0) {
      html += `<div class="critique-section"><h5>✅ Strengths</h5><ul>${cr.strengths.map(s => `<li>${esc(s)}</li>`).join('')}</ul></div>`;
    }
    if (cr.weaknesses && cr.weaknesses.length > 0) {
      html += `<div class="critique-section"><h5>⚠ Weaknesses</h5><ul>${cr.weaknesses.map(w => `<li>${esc(w)}</li>`).join('')}</ul></div>`;
    }
    html += '</div>';
    critDiv.innerHTML = html;
  }
}

// ── Social Simulation ──
function onSocialSimulation(data) {
  const resultsDiv = document.getElementById('social-results');
  if (!resultsDiv) return;
  const btn = document.getElementById('btn-social-sim');
  if (btn) btn.style.display = 'none';

  let html = '';
  if (data.overall_sentiment) {
    let sentColor = 'var(--green)';
    if (data.overall_sentiment === 'negative') sentColor = 'var(--red)';
    else if (data.overall_sentiment === 'mixed') sentColor = 'var(--orange)';
    html += `<div class="sentiment-overview">
      <span class="sentiment-indicator" style="color:${sentColor}">${esc(data.overall_sentiment).toUpperCase()}</span>
      <span class="sentiment-label">Overall Public Sentiment</span>
    </div>`;
  }
  if (data.persona_reactions && data.persona_reactions.length > 0) {
    html += '<div class="social-grid">';
    for (const p of data.persona_reactions) {
      let sentBadgeColor = 'var(--green)';
      if (p.sentiment === 'negative') sentBadgeColor = 'var(--red)';
      else if (p.sentiment === 'neutral') sentBadgeColor = 'var(--text-muted)';
      else if (p.sentiment === 'mixed') sentBadgeColor = 'var(--orange)';
      html += `<div class="social-reaction-card">
        <div class="social-card-header">
          <span class="persona-name">${esc(p.name)}</span>
          <span class="sentiment-badge" style="background:${sentBadgeColor}20;color:${sentBadgeColor}">${esc(p.sentiment || 'neutral')}</span>
        </div>
        <div class="persona-type">${esc(p.type || '')}</div>
        <div class="persona-reaction">${esc(p.reaction || '')}</div>
      </div>`;
    }
    html += '</div>';
  }
  if (data.narrative_summary) {
    html += `<div class="narrative-summary">
      <h4 class="analysis-subtitle">📖 Narrative Summary</h4>
      <p>${esc(data.narrative_summary)}</p>
    </div>`;
  }
  if (data.risk_areas && data.risk_areas.length > 0) {
    html += `<div class="risk-areas">
      <h4 class="analysis-subtitle">⚠ Misinformation Risk Areas</h4>
      <ul>${data.risk_areas.map(r => `<li class="risk-item">⚠ ${esc(r)}</li>`).join('')}</ul>
    </div>`;
  }
  resultsDiv.innerHTML = html;
}

// ── Collapsible section toggle ──
function toggleCollapsible(headerEl) {
  const section = headerEl.parentElement;
  section.classList.toggle('expanded');
  const expanded = section.classList.contains('expanded');
  headerEl.setAttribute('aria-expanded', expanded ? 'true' : 'false');
  const arrow = headerEl.querySelector('.collapsible-arrow');
  if (arrow) {
    arrow.textContent = expanded ? '▾' : '▸';
  }
}

// collapsibleKeydown makes role="button" headers operable via keyboard.
function collapsibleKeydown(e, headerEl) {
  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault();
    toggleCollapsible(headerEl);
  }
}

// ── Medical Enhancements Display ──
function renderMedicalEnhancements(debateId, medicalData) {
  if (!medicalData) return;
  const feed = document.getElementById('feed-content');
  if (!feed) return;

  // Differential Diagnosis Ranking
  if (medicalData.differential_diagnosis && medicalData.differential_diagnosis.length > 0) {
    const card = document.createElement('div');
    card.className = 'medical-dx-card';
    let html = '<div class="medical-dx-header"><h3>🔬 Differential Diagnosis Ranking</h3></div>';
    html += '<div class="medical-dx-list">';
    for (const dx of medicalData.differential_diagnosis) {
      const pct = Math.round((dx.probability || 0) * 100);
      let barColor = 'var(--accent)';
      if (pct > 70) barColor = 'var(--red)';
      else if (pct > 40) barColor = 'var(--orange)';
      else barColor = 'var(--green)';
      html += `<div class="dx-item">
        <div class="dx-label">
          <span class="dx-rank">#${dx.rank || '—'}</span>
          <span class="dx-name">${esc(dx.diagnosis || dx.name || '')}</span>
          <span class="dx-pct">${pct}%</span>
        </div>
        <div class="probability-bar">
          <div class="probability-bar-fill" style="width:${pct}%;background:${barColor}"></div>
        </div>
        ${dx.reasoning ? `<div class="dx-reasoning">${esc(dx.reasoning)}</div>` : ''}
      </div>`;
    }
    html += '</div>';
    card.innerHTML = html;
    feed.insertBefore(card, feed.firstChild);
  }

  // Dynamic Risk Score
  if (medicalData.risk_score != null) {
    const card = document.createElement('div');
    card.className = 'medical-risk-card';
    const risk = Math.round(medicalData.risk_score);
    let riskColor = 'var(--green)';
    let riskLabel = 'Low Risk';
    if (risk > 70) { riskColor = 'var(--red)'; riskLabel = 'High Risk'; }
    else if (risk > 40) { riskColor = 'var(--orange)'; riskLabel = 'Moderate Risk'; }
    let html = `<div class="medical-risk-header"><h3>⚡ Dynamic Risk Score</h3></div>`;
    html += `<div class="risk-gauge">
      <div class="risk-gauge-track">
        <div class="risk-gauge-fill" style="width:${risk}%;background:${riskColor}"></div>
      </div>
      <div class="risk-gauge-labels">
        <span style="color:var(--green)">Low</span>
        <span style="color:${riskColor};font-weight:700">${risk} — ${riskLabel}</span>
        <span style="color:var(--red)">High</span>
      </div>
    </div>`;
    if (medicalData.risk_breakdown && medicalData.risk_breakdown.length > 0) {
      html += '<div class="risk-breakdown">';
      for (const rb of medicalData.risk_breakdown) {
        html += `<div class="risk-factor">
          <span class="risk-factor-name">${esc(rb.factor || rb.name || '')}</span>
          <span class="risk-factor-value">${esc(rb.value || '')}</span>
        </div>`;
      }
      html += '</div>';
    }
    card.innerHTML = html;
    feed.insertBefore(card, feed.firstChild);
  }
}
