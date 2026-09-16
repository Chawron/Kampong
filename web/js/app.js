// app.js — Main application controller

// Navigation
function switchView(viewId) {
  document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
  document.getElementById(viewId).classList.add('active');
}

function goHome() {
  disconnectWS();
  hideMedicalDisclaimer();
  switchView('landing');
  // Reset graph
  graphNodes = [];
  graphEdges = [];
  if (simulation) simulation.stop();
  graphInitialized = false;
  const container = document.getElementById('graph-container');
  if (container) container.innerHTML = '';
  // Reset feed
  const feed = document.getElementById('feed-content');
  if (feed) feed.innerHTML = '';
  streamingBubble = null;
  streamingAgentId = null;
  streamingText = '';
  // Reset progress
  updateProgress({ phase: '', round: 0, total_rounds: 5 });
  // Reset pause state
  debatePaused = false;
  const pauseBanner = document.getElementById('pause-banner');
  if (pauseBanner) pauseBanner.classList.add('hidden');
  const pauseIcon = document.getElementById('pause-resume-icon');
  if (pauseIcon) pauseIcon.textContent = '⏸';
  // Reset verdict analysis panels
  const calibDiv = document.getElementById('analysis-calibration');
  if (calibDiv) calibDiv.innerHTML = '';
  const consisDiv = document.getElementById('analysis-consistency');
  if (consisDiv) consisDiv.innerHTML = '';
  const critDiv = document.getElementById('analysis-critique');
  if (critDiv) critDiv.innerHTML = '';
  const socialResults = document.getElementById('social-results');
  if (socialResults) socialResults.innerHTML = '';
  const socialBtn = document.getElementById('btn-social-sim');
  if (socialBtn) { socialBtn.style.display = ''; socialBtn.disabled = false; socialBtn.textContent = '🌐 Simulate Public Reaction'; }
}

// Mode selection
function selectMode(btn) {
  document.querySelectorAll('.mode-chip').forEach(c => c.classList.remove('active'));
  btn.classList.add('active');
}

// Start debate
async function startDebate() {
  const topic = document.getElementById('topic').value.trim();
  if (!topic) {
    alert('Please enter a debate topic.');
    return;
  }

  const activeChip = document.querySelector('.mode-chip.active');
  const mode = activeChip ? activeChip.dataset.mode : 'deep';

  const btn = document.getElementById('start-btn');
  btn.disabled = true;
  btn.classList.add('loading');
  btn.innerHTML = '<span class="search-spinner"></span> Starting...';

  try {
    // Save settings first
    await syncSettings();

    // Upload attached files first
    let attachments = [];
    if (attachedFiles.length > 0) {
      attachments = await uploadAttachedFiles();
    }

    const resp = await fetch('/api/debate/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ 
        topic, 
        mode,
        attachments: attachments.length > 0 ? attachments : undefined
      }),
    });

    if (!resp.ok) {
      const err = await resp.json();
      throw new Error(err.error || 'Failed to start debate');
    }

    const data = await resp.json();

    // Switch to arena view
    switchView('arena');
    document.getElementById('arena-topic').textContent = topic;
    document.getElementById('mode-badge').textContent = mode.toUpperCase();

    // Initialize graph
    initGraph();

    // Connect WebSocket
    connectWS(data.debate_id);

    // Render medical enhancements if present
    if (data.medical_data) {
      renderMedicalEnhancements(data.debate_id, data.medical_data);
    }

    // Setup scroll tracking for auto-scroll feature
    setupScrollTracking();

  } catch (err) {
    alert('Error: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.classList.remove('loading');
    btn.innerHTML = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg> Start Debate';
  }
}

// Inline config section
function toggleConfigSection() {
  const section = document.getElementById('config-section');
  if (section.style.display === 'none') {
    section.style.display = 'block';
  section.classList.remove('collapsed');
  loadConfigFields();
  // Scroll config into view
    section.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  } else {
    section.style.display = 'none';
  }
}

