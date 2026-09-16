// graph.js — Knowledge Base: Graph view with MiroFish-inspired design
let graphNodes = [];
let graphEdges = [];
let simulation = null;
let svgEl = null;
let graphGroup = null;
let gLinks = null;
let gNodes = null;
let gLabels = null;
let zoomBehavior = null;
let graphInitialized = false;
let activeFilter = 'all';
let searchQuery = '';
let currentKBView = 'graph';
let selectedNode = null;

// ── Entity Type Colors (MiroFish-inspired) ──
const ENTITY_COLORS = {
  claim: '#6366f1',      // Indigo
  concept: '#a78bfa',    // Purple
  evidence: '#34d399',   // Green
  agent: '#fb923c'       // Orange
};

// ─ View Toggle ──
function switchKBView(view) {
  currentKBView = view;
  document.querySelectorAll('.kb-toggle').forEach(b => {
    b.classList.toggle('active', b.dataset.view === view);
  });
  document.getElementById('graph-view-container').classList.toggle('active', view === 'graph');
  document.getElementById('list-view-container').classList.toggle('active', view === 'list');

  if (view === 'graph') {
    initGraph();
    restartSimulation();
  } else {
    renderNotesList();
  }
}

// ── Graph Zoom Controls ──
function graphZoomIn() {
  if (!svgEl || !zoomBehavior) return;
  svgEl.transition().duration(300).call(zoomBehavior.scaleBy, 1.4);
}

function graphZoomOut() {
  if (!svgEl || !zoomBehavior) return;
  svgEl.transition().duration(300).call(zoomBehavior.scaleBy, 0.7);
}

function graphZoomReset() {
  if (!svgEl || !zoomBehavior) return;
  svgEl.transition().duration(300).call(zoomBehavior.transform, d3.zoomIdentity);
}

// ── Graph Container Size ──
function graphContainerSize() {
  const container = document.getElementById('graph-container');
  if (!container) return { w: 600, h: 500 };
  const w = container.clientWidth || 600;
  const h = container.clientHeight || 500;
  return { w: Math.max(w, 200), h: Math.max(h, 200) };
}

// ── Initialize Graph ──
function initGraph() {
  if (graphInitialized) {
    const { w, h } = graphContainerSize();
    if (svgEl) {
      svgEl.attr('width', w).attr('height', h).attr('viewBox', `0 0 ${w} ${h}`);
      if (simulation) {
        simulation.force('center', d3.forceCenter(w / 2, h / 2));
        simulation.alpha(0.3).restart();
      }
    }
    return;
  }

  const container = document.getElementById('graph-container');
  if (!container) return;

  requestAnimationFrame(() => {
    if (graphInitialized) return;

    const { w, h } = graphContainerSize();

    svgEl = d3.select('#graph-container')
      .append('svg')
      .attr('width', w)
      .attr('height', h)
      .attr('viewBox', `0 0 ${w} ${h}`);

    zoomBehavior = d3.zoom()
      .scaleExtent([0.1, 5])
      .on('zoom', (event) => {
        graphGroup.attr('transform', event.transform);
      });

    svgEl.call(zoomBehavior);

    svgEl.on('dblclick.zoom', () => {
      svgEl.transition().duration(300).call(zoomBehavior.transform, d3.zoomIdentity);
    });

    graphGroup = svgEl.append('g').attr('class', 'graph-layer');

    // Arrow marker
    svgEl.append('defs').append('marker')
      .attr('id', 'arrowhead')
      .attr('viewBox', '0 -5 10 10')
      .attr('refX', 25)
      .attr('refY', 0)
      .attr('markerWidth', 6)
      .attr('markerHeight', 6)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M0,-5L10,0L0,5')
      .attr('fill', '#6b7280');

    gLinks = graphGroup.append('g').attr('class', 'links');
    gNodes = graphGroup.append('g').attr('class', 'nodes');
    gLabels = graphGroup.append('g').attr('class', 'labels');

    graphInitialized = true;
    setupGraphControls();
    renderEntityLegend();

    if (graphNodes.length > 0) {
      restartSimulation();
    }
  });
}

