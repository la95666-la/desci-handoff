// jc-release · admin SPA
// vanilla JS · no framework · uses fetch + poll

const state = {
  me: null,
  agents: [],
  releases: [],
  commands: [],
  upgradeCommands: [],
  submittingUpgrades: new Set(),
  latestPerComp: {},   // manifest cache (component → newest Release)
  currentView: 'agents',
  pollTimer: null,
  requests: new Map(),
  logDetails: new Map(),
  openLogs: new Set(),
  logRevision: 0,
  releaseRevision: 0,
};

// ---------- HTTP helpers ----------
async function api(path, opts = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 12000);
  try {
  const r = await fetch(path, { ...opts, signal: controller.signal });
  if (r.status === 401) {
    window.location.href = '/login';
    throw new Error('unauthorized');
  }
  const j = await r.json();
  if (!r.ok || !j.status) throw new Error(j.message || `HTTP ${r.status}`);
  return j.data;
  } catch (error) {
    if (error.name === 'AbortError') throw new Error('请求超时，请重试；已有记录保留。');
    throw error;
  } finally { clearTimeout(timer); }
}

// Coalesce only in-flight GETs, never queue duplicate writes or cache authority.
function readOnce(path) {
  if (state.requests.has(path)) return state.requests.get(path);
  const request = api(path).finally(() => state.requests.delete(path));
  state.requests.set(path, request);
  return request;
}