function loadConfigFields() {
  // Load from localStorage first (instant)
  document.getElementById('cfg-api-key').value = localStorage.getItem('kampong_api_key') || '';
  document.getElementById('cfg-base-url').value = localStorage.getItem('kampong_base_url') || '';
  document.getElementById('cfg-model').value = localStorage.getItem('kampong_model') || '';
  document.getElementById('cfg-tavily-key').value = localStorage.getItem('kampong_tavily_key') || '';

  // Also fetch from server to restore after browser cache clear
  fetch('/api/config')
    .then(r => r.json())
    .then(cfg => {
      if (cfg.llm) {
        if (!localStorage.getItem('kampong_api_key') && cfg.llm.api_key) {
          document.getElementById('cfg-api-key').value = cfg.llm.api_key;
          localStorage.setItem('kampong_api_key', cfg.llm.api_key);
        }
        if (!localStorage.getItem('kampong_base_url') && cfg.llm.base_url) {
          document.getElementById('cfg-base-url').value = cfg.llm.base_url;
          localStorage.setItem('kampong_base_url', cfg.llm.base_url);
        }
        if (!localStorage.getItem('kampong_model') && cfg.llm.model) {
          document.getElementById('cfg-model').value = cfg.llm.model;
          localStorage.setItem('kampong_model', cfg.llm.model);
        }
      }
      if (cfg.search && cfg.search.tavily_api_key && !localStorage.getItem('kampong_tavily_key')) {
        document.getElementById('cfg-tavily-key').value = cfg.search.tavily_api_key;
        localStorage.setItem('kampong_tavily_key', cfg.search.tavily_api_key);
      }
    })
    .catch(() => {}); // silently ignore — localStorage is the primary source
}

async function saveSettings() {
  const apiKey = document.getElementById('cfg-api-key').value.trim();
  const baseUrl = document.getElementById('cfg-base-url').value.trim();
  const model = document.getElementById('cfg-model').value.trim();
  const tavilyKey = document.getElementById('cfg-tavily-key').value.trim();

  localStorage.setItem('kampong_api_key', apiKey);
  localStorage.setItem('kampong_base_url', baseUrl);
  localStorage.setItem('kampong_model', model);
  localStorage.setItem('kampong_tavily_key', tavilyKey);

  // Sync to server
  await syncSettings();

  const status = document.getElementById('config-status');
  status.textContent = 'Saved!';
  setTimeout(() => { status.textContent = ''; }, 2000);
}

async function syncSettings() {
  const body = {};
  const apiKey = localStorage.getItem('kampong_api_key');
  const baseUrl = localStorage.getItem('kampong_base_url');
  const model = localStorage.getItem('kampong_model');
  const tavilyKey = localStorage.getItem('kampong_tavily_key');

  if (apiKey) body.api_key = apiKey;
  if (baseUrl) body.base_url = baseUrl;
  if (model) body.model = model;
  if (tavilyKey) body.tavily_key = tavilyKey;

  if (Object.keys(body).length > 0) {
    await fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  }
}

// Mode selector UI & keyboard shortcuts
document.addEventListener('DOMContentLoaded', () => {

  // Check for medical debate redirect (/?debate_id=...&mode=medical)
  const params = new URLSearchParams(window.location.search);
  const debateId = params.get('debate_id');
  const mode = params.get('mode');
  if (debateId) {
    viewPastDebate(debateId);
    if (mode === 'medical') {
      const badge = document.getElementById('mode-badge');
      if (badge) badge.textContent = 'MEDICAL';
      // Display medical knowledge panel + disclaimer banner
      showMedicalDisclaimer();
      renderMedicalKnowledgePanel(debateId);
    }
    // Clean URL
    window.history.replaceState({}, '', '/');
  }

  // Enter key submits
  const topicInput = document.getElementById('topic');
  if (topicInput) {
    topicInput.addEventListener('keydown', function(e) {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        startDebate();
      }
    });
  }

  // Load saved config into fields
  loadConfigFields();

  // Sync config to server on load (silently)
  syncSettings().catch(() => {});

  // Load past debates on startup
  loadPastDebates();

  // Keyboard shortcuts
  document.addEventListener('keydown', (e) => {
    // Escape to go back
    if (e.key === 'Escape') {
      const arena = document.getElementById('arena');
      if (arena && arena.classList.contains('active')) {
        goHome();
      }
      // Close verdict overlay
      const verdict = document.getElementById('verdict-overlay');
      if (verdict && !verdict.classList.contains('hidden')) {
        closeVerdict();
      }
    }
  });
});