// ── Entity Type Legend (MiroFish-style) ──
function renderEntityLegend() {
  const legendContainer = document.getElementById('graph-legend');
  if (!legendContainer) return;

  const entityTypes = [
    { type: 'claim', label: 'Claim', color: ENTITY_COLORS.claim },
    { type: 'concept', label: 'Concept', color: ENTITY_COLORS.concept },
    { type: 'evidence', label: 'Evidence', color: ENTITY_COLORS.evidence }
  ];

  legendContainer.innerHTML = entityTypes.map(t => `
    <span class="legend-item">
      <span class="dot" style="background:${t.color}"></span>
      ${t.label}
    </span>
  `).join('') + `
    <span class="legend-item">
      <span class="dot" style="background:#4ade80"></span> supports
    </span>
    <span class="legend-item">
      <span class="dot" style="background:#f87171"></span> contradicts
    </span>
    <span class="legend-item">
      <span class="dot" style="background:#60a5fa"></span> cites
    </span>
  `;
}

// ─ Setup Filter & Search Controls ──
function setupGraphControls() {
  const searchInput = document.getElementById('graph-search');
  if (searchInput) {
    searchInput.addEventListener('input', (e) => {
      searchQuery = e.target.value.toLowerCase();
      if (currentKBView === 'graph') {
        restartSimulation();
      } else {
        renderNotesList();
      }
    });
  }

  document.querySelectorAll('.kb-filter').forEach(btn => {
    btn.addEventListener('click', function() {
      document.querySelectorAll('.kb-filter').forEach(b => b.classList.remove('active'));
      this.classList.add('active');
      activeFilter = this.dataset.filter;
      if (currentKBView === 'graph') {
        restartSimulation();
      } else {
        renderNotesList();
      }
    });
  });
}

// ── Filter Nodes ──
function getFilteredNodes() {
  let filtered = [...graphNodes];
  if (activeFilter !== 'all') {
    filtered = filtered.filter(n => n.type === activeFilter);
  }
  if (searchQuery) {
    filtered = filtered.filter(n =>
      n.label.toLowerCase().includes(searchQuery) ||
      (n.content || '').toLowerCase().includes(searchQuery) ||
      (n.agent_name || '').toLowerCase().includes(searchQuery)
    );
  }
  return filtered;
}

// ── Filter Edges ──
function getFilteredEdges(filteredNodes) {
  const nodeIds = new Set(filteredNodes.map(n => n.id));
  return graphEdges.filter(e => nodeIds.has(e.from) && nodeIds.has(e.to));
}

// ── Add Graph Data (incremental) ──
function addGraphData(newNodes, newEdges) {
  const existingIds = new Set(graphNodes.map(n => n.id));
  for (const n of newNodes) {
    if (existingIds.has(n.id)) {
      const existing = graphNodes.find(x => x.id === n.id);
      if (existing) {
        existing.refs = (existing.refs || 1) + 1;
        if (n.content && !existing.content) existing.content = n.content;
        if (n.source_url && !existing.source_url) existing.source_url = n.source_url;
        if (n.agent_name && !existing.agent_name) existing.agent_name = n.agent_name;
      }
    } else {
      graphNodes.push({...n, refs: n.refs || 1});
      existingIds.add(n.id);
    }
  }

  for (const e of newEdges) {
    if (!graphEdges.some(x => x.from === e.from && x.to === e.to && x.relation === e.relation)) {
      graphEdges.push(e);
    }
  }

  if (currentKBView === 'graph') {
    restartSimulation();
  } else {
    renderNotesList();
  }
}

