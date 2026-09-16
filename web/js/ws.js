// ws.js — WebSocket client
let ws = null;
let wsDebateId = null;

function connectWS(debateId) {
  wsDebateId = debateId;
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${protocol}//${location.host}/ws/debate/${debateId}`;

  ws = new WebSocket(url);

  ws.onopen = () => {
    console.log('WS connected:', debateId);
  };

  ws.onmessage = (evt) => {
    try {
      const msg = JSON.parse(evt.data);
      handleWSEvent(msg);
    } catch (e) {
      console.error('WS parse error:', e);
    }
  };

  ws.onclose = () => {
    console.log('WS disconnected');
  };

  ws.onerror = (err) => {
    console.error('WS error:', err);
  };
}

function handleWSEvent(msg) {
  switch (msg.event) {
    case 'phase_change':
      onPhaseChange(msg.data);
      break;
    case 'panel_ready':
      onPanelReady(msg.data.agents);
      break;
    case 'agent_searching':
      onAgentSearching(msg.data);
      break;
    case 'research_round':
      onResearchRound(msg.data);
      break;
    case 'agent_thinking':
      onAgentThinking(msg.data);
      break;
    case 'agent_speaking':
      onAgentSpeaking(msg.data);
      break;
    case 'agent_done':
      onAgentDone(msg.data);
      break;
    case 'graph_update':
      onGraphUpdate(msg.data);
      break;
    case 'verdict':
      onVerdict(msg.data);
      break;
    case 'synthesis':
      onSynthesis(msg.data);
      break;
    case 'debate_paused':
      onDebatePaused(msg.data);
      break;
    case 'debate_resumed':
      onDebateResumed(msg.data);
      break;
    case 'user_injection':
      onUserInjection(msg.data);
      break;
    case 'claim_challenged':
      onClaimChallenged(msg.data);
      break;
    case 'post_debate_analysis':
      onPostDebateAnalysis(msg.data);
      break;
    case 'social_simulation':
      onSocialSimulation(msg.data);
      break;
    case 'error':
      onError(msg.data);
      break;
  }
}

function disconnectWS() {
  if (ws) {
    ws.close();
    ws = null;
  }
}