// Past debates
async function loadPastDebates() {
  try {
    const resp = await fetch('/api/debates');
    if (!resp.ok) return;
    const data = await resp.json();
    if (!data.debates || data.debates.length === 0) {
      document.getElementById('past-debates-section').style.display = 'none';
      updateStats(0, 0, 0);
      return;
    }

    const section = document.getElementById('past-debates-section');
    section.style.display = 'block';

    // Update stats
    const totalDebates = data.debates.length;
    const totalAgents = data.debates.reduce((sum, d) => sum + (d.agents || 0), 0);
    const uniqueTopics = new Set(data.debates.map(d => d.topic)).size;
    updateStats(totalDebates, totalAgents, uniqueTopics);

    const list = document.getElementById('past-debates-list');
    list.innerHTML = data.debates.map(d => `
      <div class="debate-item">
        <span class="debate-topic" onclick="viewPastDebate('${esc(d.id)}')">${esc(d.topic)}</span>
        <span class="debate-meta">
          <span class="badge badge-sm">${esc(d.mode)}</span>
          ${d.has_verdict ? '<span class="badge badge-success badge-sm">verdict</span>' : ''}
          ${d.status === 'error' ? '<span class="badge badge-error badge-sm">error</span>' : ''}
        </span>
        <div class="debate-actions">
          <button onclick="event.stopPropagation();rerunDebate('${esc(d.id)}')" title="Re-run">🔄</button>
          <button class="delete-btn" onclick="event.stopPropagation();deleteDebate('${esc(d.id)}')" title="Delete">🗑</button>
        </div>
      </div>
    `).join('');
  } catch (e) {
    // Silently ignore - past debates are optional
  }
}

function updateStats(debates, agents, topics) {
  const statDebates = document.getElementById('stat-debates');
  const statAgents = document.getElementById('stat-agents');
  const statTopics = document.getElementById('stat-topics');
  
  if (statDebates) statDebates.textContent = debates;
  if (statAgents) statAgents.textContent = agents;
  if (statTopics) statTopics.textContent = topics;
}

function viewPastDebate(id) {
  // Load persisted debate and show it in the arena (read-only)
  switchView('arena');

  // Fetch status to get cached session
  fetch(`/api/debate/${id}/status`)
    .then(r => r.json())
    .then(data => {
      document.getElementById('arena-topic').textContent = data.topic || '';
      document.getElementById('mode-badge').textContent = (data.mode || '').toUpperCase();
      if ((data.topic || '').includes('[Medical Case Data:')) showMedicalDisclaimer();

      // Update progress to show completed
      updateProgress({ phase: 'completed', round: data.rounds || 0, total_rounds: data.rounds || 5 });

      // Render panel
      if (data.agents) {
        onPanelReady(data.agents);
      }

      // Fetch transcript
      return fetch(`/api/debate/${id}/history`);
    })
    .then(r => r.json())
    .then(data => {
      const feed = document.getElementById('feed-content');
      feed.innerHTML = '';

      // Check for medical case data embedded in topic and render imaging card
      renderMedicalImagingCard(feed);

      if (data.transcript) {
        data.transcript.forEach(entry => {
          const div = document.createElement('div');
          div.className = 'agent-turn-card';
          div.innerHTML = `
            <div class="agent-card-header">
              <span class="agent-card-icon">🤖</span>
              <span class="agent-card-name">${esc(entry.agent_name || entry.agent_id)}</span>
              <span class="agent-card-role">${esc(entry.role || '')} · Round ${entry.round}</span>
            </div>
            <div class="speech-bubble">
              <div class="speech-content">${esc(entry.text)}</div>
            </div>
          `;
          feed.appendChild(div);
        });
      }

      // Fetch graph
      return fetch(`/api/debate/${id}/graph`);
    })
    .then(r => r.json())
    .then(data => {
      initGraph();
      if (data.nodes && data.nodes.length > 0) {
        updateGraph(data.nodes, data.edges || []);
      }

      // Check for verdict
      return fetch(`/api/debate/${id}/status`);
    })
    .then(r => r.json())
    .then(data => {
      if (data.verdict) {
        onVerdict(data.verdict);
      }
    })
    .catch(err => {
      console.error('Failed to load past debate:', err);
    });
}