// ── Restart Simulation with Curved Edges ──
function restartSimulation() {
  if (!graphInitialized) return;
  if (simulation) simulation.stop();

  const container = document.getElementById('graph-container');
  const width = container.clientWidth;
  const height = container.clientHeight;

  const filteredNodes = getFilteredNodes();
  const filteredEdges = getFilteredEdges(filteredNodes);

  // Convert edges for D3
  const links = filteredEdges.map(e => ({
    source: e.from,
    target: e.to,
    relation: e.relation
  }));
  const nodes = filteredNodes.map(n => ({...n}));

  // Better force parameters for cleaner layout
  simulation = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id(d => d.id).distance(120))
    .force('charge', d3.forceManyBody().strength(-300))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('collision', d3.forceCollide().radius(30))
    .force('x', d3.forceX(width / 2).strength(0.05))
    .force('y', d3.forceY(height / 2).strength(0.05));

  // Curved Links
  gLinks.selectAll('path').remove();
  const link = gLinks.selectAll('path')
    .data(links)
    .join('path')
    .attr('fill', 'none')
    .attr('stroke', d => {
      switch (d.relation) {
        case 'supports': return '#4ade80';
        case 'contradicts': return '#f87171';
        case 'cites': return '#60a5fa';
        default: return '#6b7280';
      }
    })
    .attr('stroke-width', 1.5)
    .attr('stroke-dasharray', d => d.relation === 'cites' ? '4,3' : null)
    .attr('marker-end', 'url(#arrowhead)')
    .attr('opacity', 0.6);

  // Nodes with better sizing
  gNodes.selectAll('circle').remove();
  const node = gNodes.selectAll('circle')
    .data(nodes)
    .join('circle')
    .attr('r', d => {
      const baseSize = 10;
      const refBonus = Math.min((d.refs || 1) * 2, 12);
      return baseSize + refBonus;
    })
    .attr('fill', d => ENTITY_COLORS[d.type] || ENTITY_COLORS.claim)
    .attr('stroke', d => {
      if (searchQuery && (d.label.toLowerCase().includes(searchQuery) || (d.content || '').toLowerCase().includes(searchQuery))) {
        return '#fff';
      }
      return '#1a1d24';
    })
    .attr('stroke-width', d => {
      if (searchQuery && (d.label.toLowerCase().includes(searchQuery) || (d.content || '').toLowerCase().includes(searchQuery))) {
        return 3;
      }
      return 2;
    })
    .attr('cursor', 'pointer')
    .on('click', (event, d) => {
      event.stopPropagation();
      showFloatingNodeDetails(d, event);
    })
    .call(d3.drag()
      .on('start', (event, d) => {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
      })
      .on('drag', (event, d) => {
        d.fx = event.x;
        d.fy = event.y;
      })
      .on('end', (event, d) => {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
      })
    );

  // Labels with better positioning
  gLabels.selectAll('text').remove();
  gLabels.selectAll('text')
    .data(nodes)
    .join('text')
    .text(d => {
      const maxLen = 30;
      if (d.label.length > maxLen) {
        return d.label.substring(0, maxLen - 3) + '...';
      }
      return d.label;
    })
    .attr('font-size', 11)
    .attr('fill', d => {
      if (searchQuery && (d.label.toLowerCase().includes(searchQuery) || (d.content || '').toLowerCase().includes(searchQuery))) {
        return '#fff';
      }
      return '#e8eaed';
    })
    .attr('font-weight', d => {
      if (searchQuery && (d.label.toLowerCase().includes(searchQuery) || (d.content || '').toLowerCase().includes(searchQuery))) {
        return '700';
      }
      return '500';
    })
    .attr('text-anchor', 'middle')
    .attr('dy', d => {
      const radius = 10 + Math.min((d.refs || 1) * 2, 12);
      return -(radius + 8);
    })
    .attr('pointer-events', 'none');

  // Tooltips
  node.append('title')
    .text(d => `${d.label}\nType: ${d.type}\nReferences: ${d.refs}${d.agent_name ? '\nBy: ' + d.agent_name : ''}`);

  // Tick function with curved edges
  simulation.on('tick', () => {
    link.attr('d', d => {
      const dx = d.target.x - d.source.x;
      const dy = d.target.y - d.source.y;
      const dr = Math.sqrt(dx * dx + dy * dy) * 1.5; // Curve factor
      return `M${d.source.x},${d.source.y}A${dr},${dr} 0 0,1 ${d.target.x},${d.target.y}`;
    });

    node
      .attr('cx', d => d.x)
      .attr('cy', d => d.y);

    gLabels.selectAll('text')
      .attr('x', d => d.x)
      .attr('y', d => d.y);
  });
}