async function apiJSON(path, method, body) {
  return api(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
}

// ---------- misc ----------
function fmtRel(t) {
  const d = new Date(t);
  const s = (Date.now() - d.getTime()) / 1000;
  if (s < 60) return `${Math.floor(s)}s 前`;
  if (s < 3600) return `${Math.floor(s / 60)}m 前`;
  if (s < 86400) return `${Math.floor(s / 3600)}h 前`;
  return `${Math.floor(s / 86400)}d 前`;
}
function fmtSize(n) {
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB';
}
function esc(s) {
  return String(s || '').replace(/[<>&"']/g, c => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;', '"': '&quot;', "'": '&#39;' }[c]));
}

// ---------- init ----------
async function init() {
  try {
    state.me = await api('/api/v1/auth/me');
    document.getElementById('me').textContent = state.me.username;
  } catch { return; }
  setupTabs();
  await loadAgents();
  await loadUpgradeStatus();
  startPolling();
}

function setupTabs() {
  document.querySelectorAll('nav .tab').forEach(t => {
    t.addEventListener('click', e => {
      e.preventDefault();
      switchView(t.dataset.tab);
    });
  });
}

function switchView(name) {
  state.currentView = name;
  document.querySelectorAll('nav .tab').forEach(t => t.classList.toggle('active', t.dataset.tab === name));
  document.querySelectorAll('.view').forEach(v => v.classList.add('hidden'));
  document.getElementById('view-' + name).classList.remove('hidden');
  if (name === 'agents') loadAgents();
  else if (name === 'releases') loadReleases();
  else if (name === 'logs') loadLogs();
}

async function doLogout() {
  await apiJSON('/api/v1/auth/logout', 'POST');
  window.location.href = '/login';
}

// ---------- Agents ----------
async function loadAgents() {
  try {
    state.agents = await readOnce('/api/v1/agents');
  } catch (e) { renderAgentsError(e.message); return; }
  renderAgents();
  populateServiceLogAgents();
  // Overview is optional decoration. Never block agents or log selectors on it.
  loadReleaseOverview().then(() => { if (state.currentView === 'agents') renderAgents(); });
}

function populateServiceLogAgents() {
  const select = document.getElementById('service-log-agent');
  if (!select) return;
  if (!state.agents || state.agents.length === 0) {
    select.innerHTML = '<option value="">No agents</option>';
    return;
  }
  const selected = select.value;
  select.innerHTML = state.agents
    .map(agent => `<option value="${esc(agent.id)}">${esc(agent.hostname)}</option>`)
    .join('');
  if (state.agents.some(a => a.id === selected)) select.value = selected;
}

function renderAgents() {
  const box = document.getElementById('agents-list');
  if (!state.agents || state.agents.length === 0) {
    box.innerHTML = `<div class="card"><em>暂无 agent · 点上方"生成 Enroll Token"添加</em></div>`;
    // still populate log-agent filter (empty)
    const sel = document.getElementById('log-agent');
    if (sel) sel.innerHTML = '<option value="">全部</option>';
    return;
  }
  box.innerHTML = state.agents.map(a => renderAgentCard(a)).join('');
  const sel = document.getElementById('log-agent');
  if (sel) {
    const selected = sel.value;
    sel.innerHTML = '<option value="">全部</option>' + state.agents.map(a => `<option value="${esc(a.id)}">${esc(a.hostname)}</option>`).join('');
    if (state.agents.some(a => a.id === selected)) sel.value = selected;
  }
}
function renderAgentsError(msg) {
  document.getElementById('agents-list').innerHTML = `<div class="card err">${esc(msg)}</div>`;
}

function renderAgentCard(a) {
  const comps = ['backend', 'frontend', 'admin', 'job'];
  const cells = comps.map(c => renderCompCell(a, c)).join('');
  return `
    <div class="card">
      <div class="agent-row">
        <div class="agent-info">
          <div class="h">${esc(a.hostname)}</div>
          <div class="m">${esc(a.ip)} · 上次心跳 ${fmtRel(a.last_seen_at)}</div>
        </div>
        ${cells}
        <div><button class="btn-small btn-danger" onclick="delAgent('${esc(a.id)}')">删</button></div>
      </div>
    </div>
  `;
}

function renderCompCell(agent, comp) {
  const key = `${agent.id}:${comp}`;
  const command = state.upgradeCommands.find(c => c.agent_id === agent.id && c.component === comp && c.kind === 'upgrade');
  const active = state.submittingUpgrades.has(key) || command && ['pending', 'in_progress'].includes(command.status);
  const deployed = agent.deployed ? agent.deployed[comp] : '';
  const latest = state.latestPerComp[comp];
  const isLatest = latest && deployed === latest.id;
  const btnLabel = latest ? (isLatest ? '最新' : '升级到最新') : '无版本';
  const disabled = !latest || isLatest || active;
  const verStr = deployed ? findReleaseVer(deployed) || deployed.slice(0, 6) : '—';
  return `
    <div class="comp-cell">
      <div><strong>${comp}</strong></div>
      <div class="ver mono">${esc(verStr)}</div>
      ${command ? `<div class="upgrade-state" role="status"><span class="pill ${esc(command.status)}">${esc(command.status)}</span> <small>${esc(command.id.slice(0, 8))}</small>${command.status === 'failed' ? `<div class="err">${esc(command.error || '升级失败，请查看日志')}</div>` : ''}</div>` : ''}
      <div class="actions">
        <button class="btn-small" ${disabled ? 'disabled' : ''} onclick="upgradeAgent('${esc(agent.id)}','${comp}','${latest ? esc(latest.id) : ''}')">${active ? '等待 / 升级中…' : btnLabel}</button>
        <button class="btn-small btn-danger" disabled title="服务端无法核实本地备份对应的目标版本；不会发送未核实的回滚指令">回滚不可用</button>
      </div>
    </div>
  `;
}

function findReleaseVer(id) {
  const r = (state.releases || []).find(x => x.id === id);
  return r ? r.version : null;
}

async function delAgent(id) {
  if (!confirm('删除该 agent？（agent 侧 token 会失效，需要重新 enroll）')) return;
  try { await apiJSON('/api/v1/agents/' + id, 'DELETE'); await loadAgents(); }
  catch (e) { alert('删除失败: ' + e.message); }
}

async function genEnrollToken() {
  try {
    const t = await apiJSON('/api/v1/enroll', 'POST');
    document.getElementById('modal-content').innerHTML = `
      <h3>Enroll Token（15 分钟有效，仅可使用一次）</h3>
      <p>把这个 token 填入 agent 配置的 <code>enroll_token</code> 字段，然后启动 agent。</p>
      <div class="enroll-code">${esc(t.token)}</div>
      <p class="hint">到期时间: ${new Date(t.expires_at).toLocaleString()}</p>
    `;
    document.getElementById('modal').classList.remove('hidden');
  } catch (e) { alert('生成失败: ' + e.message); }
}

async function upgradeAgent(agentID, comp, releaseID) {
  if (!releaseID) return;
  const key = `${agentID}:${comp}`;
  if (state.submittingUpgrades.has(key) || state.upgradeCommands.some(c => c.agent_id === agentID && c.component === comp && c.kind === 'upgrade' && ['pending','in_progress'].includes(c.status))) return;
  if (!confirm(`确认升级 ${comp} 到最新版本？\nAgent 会在失败时尝试从本地备份恢复；若恢复失败，必须人工处理。`)) return;
  state.submittingUpgrades.add(key);
  renderAgents();
  try {
    const command = await apiJSON(`/api/v1/agents/${agentID}/commands`, 'POST', { kind: 'upgrade', component: comp, release_id: releaseID });
    // Render the server-issued Pending immediately, before any follow-up GET.
    state.upgradeCommands = [command, ...state.upgradeCommands.filter(c => c.id !== command.id)];
    renderAgents();
    document.getElementById('upgrade-status').textContent = `${comp} 升级命令 ${command.id.slice(0,8)} 已提交，状态将自动更新。`;
    if (state.currentView === 'logs') loadLogs();
  } catch (e) {
    document.getElementById('upgrade-status').textContent = `下发结果未确认：${e.message}。正在查询命令记录，请勿重复下发。`;
    await loadUpgradeStatus();
  } finally { state.submittingUpgrades.delete(key); renderAgents(); }
}

async function loadUpgradeStatus() {
  try {
    const recent = await readOnce('/api/v1/commands?limit=100&summary=true');
    const upgrades = recent.filter(c => c.kind === 'upgrade');
    // Retain and query original known active IDs even outside the last 100.
    for (const old of state.upgradeCommands) {
      if (!upgrades.some(c => c.id === old.id) && ['pending','in_progress'].includes(old.status)) {
        try { upgrades.push(await readOnce('/api/v1/commands/' + encodeURIComponent(old.id))); }
        catch { upgrades.push(old); }
      }
    }
    state.upgradeCommands = upgrades.sort((a,b) => new Date(b.created_at) - new Date(a.created_at));
    if (state.currentView === 'agents') renderAgents();
  } catch (e) { document.getElementById('upgrade-status').textContent = `状态刷新失败：${e.message}；已显示的状态保留。`; }
}

// ---------- Releases ----------
async function loadReleaseOverview() {
  try {
    const releases = await readOnce('/api/v1/releases?limit=200');
    const latest = {};
    releases.forEach(r => {
      if (!latest[r.component] || new Date(latest[r.component].uploaded_at) < new Date(r.uploaded_at)) latest[r.component] = r;
    });
    state.latestPerComp = latest;
  } catch { /* Retain last overview; an optional request cannot erase agents. */ }
}

async function loadReleases() {
  const comp = document.getElementById('rel-comp') ? document.getElementById('rel-comp').value : '';
  const revision = ++state.releaseRevision;
  const status = document.getElementById('releases-status');
  status.textContent = '正在读取版本…';
  try {
    const releases = await readOnce(`/api/v1/releases?component=${encodeURIComponent(comp)}&limit=100`);
    if (revision !== state.releaseRevision) return;
    state.releases = releases;
    renderReleases();
    status.textContent = `已加载 ${releases.length} 条最近版本`;
  } catch (e) { if (revision === state.releaseRevision) status.textContent = `版本读取失败：${e.message}`; }
}

function renderReleases() {
  const box = document.getElementById('releases-list');
  if (!state.releases || state.releases.length === 0) {
    box.innerHTML = '<div class="card"><em>暂无版本</em></div>';
    return;
  }
  const rows = state.releases.map(r => `
    <tr>
      <td>${esc(r.component)}</td>
      <td class="mono">${esc(r.version)}</td>
      <td>${fmtSize(r.size)}</td>
      <td class="mono">${esc(r.sha256.slice(0, 12))}…</td>
      <td>${new Date(r.uploaded_at).toLocaleString()}</td>
      <td>${esc(r.uploaded_by)}</td>
      <td>${esc(r.notes || '—')}</td>
      <td><button class="btn-small btn-danger" onclick="delRelease('${esc(r.id)}')">删</button></td>
    </tr>
  `).join('');
  box.innerHTML = `<table><thead><tr><th>组件</th><th>版本</th><th>大小</th><th>SHA256</th><th>上传时间</th><th>发布者</th><th>说明</th><th></th></tr></thead><tbody>${rows}</tbody></table>`;
}

async function doUpload(e) {
  e.preventDefault();
  const btn = document.getElementById('up-btn');
  const status = document.getElementById('up-status');
  btn.disabled = true;
  status.textContent = '上传中…';
  const fd = new FormData();
  fd.append('component', document.getElementById('up-comp').value);
  fd.append('version', document.getElementById('up-ver').value);
  fd.append('notes', document.getElementById('up-notes').value);
  fd.append('file', document.getElementById('up-file').files[0]);
  try {
    const r = await fetch('/api/v1/releases', { method: 'POST', body: fd });
    const j = await r.json();
    if (!r.ok || !j.status) throw new Error(j.message || '失败');
    status.textContent = '上传成功: ' + j.data.version;
    document.getElementById('upload-form').reset();
    await loadReleases();
    await loadAgents();
  } catch (e) { status.textContent = '失败: ' + e.message; }
  btn.disabled = false;
  return false;
}

async function delRelease(id) {
  if (!confirm('删除该版本？（有 agent 部署此版本时会拒绝）')) return;
  try { await apiJSON('/api/v1/releases/' + id, 'DELETE'); await loadReleases(); }
  catch (e) { alert('失败: ' + e.message); }
}

// ---------- Logs / Commands ----------
let serviceLogPollToken = 0;

async function queryServiceLogs() {
  const agentID = document.getElementById('service-log-agent').value;
  const target = document.getElementById('service-log-target').value;
  const lines = Number(document.getElementById('service-log-lines').value);
  const sinceMinutes = Number(document.getElementById('service-log-since').value);
  const button = document.getElementById('service-log-query');
  const status = document.getElementById('service-log-status');
  const result = document.getElementById('service-log-result');
  if (!agentID) {
    status.textContent = 'Select an agent first.';
    return;
  }
  button.disabled = true;
  result.classList.add('hidden');
  status.textContent = '正在提交最近日志查询...';
  try {
    const command = await apiJSON('/api/v1/service-logs', 'POST', {
      agent_id: agentID,
      target,
      lines,
      since_seconds: sinceMinutes * 60,
    });
    const token = ++serviceLogPollToken;
    status.textContent = `查询 ${command.id.slice(0, 8)} 已提交，正在通知 Agent 读取现有日志...`;
    await pollServiceLog(command.id, token, 0);
  } catch (e) {
    status.textContent = 'Query failed: ' + e.message;
    button.disabled = false;
  }
}

async function pollServiceLog(id, token, attempt) {
  if (token !== serviceLogPollToken) return;
  const button = document.getElementById('service-log-query');
  const status = document.getElementById('service-log-status');
  const result = document.getElementById('service-log-result');
  try {
    const command = await readOnce('/api/v1/service-logs/' + encodeURIComponent(id));
    const elapsed = Math.max(0, attempt * 2);
    if (command.status === 'pending') {
      status.textContent = `正在等待 Agent 接收查询（${elapsed}s），通常会在数秒内返回...`;
    } else if (command.status === 'in_progress') {
      status.textContent = `Agent 正在读取 ${command.component} 的最近日志（${elapsed}s）...`;
    } else {
      status.textContent = `${command.component} 最近日志：${command.status}`;
    }
    if (command.status === 'success' || command.status === 'failed') {
      result.innerHTML = `${command.error ? `<div class="err">${esc(command.error)}</div>` : ''}<pre>${esc(command.log || '(no output)')}</pre>`;
      result.classList.remove('hidden');
      button.disabled = false;
      return;
    }
    if (attempt >= 30) {
      status.textContent = '等待超过 60 秒。请检查 Agent 在线状态后重试；该查询仍保留在下方命令历史中。';
      button.disabled = false;
      return;
    }
    window.setTimeout(() => pollServiceLog(id, token, attempt + 1), 2000);
  } catch (e) {
    status.textContent = 'Result fetch failed: ' + e.message;
    button.disabled = false;
  }
}

async function loadLogs() {
  const agentID = document.getElementById('log-agent') ? document.getElementById('log-agent').value : '';
  const revision = ++state.logRevision;
  const status = document.getElementById('logs-status');
  try {
    const commands = await readOnce(`/api/v1/commands?agent_id=${encodeURIComponent(agentID)}&limit=100&summary=true`);
    if (revision !== state.logRevision) return;
    state.commands = commands;
    status.textContent = `已加载 ${commands.length} 条记录；点击展开时读取日志正文`;
    renderLogs();
  } catch (e) { if (revision === state.logRevision) status.textContent = `历史记录读取失败：${e.message}`; }
}

function renderLogs() {
  const box = document.getElementById('logs-list');
  if (!state.commands || state.commands.length === 0) {
    box.innerHTML = '<div class="card"><em>暂无升级记录</em></div>';
    return;
  }
  const rows = state.commands.map(c => {
    const durMs = c.finished_at && c.started_at ? (new Date(c.finished_at) - new Date(c.started_at)) : null;
    const dur = durMs ? `${(durMs / 1000).toFixed(1)}s` : (c.status === 'in_progress' ? '进行中…' : '—');
    const agent = (state.agents.find(a => a.id === c.agent_id) || {}).hostname || c.agent_id.slice(0, 6);
    return `
      <tr class="expandable" onclick="toggleLog('${esc(c.id)}')">
        <td class="mono">${esc(c.id.slice(0, 8))}</td>
        <td>${esc(agent)}</td>
        <td>${esc(c.kind)}</td>
        <td>${esc(c.component)}</td>
        <td><span class="pill ${c.status}">${esc(c.status)}</span></td>
        <td>${new Date(c.created_at).toLocaleString()}</td>
        <td>${dur}</td>
        <td>${esc(c.issued_by || '—')}</td>
        <td><button type="button" class="btn-small log-toggle" aria-expanded="${state.openLogs.has(c.id)}" onclick="event.stopPropagation(); toggleLog('${esc(c.id)}')">${state.openLogs.has(c.id) ? '收起日志' : '查看日志'}</button></td>
      </tr>
      <tr id="log-detail-${esc(c.id)}" class="log-detail ${state.openLogs.has(c.id) ? '' : 'hidden'}">
        <td colspan="9">
          <div class="log-expand">
            ${c.error ? `<div class="err">错误: ${esc(c.error)}</div>` : ''}
            <pre>${esc(state.logDetails.get(c.id)?.text || (state.openLogs.has(c.id) ? '正在读取日志…' : '展开后读取日志'))}</pre>
          </div>
        </td>
      </tr>
    `;
  }).join('');
  box.innerHTML = `<table><thead><tr><th>命令ID</th><th>Agent</th><th>类型</th><th>组件</th><th>状态</th><th>创建时间</th><th>耗时</th><th>发起人</th><th>日志</th></tr></thead><tbody>${rows}</tbody></table>`;
}

async function toggleLog(id) {
  const el = document.getElementById('log-detail-' + id);
  if (!el) return;
  if (state.openLogs.has(id)) { state.openLogs.delete(id); renderLogs(); return; }
  state.openLogs.add(id); renderLogs();
  await loadLogDetail(id);
}

async function loadLogDetail(id) {
  const cached = state.logDetails.get(id);
  if (cached?.terminal) return;
  try {
    const command = await readOnce('/api/v1/commands/' + encodeURIComponent(id));
    const text = command.log || '（无日志）';
    state.logDetails.set(id, { text, terminal: command.status === 'success' || command.status === 'failed' });
    const pre = document.querySelector(`#log-detail-${CSS.escape(id)} pre`);
    if (pre) pre.textContent = text;
  } catch (e) {
    const pre = document.querySelector(`#log-detail-${CSS.escape(id)} pre`);
    if (pre) pre.textContent = `日志读取失败：${e.message}；收起后展开可重试。`;
  }
}

// ---------- Poll for in-progress ----------
function startPolling() {
  if (state.pollTimer) clearTimeout(state.pollTimer);
  state.pollTimer = setTimeout(async () => {
    try {
    // refresh commands if any in-progress or the logs view is active
    if (document.visibilityState !== 'hidden') {
      if (state.currentView === 'agents' || state.upgradeCommands.some(c => ['pending','in_progress'].includes(c.status))) await loadUpgradeStatus();
      if (state.currentView === 'logs') await loadLogs();
      for (const id of state.openLogs) await loadLogDetail(id);
      if (state.currentView === 'agents') await loadAgents();
    }
    } finally { startPolling(); }
  }, 3000);
}

// ---------- Modal ----------
function closeModal(e) {
  if (e && e.target !== e.currentTarget) return;
  document.getElementById('modal').classList.add('hidden');
  document.getElementById('modal-content').innerHTML = '';
}

// Init when DOM ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}

// Expose for onclick handlers
window.doLogout = doLogout;
window.loadAgents = loadAgents;
window.loadReleases = loadReleases;
window.loadLogs = loadLogs;
window.queryServiceLogs = queryServiceLogs;
window.delAgent = delAgent;
window.genEnrollToken = genEnrollToken;
window.upgradeAgent = upgradeAgent;
window.doUpload = doUpload;
window.delRelease = delRelease;
window.toggleLog = toggleLog;
window.closeModal = closeModal;