async function deleteDebate(id) {
  if (!confirm('Delete this debate?')) return;
  try {
    await fetch(`/api/debate/${id}`, { method: 'DELETE' });
    loadPastDebates();
  } catch (e) {
    alert('Failed to delete: ' + e.message);
  }
}

async function rerunDebate(id) {
  try {
    const resp = await fetch(`/api/debate/${id}/status`);
    const data = await resp.json();
    // Populate the topic textarea
    document.getElementById('topic').value = data.topic || '';
    // Set the mode chip
    const mode = data.mode || 'deep';
    document.querySelectorAll('.mode-chip').forEach(chip => {
      chip.classList.toggle('active', chip.dataset.mode === mode);
    });
    // Scroll to top so user can see the form
    window.scrollTo({ top: 0, behavior: 'smooth' });
    // Focus the topic input
    document.getElementById('topic').focus();
  } catch (e) {
    alert('Failed to load debate: ' + e.message);
  }
}

// HTML escape helper
function esc(str) {
  const div = document.createElement('div');
  div.textContent = str || '';
  return div.innerHTML;
}

// ── File Attachment Management ──

let attachedFiles = [];

function initFileAttachment() {
  const dropZone = document.getElementById('file-drop-zone');
  const fileInput = document.getElementById('file-input');
  
  if (!dropZone || !fileInput) return;

  // Click to upload
  dropZone.addEventListener('click', () => fileInput.click());
  
  // File input change
  fileInput.addEventListener('change', (e) => {
    handleFiles(e.target.files);
    fileInput.value = ''; // Reset input
  });

  // Drag and drop events
  dropZone.addEventListener('dragover', (e) => {
    e.preventDefault();
    dropZone.classList.add('drag-over');
  });

  dropZone.addEventListener('dragleave', () => {
    dropZone.classList.remove('drag-over');
  });

  dropZone.addEventListener('drop', (e) => {
    e.preventDefault();
    dropZone.classList.remove('drag-over');
    handleFiles(e.dataTransfer.files);
  });
}

function handleFiles(files) {
  const maxSize = 10 * 1024 * 1024; // 10MB
  const allowedTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'application/pdf', 'text/plain', 'text/markdown'];
  
  for (const file of files) {
    // Validate size
    if (file.size > maxSize) {
      alert(`File "${file.name}" is too large. Max size is 10MB.`);
      continue;
    }
    
    // Validate type
    if (!allowedTypes.includes(file.type) && !file.name.match(/\.(txt|md)$/i)) {
      alert(`File "${file.name}" is not supported. Supported: images, PDFs, text files.`);
      continue;
    }
    
    // Check for duplicates
    if (attachedFiles.some(f => f.name === file.name && f.size === file.size)) {
      continue;
    }
    
    attachedFiles.push(file);
  }
  
  renderAttachedFiles();
}