// ── Floating Node Detail Panel (MiroFish-style) ──
function showFloatingNodeDetails(node, event) {
  selectedNode = node;

  // Create or update floating panel
  let panel = document.getElementById('floating-node-panel');
  if (!panel) {
    panel = document.createElement('div');
    panel.id = 'floating-node-panel';
    panel.className = 'floating-detail-panel';
    document.getElementById('graph-view-container').appendChild(panel);
  }

  // Build connections list
  const connections = [];
  graphEdges.forEach(e => {
    if (e.from === node.id) {
      const target = graphNodes.find(n => n.id === e.to);
      if (target) connections.push({ node: target, relation: e.relation, direction: '→' });
    } else if (e.to === node.id) {
      const source = graphNodes.find(n => n.id === e.from);
      if (source) connections.push({ node: source, relation: e.relation, direction: '←' });
    }
  });

  panel.innerHTML = `
    <div class="floating-detail-header">
      <span class="floating-detail-type" style="background:${ENTITY_COLORS[node.type] || ENTITY_COLORS.claim}20; color:${ENTITY_COLORS[node.type] || ENTITY_COLORS.claim}">${node.type}</span>
      <button class="floating-detail-close" onclick="closeFloatingNodeDetails()">×</button>
    </div>
    <h4 class="floating-detail-title">${esc(node.label)}</h4>
    <div class="floating-detail-meta">
      ${node.agent_name ? `<span>👤 ${esc(node.agent_name)}</span>` : ''}
      ${node.round ? `<span>🔄 Round ${node.round}</span>` : ''}
      ${node.refs > 1 ? `<span>📌 ${node.refs} refs</span>` : ''}
    </div>
    ${node.content ? `<div class="floating-detail-body">${esc(node.content)}</div>` : ''}
    ${node.source_url ? `<div class="floating-detail-source"><a href="${esc(safeUrl(node.source_url))}" target="_blank">↗ View Source</a></div>` : ''}
    <div class="floating-detail-connections">
      <h5>Connections (${connections.length})</h5>
      ${connections.length > 0 ? connections.map(c => `
        <div class="connection-item" onclick="navigateToNode('${esc(c.node.id)}')">
          <span class="connection-relation ${c.relation}">${c.relation}</span>
          <span class="connection-direction">${c.direction}</span>
          <span class="connection-label">${esc(c.node.label)}</span>
        </div>
      `).join('') : '<span class="no-connections">No connections</span>'}
    </div>
  `;

  // Position panel near the clicked node
  const container = document.getElementById('graph-view-container');
  const containerRect = container.getBoundingClientRect();
  const x = event.clientX - containerRect.left;
  const y = event.clientY - containerRect.top;

  panel.style.left = Math.min(x + 20, containerRect.width - 320) + 'px';
  panel.style.top = Math.min(y - 50, containerRect.height - 400) + 'px';
  panel.classList.remove('hidden');
}

function closeFloatingNodeDetails() {
  const panel = document.getElementById('floating-node-panel');
  if (panel) {
    panel.classList.add('hidden');
  }
  selectedNode = null;
}

function navigateToNode(nodeId) {
  const node = graphNodes.find(n => n.id === nodeId);
  if (node) {
    showFloatingNodeDetails(node, { clientX: 0, clientY: 0 });
    if (currentKBView === 'graph') focusNode(nodeId);
  }
}

