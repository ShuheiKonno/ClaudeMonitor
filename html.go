package main

func getHTML() string {
	return `<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Claude モニター</title>
<style>
  :root {
    --bg-grad: linear-gradient(135deg, #1e2030 0%, #2b2845 100%);
    --fg: #e5e7ee;
    --fg-dim: #9ea3b5;
    --accent: #8b9eff;
    --card: rgba(255,255,255,0.05);
    --border: rgba(255,255,255,0.08);
    --bar-bg: rgba(255,255,255,0.1);
    --bar-ok: #4ade80;
    --bar-warn: #facc15;
    --bar-crit: #f87171;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  html, body {
    width: 100%;
    height: 100%;
    overflow: hidden;
    font-family: "Yu Gothic UI", "Meiryo", "Segoe UI", sans-serif;
    font-size: 12px;
    color: var(--fg);
    background: var(--bg-grad);
    user-select: none;
  }
  body { display: flex; flex-direction: column; }

  .titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    background: rgba(0,0,0,0.25);
    border-bottom: 1px solid var(--border);
    cursor: move;
  }
  .title {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.04em;
  }
  .title-actions { display: flex; gap: 4px; }
  .title-btn {
    width: 20px; height: 20px;
    display: flex; align-items: center; justify-content: center;
    background: transparent;
    border: none;
    color: var(--fg-dim);
    cursor: pointer;
    border-radius: 3px;
    font-size: 12px;
  }
  .title-btn:hover { background: rgba(255,255,255,0.1); color: var(--fg); }

  .content {
    flex: 1;
    padding: 10px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    overflow: hidden;
  }

  .section {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 8px 10px;
  }
  .section-title {
    font-size: 10px;
    font-weight: 600;
    color: var(--fg-dim);
    letter-spacing: 0.05em;
    margin-bottom: 6px;
  }

  .metric-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 11px;
    margin-top: 3px;
  }
  .metric-label { color: var(--fg-dim); }
  .metric-value { font-variant-numeric: tabular-nums; }

  .bar-row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
  }
  .bar {
    flex: 1;
    height: 6px;
    background: var(--bar-bg);
    border-radius: 3px;
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    background: var(--bar-ok);
    transition: width 0.3s ease, background 0.3s ease;
  }
  .bar-fill.warn { background: var(--bar-warn); }
  .bar-fill.crit { background: var(--bar-crit); }
  .bar-pct {
    font-size: 10px;
    font-variant-numeric: tabular-nums;
    color: var(--fg-dim);
    min-width: 30px;
    text-align: right;
  }
  .bar-detail {
    font-size: 10px;
    color: var(--fg-dim);
    font-variant-numeric: tabular-nums;
    margin-top: 2px;
  }

  .footer {
    text-align: center;
    font-size: 10px;
    color: var(--fg-dim);
    padding: 4px 0;
  }

  /* 設定パネル */
  .settings {
    display: none;
    flex: 1;
    padding: 10px;
    flex-direction: column;
    gap: 8px;
    overflow: hidden;
  }
  .settings.active { display: flex; }
  .content.hidden { display: none; }

  .settings label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
  }
  .settings input[type="number"],
  .settings select {
    flex: 1;
    background: rgba(0,0,0,0.3);
    border: 1px solid var(--border);
    color: var(--fg);
    padding: 3px 6px;
    border-radius: 3px;
    font-size: 11px;
    font-family: inherit;
  }
  .settings .group {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 8px 10px;
  }
  .settings .group-title {
    font-size: 10px;
    font-weight: 600;
    color: var(--fg-dim);
    margin-bottom: 6px;
  }
  .settings .row {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
  }
  .settings button {
    padding: 6px 12px;
    background: var(--accent);
    color: #12152a;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 600;
    font-size: 11px;
    font-family: inherit;
  }
  .settings button:hover { opacity: 0.85; }
</style>
</head>
<body>
  <div class="titlebar" id="titlebar">
    <div class="title">Claude モニター</div>
    <div class="title-actions">
      <button class="title-btn" id="btn-refresh" title="更新">⟳</button>
      <button class="title-btn" id="btn-settings" title="設定">⚙</button>
      <button class="title-btn" id="btn-close" title="閉じる">✕</button>
    </div>
  </div>

  <div class="content" id="main-view">
    <div class="section">
      <div class="section-title">直近5時間</div>
      <div class="bar-row">
        <div class="metric-label" style="min-width:56px">トークン</div>
        <div class="bar"><div class="bar-fill" id="bar5h-tok" style="width:0%"></div></div>
        <div class="bar-pct" id="pct5h-tok">0%</div>
      </div>
      <div class="bar-detail" id="detail5h-tok">0 / 0</div>
    </div>

    <div class="section">
      <div class="section-title">直近7日間</div>
      <div class="bar-row">
        <div class="metric-label" style="min-width:56px">トークン</div>
        <div class="bar"><div class="bar-fill" id="bar7d-tok" style="width:0%"></div></div>
        <div class="bar-pct" id="pct7d-tok">0%</div>
      </div>
      <div class="bar-detail" id="detail7d-tok">0 / 0</div>
    </div>

    <div class="footer" id="updated">未更新</div>
  </div>

  <div class="settings" id="settings-view">
    <div class="group">
      <div class="row">
        <span style="min-width:48px">プラン</span>
        <select id="plan-select">
          <option value="auto">自動推定（履歴から）</option>
          <option value="pro">Claude Pro</option>
          <option value="max100">Claude Max $100</option>
          <option value="max200">Claude Max $200</option>
        </select>
      </div>
    </div>

    <div class="group">
      <div class="group-title">表示オプション</div>
      <div class="row" style="gap:12px">
        <label><input type="checkbox" id="topmost"> 最前面</label>
        <label><input type="checkbox" id="transparent"> 半透明</label>
      </div>
    </div>

    <div class="row" style="justify-content: flex-end; gap: 8px;">
      <button id="btn-cancel" style="background: rgba(255,255,255,0.1); color: var(--fg);">戻る</button>
      <button id="btn-save">保存</button>
    </div>
  </div>

<script>
const numFmt = new Intl.NumberFormat('ja-JP');

function formatRelative(iso) {
  const t = new Date(iso);
  const diff = (Date.now() - t.getTime()) / 1000;
  if (diff < 5) return 'たった今';
  if (diff < 60) return Math.floor(diff) + '秒前に更新';
  if (diff < 3600) return Math.floor(diff/60) + '分前に更新';
  if (diff < 86400) return Math.floor(diff/3600) + '時間前に更新';
  return Math.floor(diff/86400) + '日前に更新';
}

function applyBar(pctEl, barEl, detailEl, used, limit) {
  const pct = limit > 0 ? Math.min(100, Math.round(used * 100 / limit)) : 0;
  pctEl.textContent = limit > 0 ? (pct + '%') : '—';
  barEl.style.width = (limit > 0 ? pct : 0) + '%';
  barEl.classList.remove('warn', 'crit');
  if (pct >= 95) barEl.classList.add('crit');
  else if (pct >= 80) barEl.classList.add('warn');
  const limitText = limit > 0 ? ('約' + numFmt.format(limit)) : '未設定';
  detailEl.textContent = numFmt.format(used) + ' / ' + limitText;
}

let lastUpdated = null;

async function fetchUsage() {
  try {
    const res = await fetch('/api/usage');
    const u = await res.json();
    applyBar(
      document.getElementById('pct5h-tok'),
      document.getElementById('bar5h-tok'),
      document.getElementById('detail5h-tok'),
      u.fiveHour.tokens, u.fiveHour.limitTokens);

    applyBar(
      document.getElementById('pct7d-tok'),
      document.getElementById('bar7d-tok'),
      document.getElementById('detail7d-tok'),
      u.sevenDay.tokens, u.sevenDay.limitTokens);

    lastUpdated = u.updatedAt;
    updateFooter();
  } catch (e) {
    document.getElementById('updated').textContent = '取得エラー';
  }
}

function updateFooter() {
  if (lastUpdated) {
    document.getElementById('updated').textContent = formatRelative(lastUpdated);
  }
}

// --- タイトルバーのボタン ---
document.getElementById('btn-close').addEventListener('click', () => {
  fetch('/api/close', { method: 'GET' });
});
document.getElementById('btn-refresh').addEventListener('click', async () => {
  const res = await fetch('/api/refresh');
  const u = await res.json();
  lastUpdated = u.updatedAt;
  fetchUsage();
});

// --- 設定パネル ---
const mainView = document.getElementById('main-view');
const settingsView = document.getElementById('settings-view');

async function openSettings() {
  const res = await fetch('/api/settings');
  const s = await res.json();
  document.getElementById('plan-select').value = s.plan || 'auto';
  document.getElementById('topmost').checked = !!s.topmost;
  document.getElementById('transparent').checked = !!s.transparent;
  mainView.classList.add('hidden');
  settingsView.classList.add('active');
}
function closeSettings() {
  settingsView.classList.remove('active');
  mainView.classList.remove('hidden');
}
document.getElementById('btn-settings').addEventListener('click', openSettings);
document.getElementById('btn-cancel').addEventListener('click', closeSettings);
document.getElementById('btn-save').addEventListener('click', async () => {
  const payload = {
    plan: document.getElementById('plan-select').value,
    topmost: document.getElementById('topmost').checked,
    transparent: document.getElementById('transparent').checked,
  };
  await fetch('/api/setoption', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(payload),
  });
  closeSettings();
  fetchUsage();
});

// --- ウィンドウドラッグ ---
let dragging = false;
const titlebar = document.getElementById('titlebar');
titlebar.addEventListener('mousedown', async (e) => {
  if (e.target.closest('.title-btn')) return;
  dragging = true;
  await fetch('/api/dragstart');
});
document.addEventListener('mousemove', () => {
  if (dragging) fetch('/api/dragmove');
});
document.addEventListener('mouseup', () => {
  if (dragging) {
    dragging = false;
    fetch('/api/persistwindow');
  }
});

fetchUsage();
setInterval(fetchUsage, 60000);
setInterval(updateFooter, 5000);
</script>
</body>
</html>`
}