function renderAttachedFiles() {
  const list = document.getElementById('attached-files-list');
  if (!list) return;
  
  if (attachedFiles.length === 0) {
    list.innerHTML = '';
    return;
  }
  
  list.innerHTML = attachedFiles.map((file, index) => {
    const sizeStr = formatFileSize(file.size);
    const isImage = file.type.startsWith('image/');
    
    return `
      <div class="attached-file-item">
        <div class="attached-file-icon">
          ${isImage ? `<img src="${URL.createObjectURL(file)}" alt="">` : '📄'}
        </div>
        <div class="attached-file-info">
          <div class="attached-file-name">${esc(file.name)}</div>
          <div class="attached-file-size">${sizeStr}</div>
        </div>
        <button class="attached-file-remove" onclick="removeAttachedFile(${index})" title="Remove">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </div>
    `;
  }).join('');
}

function removeAttachedFile(index) {
  attachedFiles.splice(index, 1);
  renderAttachedFiles();
}

function formatFileSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

async function uploadAttachedFiles() {
  const uploadedFiles = [];
  
  for (const file of attachedFiles) {
    const formData = new FormData();
    formData.append('file', file);
    
    try {
      const resp = await fetch('/api/upload', {
        method: 'POST',
        body: formData
      });
      
      if (!resp.ok) {
        throw new Error(`Upload failed for ${file.name}`);
      }
      
      const result = await resp.json();
      uploadedFiles.push(result);
    } catch (e) {
      console.error('Failed to upload file:', file.name, e);
      alert(`Failed to upload ${file.name}: ${e.message}`);
    }
  }
  
  // Clear attached files after upload
  attachedFiles = [];
  renderAttachedFiles();
  
  return uploadedFiles;
}

// Initialize file attachment on page load
document.addEventListener('DOMContentLoaded', () => {
  initFileAttachment();
  loadProviders(); // Load LLM providers
});

// ── Provider Management ──

async function loadProviders() {
  try {
    const resp = await fetch('/api/providers');
    const data = await resp.json();
    renderProviders(data.providers || []);
  } catch (e) {
    console.error('Failed to load providers:', e);
  }
}

function renderProviders(providers) {
  const list = document.getElementById('providers-list');
  if (!list) return;

  if (providers.length === 0) {
    list.innerHTML = '<p class="empty-state">No providers configured. Click "+ Add Provider" to add one.</p>';
    return;
  }

  list.innerHTML = providers.map(p => `
    <div class="provider-item">
      <div class="provider-info">
        <div class="provider-name">
          ${esc(p.name)}
          ${p.enabled ? '<span class="badge badge-success">Enabled</span>' : '<span class="badge badge-muted">Disabled</span>'}
        </div>
        <div class="provider-details">
          <span>Model: ${esc(p.model)}</span>
          ${p.vision_model ? `<span>Vision: ${esc(p.vision_model)}</span>` : ''}
          <span>URL: ${esc(p.base_url)}</span>
          ${p.has_api_key ? '<span class="badge badge-info">API Key Set</span>' : '<span class="badge badge-warning">No API Key</span>'}
        </div>
      </div>
      <div class="provider-actions">
        <button class="btn-ghost btn-sm" onclick="deleteProvider('${esc(p.name)}')" title="Delete">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
        </button>
      </div>
    </div>
  `).join('');
}

function showAddProviderForm() {
  const form = document.getElementById('add-provider-form');
  if (form) {
    form.style.display = 'block';
    document.getElementById('provider-name').focus();
  }
}

function hideAddProviderForm() {
  const form = document.getElementById('add-provider-form');
  if (form) {
    form.style.display = 'none';
    // Clear form
    document.getElementById('provider-name').value = '';
    document.getElementById('provider-base-url').value = '';
    document.getElementById('provider-api-key').value = '';
    document.getElementById('provider-model').value = '';
    document.getElementById('provider-vision-model').value = '';
    document.getElementById('provider-enabled').checked = true;
  }
}