function focusNode(nodeId) {
  searchQuery = '';
  activeFilter = 'all';
  const searchInput = document.getElementById('graph-search');
  if (searchInput) searchInput.value = '';
  document.querySelectorAll('.kb-filter').forEach(b => b.classList.remove('active'));
  const allBtn = document.querySelector('.kb-filter[data-filter="all"]');
  if (allBtn) allBtn.classList.add('active');
  restartSimulation();
}

// ── Full Graph Replacement ──
function updateGraph(nodes, edges) {
  graphNodes = nodes || [];
  graphEdges = edges || [];
  initGraph();
  restartSimulation();
  if (currentKBView === 'list') {
    renderNotesList();
  }
}

// ─ Notes List View ──
function renderNotesList() {
  const listEl = document.getElementById('notes-list');
  const emptyEl = document.getElementById('notes-empty');
  if (!listEl || !emptyEl) return;

  const filtered = getFilteredNodes();

  if (filtered.length === 0) {
    listEl.innerHTML = '';
    emptyEl.style.display = 'flex';
    return;
  }

  emptyEl.style.display = 'none';

  const typeOrder = { claim: 0, concept: 1, evidence: 2 };
  filtered.sort((a, b) => {
    const ta = typeOrder[a.type] ?? 3;
    const tb = typeOrder[b.type] ?? 3;
    if (ta !== tb) return ta - tb;
    return (b.refs || 1) - (a.refs || 1);
  });

  listEl.innerHTML = filtered.map(n => {
    const preview = n.content || n.label;
    const agentInfo = n.agent_name ? `${n.agent_name}` : '';
    const roundInfo = n.round ? `Round ${n.round}` : '';
    const connCount = countConnections(n.id);

    return `<div class="note-card" onclick="showNodeDetailsFromList('${esc(n.id)}')">
      <div class="note-card-header">
        <span class="note-card-type" style="background:${ENTITY_COLORS[n.type] || ENTITY_COLORS.claim}20; color:${ENTITY_COLORS[n.type] || ENTITY_COLORS.claim}">${n.type}</span>
        ${connCount > 0 ? `<span class="note-card-links">${connCount} link${connCount !== 1 ? 's' : ''}</span>` : ''}
      </div>
      <div class="note-card-title">${esc(n.label)}</div>
      <div class="note-card-preview">${esc(preview)}</div>
      <div class="note-card-meta">
        ${agentInfo ? `<span>👤 ${esc(agentInfo)}</span>` : ''}
        ${roundInfo ? `<span>🔄 ${esc(roundInfo)}</span>` : ''}
        ${n.refs > 1 ? `<span>📌 ${n.refs} refs</span>` : ''}
      </div>
    </div>`;
  }).join('');
}

function showNodeDetailsFromList(nodeId) {
  const node = graphNodes.find(n => n.id === nodeId);
  if (node) {
    showFloatingNodeDetails(node, { clientX: 400, clientY: 300 });
  }
}

function countConnections(nodeId) {
  return graphEdges.filter(e => e.from === nodeId || e.to === nodeId).length;
}

function esc(s) {
  if (!s) return '';
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

// safeUrl restricts href targets to http(s) — blocks javascript:/data: XSS.
function safeUrl(url) {
  const u = String(url || '').trim();
  return /^https?:\/\//i.test(u) ? u : '#';
}

// Resize handler
window.addEventListener('resize', () => {
  if (graphInitialized && simulation && currentKBView === 'graph') {
    const container = document.getElementById('graph-container');
    if (container) {
      const { w, h } = graphContainerSize();
      svgEl.attr('width', w).attr('height', h).attr('viewBox', `0 0 ${w} ${h}`);
      simulation.force('center', d3.forceCenter(w / 2, h / 2));
      simulation.alpha(0.3).restart();
    }
  }
});
