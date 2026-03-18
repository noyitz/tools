// Main application controller

const App = {
  connected: false,       // gRPC connected to BBR
  clusterConnected: false, // oc logged into cluster
  phase: 'request',
  presets: [],

  async init() {
    HeadersEditor.init('input-headers', 'add-header-btn');
    JsonEditor.init('input-body', 'body-error');

    // Tab switching
    for (const tab of document.querySelectorAll('.sidebar-tab')) {
      tab.addEventListener('click', () => this.switchTab(tab.dataset.tab));
    }

    // Environment tab
    document.getElementById('cluster-login-btn').addEventListener('click', () => this.clusterLogin());
    document.getElementById('cluster-logout-btn').addEventListener('click', () => this.clusterLogout());
    document.getElementById('refresh-cluster').addEventListener('click', () => this.loadClusterInfo());

    // Sandbox tab
    document.getElementById('send-btn').addEventListener('click', () => this.send());
    document.getElementById('prettify-btn').addEventListener('click', () => JsonEditor.prettify());

    for (const btn of document.querySelectorAll('.btn-phase')) {
      btn.addEventListener('click', () => this.setPhase(btn.dataset.phase));
    }

    // Cmd+Enter to send
    document.addEventListener('keydown', (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        e.preventDefault();
        this.send();
      }
    });

    // Load data
    await Promise.all([this.loadPresets(), this.loadClusterInfo()]);
  },

  switchTab(tabId) {
    for (const tab of document.querySelectorAll('.sidebar-tab')) {
      tab.classList.toggle('active', tab.dataset.tab === tabId);
    }
    for (const content of document.querySelectorAll('.tab-content')) {
      content.classList.toggle('active', content.id === 'tab-' + tabId);
    }
    if (tabId === 'environment') {
      this.loadClusterInfo();
    }
  },

  // ===== Environment Tab =====

  async loadClusterInfo() {
    try {
      const resp = await fetch('/api/cluster');
      const info = await resp.json();
      this.clusterInfo = info;
      this.renderClusterState(info);
    } catch (e) {
      document.getElementById('cluster-login-error').textContent = e.message;
    }
  },

  renderClusterState(info) {
    const loginForm = document.getElementById('cluster-login-form');
    const connectedView = document.getElementById('cluster-connected-view');
    const bbrSection = document.getElementById('bbr-section');
    const clusterInfoEl = document.getElementById('cluster-info');

    if (!info.cluster || !info.cluster.connected) {
      // Show login form
      this.clusterConnected = false;
      loginForm.style.display = '';
      connectedView.style.display = 'none';
      bbrSection.style.display = 'none';
      this.updateSandboxStatus(false);
      return;
    }

    // Show connected view
    this.clusterConnected = true;
    loginForm.style.display = 'none';
    connectedView.style.display = '';
    bbrSection.style.display = '';

    clusterInfoEl.innerHTML = `
      <div class="info-row">
        <span class="info-label">Status</span>
        <span class="status-badge running">Connected</span>
      </div>
      <div class="info-row">
        <span class="info-label">Server</span>
        <span class="info-value">${esc(info.cluster.server)}</span>
      </div>
      <div class="info-row">
        <span class="info-label">User</span>
        <span class="info-value">${esc(info.cluster.user)}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Namespace</span>
        <span class="info-value">${esc(info.bbr.namespace)}</span>
      </div>`;

    // BBR card
    const bbrEl = document.getElementById('bbr-info');
    if (info.bbr.status === 'not deployed') {
      bbrEl.innerHTML = `
        <div class="info-row">
          <span class="info-label">Status</span>
          <span class="status-badge error">Not Deployed</span>
        </div>
        <div class="env-note">Deploy BBR with: <code style="color:var(--green)">./deploy/build.sh</code></div>`;
    } else {
      bbrEl.innerHTML = `
        <div class="info-row">
          <span class="info-label">Status</span>
          <span class="status-badge running">${esc(info.bbr.status)}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Pod</span>
          <span class="info-value">${esc(info.bbr.podName)}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Uptime</span>
          <span class="info-value">${esc(info.bbr.uptime)}</span>
        </div>
        <div class="info-row">
          <span class="info-label">Ports</span>
          <span class="info-value">${(info.bbr.ports || []).map(p => p.name + ':' + p.port).join(', ') || 'N/A'}</span>
        </div>`;
    }

    // Plugins card
    const pluginsEl = document.getElementById('plugins-info');
    if (!info.bbr.plugins || info.bbr.plugins.length === 0) {
      pluginsEl.innerHTML = '<div class="placeholder">No plugins configured</div>';
    } else {
      const pluginDescriptions = {
        'body-field-to-header': 'Extracts a body field and sets it as an HTTP header',
        'inference-api-translator': 'Translates between OpenAI and provider-native API formats (e.g. Anthropic)',
        'example-plugin': 'Example/template plugin for development reference',
      };

      pluginsEl.innerHTML = info.bbr.plugins.map((p, i) => {
        const desc = pluginDescriptions[p.type] || '';
        return `
        <div class="plugin-card">
          <div class="plugin-card-header">
            <span class="plugin-icon">&#128268;</span>
            <div>
              <div class="plugin-type">${esc(p.type)}</div>
              <div class="plugin-name">instance: ${esc(p.name)}</div>
            </div>
            <span class="plugin-order">#${i + 1}</span>
          </div>
          ${desc ? `<div class="plugin-desc">${esc(desc)}</div>` : ''}
          ${p.config ? `<div class="plugin-config">${esc(p.config)}</div>` : ''}
        </div>`;
      }).join('');
    }

    // Update diagram and try to auto-connect gRPC to BBR
    this.updateDiagram(info);
    this.tryConnectBBR(info.portForward.target);
  },

  updateDiagram(info) {
    const clusterConnected = info.cluster && info.cluster.connected;
    const bbrRunning = info.bbr && info.bbr.status === 'Running';
    const pluginCount = (info.bbr && info.bbr.plugins) ? info.bbr.plugins.length : 0;

    // Cluster zone
    const clusterZone = document.getElementById('dia-cluster-zone');
    const clusterLabel = document.getElementById('dia-cluster-label');
    if (clusterConnected) {
      clusterZone.classList.add('arch-cluster-active');
      const shortServer = info.cluster.server.replace(/^https?:\/\//, '').replace(/:6443$/, '');
      clusterLabel.textContent = 'Cluster (' + shortServer + ')';
    } else {
      clusterZone.classList.remove('arch-cluster-active');
      clusterLabel.textContent = 'Cluster (not connected)';
    }

    // BBR node
    const bbrNode = document.getElementById('dia-bbr');
    const bbrDetail = document.getElementById('dia-bbr-detail');
    if (clusterConnected && bbrRunning) {
      bbrNode.classList.add('arch-active');
      bbrDetail.textContent = 'Running :9004';
    } else if (clusterConnected) {
      bbrNode.classList.remove('arch-active');
      bbrDetail.textContent = info.bbr.status || 'not deployed';
    } else {
      bbrNode.classList.remove('arch-active');
      bbrDetail.textContent = 'unknown';
    }

    // Plugins node
    const pluginsNode = document.getElementById('dia-plugins');
    const pluginsDetail = document.getElementById('dia-plugins-detail');
    const pluginArrow = document.getElementById('dia-plugin-arrow');
    if (clusterConnected && bbrRunning && pluginCount > 0) {
      pluginsNode.classList.add('arch-active');
      pluginArrow.classList.add('arch-active');
      pluginsDetail.textContent = pluginCount + ' plugin' + (pluginCount > 1 ? 's' : '');
    } else {
      pluginsNode.classList.remove('arch-active');
      pluginArrow.classList.remove('arch-active');
      pluginsDetail.textContent = '--';
    }
  },

  async clusterLogin() {
    const server = document.getElementById('cluster-server').value.trim();
    const username = document.getElementById('cluster-user').value.trim();
    const password = document.getElementById('cluster-pass').value;
    const namespace = document.getElementById('cluster-namespace').value.trim();
    const errorEl = document.getElementById('cluster-login-error');
    const loginBtn = document.getElementById('cluster-login-btn');

    if (!server || !username || !password) {
      errorEl.textContent = 'All fields are required';
      return;
    }

    errorEl.textContent = '';
    loginBtn.textContent = 'Logging in...';
    loginBtn.disabled = true;

    try {
      const resp = await fetch('/api/cluster/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server, username, password, namespace })
      });
      const data = await resp.json();

      if (data.error) {
        errorEl.textContent = data.error;
      } else {
        this.clusterInfo = data;
        this.renderClusterState(data);
      }
    } catch (e) {
      errorEl.textContent = 'Login failed: ' + e.message;
    } finally {
      loginBtn.textContent = 'Login to Cluster';
      loginBtn.disabled = false;
    }
  },

  async clusterLogout() {
    try {
      await fetch('/api/cluster/logout', { method: 'POST' });
    } catch (e) {
      // ignore
    }

    this.connected = false;
    this.clusterConnected = false;

    // Reset UI
    document.getElementById('cluster-login-form').style.display = '';
    document.getElementById('cluster-connected-view').style.display = 'none';
    document.getElementById('bbr-section').style.display = 'none';
    document.getElementById('cluster-server').value = '';
    document.getElementById('cluster-user').value = '';
    document.getElementById('cluster-pass').value = '';
    this.updateSandboxStatus(false);
    this.updateDiagramConnection(false);
    this.updateDiagram({ cluster: { connected: false }, bbr: {}, portForward: {} });
  },

  // ===== BBR gRPC Connection =====

  async tryConnectBBR(target) {
    try {
      const resp = await fetch('/api/connect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target })
      });
      const data = await resp.json();
      this.connected = data.connected;
      this.updateSandboxStatus(data.connected, target);
      this.updateDiagramConnection(data.connected, target);
    } catch (e) {
      this.updateSandboxStatus(false);
      this.updateDiagramConnection(false);
    }
  },

  updateDiagramConnection(connected, target) {
    const pfNode = document.getElementById('dia-portforward');
    const pfArrow = document.getElementById('dia-pf-arrow');
    const pfDetail = document.getElementById('dia-pf-detail');
    const tunnel = document.getElementById('dia-tunnel');
    const hint = document.getElementById('dia-action-hint');

    pfNode.classList.remove('arch-active', 'arch-error');
    pfArrow.classList.remove('arch-active');
    tunnel.classList.remove('arch-active');

    if (connected) {
      pfNode.classList.add('arch-active');
      pfArrow.classList.add('arch-active');
      tunnel.classList.add('arch-active');
      pfDetail.textContent = target || 'localhost:9004';
      hint.style.display = 'none';
    } else if (this.clusterConnected) {
      // Cluster is connected but port-forward is not running
      pfNode.classList.add('arch-error');
      pfDetail.textContent = 'not running';
      hint.style.display = '';
      const ns = (this.clusterInfo && this.clusterInfo.bbr) ? this.clusterInfo.bbr.namespace : 'bbr-plugins';
      const pfCmd = `oc port-forward svc/bbr-plugins 9004:9004 -n ${ns}`;
      hint.innerHTML = `
        <div class="hint-title">&#9888; Action Required — Port Forward Not Running</div>
        <ol class="hint-steps">
          <li><span class="step-num">1</span> Open a new terminal window</li>
          <li><span class="step-num">2</span> Run: <code>${esc(pfCmd)}</code> <button class="copy-btn" onclick="navigator.clipboard.writeText('${pfCmd}').then(()=>this.textContent='Copied!').catch(()=>{})" title="Copy to clipboard">&#128203;</button></li>
          <li><span class="step-num">3</span> Come back here and click: <button class="btn btn-primary" onclick="App.loadClusterInfo()" style="font-size:12px;padding:5px 14px;margin-left:4px">&#8635; Retry Connection</button></li>
        </ol>`;
    } else {
      pfDetail.textContent = 'not running';
      hint.style.display = 'none';
    }
  },

  updateSandboxStatus(connected, target) {
    const statusEl = document.getElementById('status-indicator');
    const sendBtn = document.getElementById('send-btn');

    if (connected) {
      const clusterName = (this.clusterInfo && this.clusterInfo.cluster && this.clusterInfo.cluster.server)
        ? this.clusterInfo.cluster.server.replace(/^https?:\/\//, '').replace(/:6443$/, '')
        : target || 'BBR';
      statusEl.textContent = 'Connected to ' + clusterName;
      statusEl.className = 'status connected';
      sendBtn.disabled = false;
    } else {
      if (!this.clusterConnected) {
        statusEl.innerHTML = 'Not connected \u2014 go to the <a href="#" onclick="App.switchTab(\'environment\');return false" style="color:var(--accent)">Environment</a> tab to log in to a cluster';
      } else {
        statusEl.innerHTML = 'BBR not reachable \u2014 open a terminal and run the <code style="color:var(--green)">oc port-forward</code> command from the <a href="#" onclick="App.switchTab(\'environment\');return false" style="color:var(--accent)">Environment</a> tab, then refresh this page';
      }
      statusEl.className = 'status disconnected';
      sendBtn.disabled = true;
    }
  },

  // ===== Sandbox Tab =====

  async loadPresets() {
    try {
      const resp = await fetch('/api/presets');
      this.presets = await resp.json();
      this.renderPresets();
    } catch (e) {
      console.error('Failed to load presets:', e);
    }
  },

  renderPresets() {
    const container = document.getElementById('presets-container');
    container.innerHTML = '';
    const filtered = this.presets.filter(p => p.phase === this.phase);
    for (const preset of filtered) {
      const btn = document.createElement('button');
      btn.className = 'btn btn-preset';
      btn.textContent = preset.name;
      btn.title = preset.description;
      btn.addEventListener('click', () => this.loadPreset(preset));
      container.appendChild(btn);
    }
  },

  loadPreset(preset) {
    HeadersEditor.setHeaders(preset.headers);
    JsonEditor.setValue(preset.body);
    for (const btn of document.querySelectorAll('.btn-preset')) {
      btn.classList.toggle('active', btn.textContent === preset.name);
    }
  },

  setPhase(phase) {
    this.phase = phase;
    for (const btn of document.querySelectorAll('.btn-phase')) {
      btn.classList.toggle('active', btn.dataset.phase === phase);
    }
    this.renderPresets();
    document.getElementById('result-headers').innerHTML = '<div class="placeholder">Send a request to see results</div>';
    document.getElementById('result-body').innerHTML = '<div class="placeholder">Send a request to see results</div>';
    document.getElementById('body-status').textContent = '';
    document.getElementById('body-status').className = 'body-status';
    document.getElementById('duration').textContent = '';
  },

  async send() {
    if (!this.connected) return;

    const headers = HeadersEditor.getHeaders();
    const body = JsonEditor.getValue();
    if (body === null) {
      document.getElementById('body-error').textContent = 'Fix JSON errors before sending';
      return;
    }

    const sendBtn = document.getElementById('send-btn');
    const durationEl = document.getElementById('duration');
    sendBtn.classList.add('loading');
    sendBtn.disabled = true;
    durationEl.textContent = '';

    const endpoint = this.phase === 'request' ? '/api/send/request' : '/api/send/response';

    try {
      const resp = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ headers, body })
      });
      const result = await resp.json();

      if (result.error) {
        document.getElementById('result-headers').innerHTML =
          `<div class="placeholder" style="color:var(--red)">${esc(result.error)}</div>`;
        document.getElementById('result-body').innerHTML = '<div class="placeholder">Error occurred</div>';
        return;
      }

      DiffRenderer.renderHeaderMutations('result-headers', result.headerMutations);
      DiffRenderer.renderBodyResult('result-body', 'body-status', body, result);
      durationEl.textContent = `${result.durationMs.toFixed(2)}ms`;
      if (result.clearRouteCache) durationEl.textContent += ' | route cache cleared';
    } catch (e) {
      document.getElementById('result-headers').innerHTML =
        `<div class="placeholder" style="color:var(--red)">Request failed: ${esc(e.message)}</div>`;
    } finally {
      sendBtn.classList.remove('loading');
      sendBtn.disabled = false;
    }
  }
};

function esc(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

document.addEventListener('DOMContentLoaded', () => App.init());