async function saveProvider() {
  const name = document.getElementById('provider-name').value.trim();
  const baseUrl = document.getElementById('provider-base-url').value.trim();
  const apiKey = document.getElementById('provider-api-key').value.trim();
  const model = document.getElementById('provider-model').value.trim();
  const visionModel = document.getElementById('provider-vision-model').value.trim();
  const enabled = document.getElementById('provider-enabled').checked;

  if (!name || !baseUrl || !model) {
    alert('Please fill in name, base URL, and model.');
    return;
  }

  try {
    const resp = await fetch('/api/providers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        base_url: baseUrl,
        api_key: apiKey,
        model,
        vision_model: visionModel,
        enabled
      })
    });

    if (!resp.ok) {
      const err = await resp.json();
      throw new Error(err.error || 'Failed to save provider');
    }

    hideAddProviderForm();
    loadProviders();
  } catch (e) {
    alert('Failed to save provider: ' + e.message);
  }
}

async function deleteProvider(name) {
  if (!confirm(`Delete provider "${name}"?`)) return;

  try {
    const resp = await fetch(`/api/providers/${encodeURIComponent(name)}`, {
      method: 'DELETE'
    });

    if (!resp.ok) {
      const err = await resp.json();
      throw new Error(err.error || 'Failed to delete provider');
    }

    loadProviders();
  } catch (e) {
    alert('Failed to delete provider: ' + e.message);
  }
}

// ── Medical Knowledge Panel in Arena ──

// showMedicalDisclaimer reveals the educational-use banner for medical debates.
function showMedicalDisclaimer() {
  const banner = document.getElementById('medical-disclaimer');
  if (banner) banner.classList.remove('hidden');
}

function hideMedicalDisclaimer() {
  const banner = document.getElementById('medical-disclaimer');
  if (banner) banner.classList.add('hidden');
}

function renderMedicalKnowledgePanel(debateId) {
  const knowledge = sessionStorage.getItem('med_knowledge_' + debateId);
  if (!knowledge) return;

  // Find the knowledge base sidebar panel
  const kbPanel = document.querySelector('.kb-panel') || document.getElementById('knowledge-panel');
  if (!kbPanel) {
    // Fallback: inject into the arena right sidebar area
    const rightSidebar = document.querySelector('.arena-sidebar-right') || document.querySelector('.sidebar-right');
    if (rightSidebar) {
      const div = document.createElement('div');
      div.className = 'medical-knowledge-panel';
      div.innerHTML = '<h3>📚 Medical Knowledge</h3><div class="medical-knowledge-content">' + renderMarkdownLite(knowledge) + '</div>';
      rightSidebar.prepend(div);
    }
    return;
  }

  // Replace KB panel content with medical knowledge
  kbPanel.innerHTML = `
    <div class="medical-knowledge-panel">
      <h3>📚 Medical Knowledge Base</h3>
      <div class="medical-knowledge-content">${renderMarkdownLite(knowledge)}</div>
    </div>
  `;
}

// Simple markdown-to-HTML converter for medical knowledge display.
// HTML is escaped first — LLM-generated text must never be treated as markup.
function renderMarkdownLite(md) {
  let html = escapeHtml(String(md || ''));
  return html
    .replace(/^## (.+)$/gm, '<h4>$1</h4>')
    .replace(/^### (.+)$/gm, '<h5>$1</h5>')
    .replace(/^- \*\*(.+?)\*\*(.*)$/gm, '<li><strong>$1</strong>$2</li>')
    .replace(/^- (.+)$/gm, '<li>$1</li>')
    .replace(/^  - (.+)$/gm, '<li class="sub-item">$1</li>')
    .replace(/\n\n/g, '<br>')
    .replace(/(<li[^>]*>.*<\/li>)/gs, function(match) {
      return '<ul>' + match + '</ul>';
    })
    .replace(/<\/ul>\s*<ul>/g, ''); // merge adjacent lists
}

// escapeHtml neutralizes <, >, & and quotes for safe innerHTML injection.
function escapeHtml(text) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

// ── Medical Imaging Display in Arena ──

function renderMedicalImagingCard(feed) {
  const topicEl = document.getElementById('arena-topic');
  if (!topicEl) return;

  const topicText = topicEl.textContent || '';
  const match = topicText.match(/\[Medical Case Data:\s*(\{[\s\S]*\})\]\s*$/);
  if (!match) return;

  let caseData;
  try {
    caseData = JSON.parse(match[1]);
  } catch (e) {
    return;
  }

  const imaging = caseData.imaging_results;
  if (!imaging || !imaging.length) return;

  const card = document.createElement('div');
  card.className = 'medical-imaging-card';

  let html = '<div class="medical-imaging-header"><h3>🩻 Medical Imaging</h3></div>';
  html += '<div class="medical-imaging-grid">';

  imaging.forEach(img => {
    html += '<div class="medical-imaging-item">';
    html += `<div class="imaging-item-header"><strong>${esc(img.modality || 'Unknown')}</strong> — ${esc(img.body_part || 'N/A')}</div>`;

    if (img.image_path) {
      const servePath = img.image_path.replace(/^data[\/\\]uploads[\/\\]/, '/uploads/');
      html += `<div class="imaging-item-preview"><img src="${esc(servePath)}" alt="${esc(img.modality)}" loading="lazy"></div>`;
    }

    if (img.finding) {
      html += `<div class="imaging-item-finding"><strong>Finding:</strong> ${esc(img.finding)}</div>`;
    }

    if (img.date) {
      html += `<div class="imaging-item-date">Date: ${esc(img.date)}</div>`;
    }

    html += '</div>';
  });

  html += '</div>';
  html += '<div class="medical-imaging-disclaimer">* Images analyzed by AI for educational discussion only. Professional radiologist review required. *</div>';

  card.innerHTML = html;
  feed.insertBefore(card, feed.firstChild);
}

// ── Debate Control: Pause / Resume ──

let debatePaused = false;

function togglePauseResume() {
  if (!wsDebateId) return;
  if (debatePaused) {
    resumeDebate();
  } else {
    pauseDebate();
  }
}

async function pauseDebate() {
  if (!wsDebateId) return;
  try {
    const resp = await fetch(`/api/debate/${wsDebateId}/pause`, { method: 'POST' });
    if (!resp.ok) {
      const err = await resp.json();
      alert(err.error || 'Failed to pause debate');
    }
  } catch (e) {
    alert('Failed to pause: ' + e.message);
  }
}

async function resumeDebate() {
  if (!wsDebateId) return;
  try {
    const resp = await fetch(`/api/debate/${wsDebateId}/resume`, { method: 'POST' });
    if (!resp.ok) {
      const err = await resp.json();
      alert(err.error || 'Failed to resume debate');
    }
  } catch (e) {
    alert('Failed to resume: ' + e.message);
  }
}

// ── Inject Evidence Modal ──

function openInjectModal() {
  if (!wsDebateId) return;
  const modal = document.getElementById('inject-modal');
  if (!modal) return;

  // Populate agent dropdown
  const select = document.getElementById('inject-target-agent');
  if (select) {
    select.innerHTML = '<option value="">All agents (general)</option>';
    for (const a of currentAgents) {
      select.innerHTML += `<option value="${esc(a.id)}">${esc(a.icon || '')} ${esc(a.name)}</option>`;
    }
  }

  // Clear textarea
  const textarea = document.getElementById('inject-text');
  if (textarea) textarea.value = '';

  modal.classList.remove('hidden');
}

function closeInjectModal() {
  const modal = document.getElementById('inject-modal');
  if (modal) modal.classList.add('hidden');
}

async function submitInjection() {
  if (!wsDebateId) return;
  const text = document.getElementById('inject-text').value.trim();
  if (!text) { alert('Please enter evidence text.'); return; }

  const targetAgentId = document.getElementById('inject-target-agent').value;

  try {
    const resp = await fetch(`/api/debate/${wsDebateId}/inject`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'evidence',
        text: text,
        target_agent_id: targetAgentId || undefined
      })
    });
    if (!resp.ok) {
      const err = await resp.json();
      throw new Error(err.error || 'Failed to inject evidence');
    }
    closeInjectModal();
  } catch (e) {
    alert('Injection failed: ' + e.message);
  }
}

// ── Challenge Claim Modal ──

let selectedClaimEntryId = null;

function openChallengeModal() {
  if (!wsDebateId) return;
  const modal = document.getElementById('challenge-modal');
  if (!modal) return;

  // Load claims from the knowledge graph notes
  loadClaimsForChallenge();

  // Clear textarea
  const textarea = document.getElementById('challenge-text');
  if (textarea) textarea.value = '';
  selectedClaimEntryId = null;

  const submitBtn = document.getElementById('submit-challenge-btn');
  if (submitBtn) submitBtn.disabled = true;

  modal.classList.remove('hidden');
}

function closeChallengeModal() {
  const modal = document.getElementById('challenge-modal');
  if (modal) modal.classList.add('hidden');
}

async function loadClaimsForChallenge() {
  const listDiv = document.getElementById('challenge-claims-list');
  if (!listDiv) return;

  try {
    const resp = await fetch(`/api/debate/${wsDebateId}/graph`);
    const data = await resp.json();
    const claimNodes = (data.nodes || []).filter(n => n.type === 'claim');

    if (claimNodes.length === 0) {
      listDiv.innerHTML = '<p class="empty-state">No claims found yet. Claims appear as agents speak.</p>';
      return;
    }

    listDiv.innerHTML = claimNodes.map(n => `
      <div class="claim-select-item" onclick="selectClaimForChallenge('${esc(n.id)}', this)">
        <span class="claim-select-text">${esc(n.title || n.label || n.id)}</span>
      </div>
    `).join('');
  } catch (e) {
    listDiv.innerHTML = '<p class="empty-state">Failed to load claims.</p>';
  }
}

function selectClaimForChallenge(entryId, el) {
  selectedClaimEntryId = entryId;
  // Highlight selected
  document.querySelectorAll('.claim-select-item').forEach(item => item.classList.remove('selected'));
  if (el) el.classList.add('selected');
  const submitBtn = document.getElementById('submit-challenge-btn');
  if (submitBtn) submitBtn.disabled = false;
}

async function submitChallenge() {
  if (!wsDebateId || !selectedClaimEntryId) return;
  const challengeText = document.getElementById('challenge-text').value.trim();
  if (!challengeText) { alert('Please enter your challenge.'); return; }

  try {
    const resp = await fetch(`/api/debate/${wsDebateId}/challenge`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        transcript_entry_id: selectedClaimEntryId,
        challenge_text: challengeText
      })
    });
    if (!resp.ok) {
      const err = await resp.json();
      throw new Error(err.error || 'Failed to submit challenge');
    }
    closeChallengeModal();
  } catch (e) {
    alert('Challenge failed: ' + e.message);
  }
}

// ── Social Simulation ──

async function triggerSocialSimulation() {
  if (!wsDebateId) return;
  const btn = document.getElementById('btn-social-sim');
  if (btn) {
    btn.disabled = true;
    btn.textContent = '⏳ Simulating...';
  }

  try {
    const resp = await fetch(`/api/debate/${wsDebateId}/social-simulation`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    if (!resp.ok) {
      const err = await resp.json();
      throw new Error(err.error || 'Social simulation failed');
    }
    const data = await resp.json();
    // Render via the WS handler
    onSocialSimulation(data);
  } catch (e) {
    alert('Social simulation failed: ' + e.message);
    if (btn) {
      btn.disabled = false;
      btn.textContent = '🌐 Simulate Public Reaction';
    }
  }
}
