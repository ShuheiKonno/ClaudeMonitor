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
    --err-bg: rgba(248,113,113,0.15);
    --err-border: rgba(248,113,113,0.4);
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
    padding: 4px 8px;
    background: rgba(0,0,0,0.25);
    border-bottom: 1px solid var(--border);
    cursor: move;
  }
  .title {
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.04em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .title-actions { display: flex; gap: 2px; flex-shrink: 0; }
  .title-btn {
    width: 18px; height: 18px;
    display: flex; align-items: center; justify-content: center;
    background: transparent;
    border: none;
    color: var(--fg-dim);
    cursor: pointer;
    border-radius: 3px;
    font-size: 11px;
  }
  .title-btn:hover { background: rgba(255,255,255,0.1); color: var(--fg); }

  .content {
    flex: 1;
    padding: 6px 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    overflow: hidden;
  }

  .row-bar {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 3px;
    padding: 6px 10px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 4px;
  }
  .row-top {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .row-label {
    font-size: 11px;
    color: var(--fg-dim);
    min-width: 32px;
    flex-shrink: 0;
  }
  .bar {
    flex: 1;
    height: 16px;
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
    font-size: 12px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--fg);
    min-width: 30px;
    text-align: right;
    flex-shrink: 0;
  }
  .bar-reset {
    font-size: 10px;
    color: var(--fg-dim);
    font-variant-numeric: tabular-nums;
    text-align: right;
  }

  .footer {
    display: flex;
    justify-content: space-between;
    font-size: 9px;
    color: var(--fg-dim);
    padding: 2px 0 0;
  }

  .auth-banner {
    display: none;
    background: var(--err-bg);
    border: 1px solid var(--err-border);
    border-radius: 4px;
    padding: 6px 8px;
    font-size: 10px;
    line-height: 1.4;
  }
  .auth-banner.show { display: block; }
  .auth-banner strong { color: #fca5a5; }
  .auth-banner code {
    background: rgba(0,0,0,0.3);
    padding: 1px 4px;
    border-radius: 2px;
    font-size: 10px;
  }

  /* 設定パネル */
  .settings {
    display: none;
    flex: 1;
    padding: 6px 8px;
    flex-direction: column;
    gap: 4px;
    overflow: hidden;
  }
  .settings.active { display: flex; }
  .content.hidden { display: none; }

  .settings .group {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 5px 8px;
  }
  .settings .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .settings label {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
  }
  .settings button {
    padding: 4px 10px;
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
  .settings .account {
    font-size: 10px;
    color: var(--fg-dim);
    line-height: 1.3;
  }
  .settings .account b { color: var(--fg); font-weight: 600; font-size: 11px; }
</style>
</head>
<body>
  <div class="titlebar" id="titlebar">
    <div class="title" id="title-text">Claude モニター</div>
    <div class="title-actions">
      <button class="title-btn" id="btn-refresh" title="更新">⟳</button>
      <button class="title-btn" id="btn-settings" title="設定">⚙</button>
      <button class="title-btn" id="btn-close" title="閉じる">✕</button>
    </div>
  </div>

  <div class="content" id="main-view">
    <div class="auth-banner" id="auth-banner">
      <strong id="auth-banner-title">認証エラー</strong>
      <div id="auth-banner-body">ターミナルで <code>claude</code> を実行してログインしてください。</div>
    </div>

    <div class="row-bar">
      <div class="row-top">
        <span class="row-label">5時間</span>
        <div class="bar"><div class="bar-fill" id="bar-5h" style="width:0%"></div></div>
        <span class="bar-pct" id="pct-5h">—</span>
      </div>
      <div class="bar-reset" id="reset-5h"></div>
    </div>

    <div class="row-bar">
      <div class="row-top">
        <span class="row-label">7日</span>
        <div class="bar"><div class="bar-fill" id="bar-7d" style="width:0%"></div></div>
        <span class="bar-pct" id="pct-7d">—</span>
      </div>
      <div class="bar-reset" id="reset-7d"></div>
    </div>

    <div class="footer">
      <span id="account-label"></span>
      <span id="updated">未更新</span>
    </div>
  </div>

  <div class="settings" id="settings-view">
    <div class="group">
      <div class="account" id="account-info">取得中…</div>
    </div>

    <div class="group">
      <div class="row" style="gap:12px">
        <label><input type="checkbox" id="topmost"> 最前面</label>
        <label><input type="checkbox" id="transparent"> 半透明</label>
      </div>
    </div>

    <div class="row" style="justify-content: flex-end; gap: 6px; margin-top: auto;">
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
  if (diff < 60) return Math.floor(diff) + '秒前';
  if (diff < 3600) return Math.floor(diff/60) + '分前';
  if (diff < 86400) return Math.floor(diff/3600) + '時間前';
  return Math.floor(diff/86400) + '日前';
}

function formatCountdown(iso) {
  if (!iso) return '';
  const t = new Date(iso);
  const sec = Math.max(0, Math.floor((t.getTime() - Date.now()) / 1000));
  if (sec < 60) return sec + 's';
  const m = Math.floor(sec / 60);
  if (m < 60) return m + 'm';
  const h = Math.floor(m / 60);
  const rem = m % 60;
  if (h < 24) return rem > 0 ? h + 'h ' + rem + 'm' : h + 'h';
  const d = Math.floor(h / 24);
  return d + 'd';
}

function applyWindow(prefix, win) {
  const pctEl = document.getElementById('pct-' + prefix);
  const barEl = document.getElementById('bar-' + prefix);
  const resetEl = document.getElementById('reset-' + prefix);
  if (!win || typeof win.utilization !== 'number' || (win.utilization === 0 && !win.resetsAt)) {
    // サーバが 0% を返すこともあるが resetsAt が無ければ未取得扱い
    pctEl.textContent = '—';
    barEl.style.width = '0%';
    barEl.classList.remove('warn', 'crit');
    resetEl.textContent = '';
    return;
  }
  const pct = Math.max(0, Math.min(100, Math.round(win.utilization)));
  pctEl.textContent = pct + '%';
  barEl.style.width = pct + '%';
  barEl.classList.remove('warn', 'crit');
  if (pct >= 95) barEl.classList.add('crit');
  else if (pct >= 80) barEl.classList.add('warn');
  resetEl.textContent = formatCountdown(win.resetsAt);
}

let lastUpdated = null;
let lastSnapshot = null;

function renderAuthBanner(snap) {
  const banner = document.getElementById('auth-banner');
  const titleEl = document.getElementById('auth-banner-title');
  const bodyEl = document.getElementById('auth-banner-body');
  if (!snap || snap.authState === 'ok' || snap.authState === 'init') {
    banner.classList.remove('show');
    return;
  }
  banner.classList.add('show');
  switch (snap.authState) {
    case 'missing':
      titleEl.textContent = '未ログイン';
      bodyEl.innerHTML = 'ターミナルで <code>claude</code> を実行してログインしてください。';
      break;
    case 'expired':
      titleEl.textContent = 'トークン期限切れ';
      bodyEl.innerHTML = 'ターミナルで <code>claude</code> を実行して再ログインしてください。';
      break;
    case 'network_error':
      titleEl.textContent = '取得失敗';
      bodyEl.textContent = snap.lastError || 'ネットワークエラー';
      break;
    default:
      titleEl.textContent = 'エラー';
      bodyEl.textContent = snap.lastError || '';
  }
}

function renderAccount(snap) {
  const label = document.getElementById('account-label');
  const info = document.getElementById('account-info');
  const plan = snap.subscriptionType || '';
  const name = snap.displayName || snap.email || '';
  label.textContent = plan ? plan : '';
  if (!name) {
    info.textContent = '未ログイン';
    return;
  }
  info.innerHTML = '<b>' + name + '</b>' +
    (snap.email && snap.email !== name ? '<br>' + snap.email : '') +
    (plan ? '<br>プラン: ' + plan : '');
}

async function fetchUsage() {
  try {
    const res = await fetch('/api/usage');
    const snap = await res.json();
    lastSnapshot = snap;
    applyWindow('5h', snap.fiveHour);
    applyWindow('7d', snap.sevenDay);
    renderAuthBanner(snap);
    renderAccount(snap);
    lastUpdated = snap.updatedAt;
    updateFooter();
  } catch (e) {
    document.getElementById('updated').textContent = '取得エラー';
  }
}

function updateFooter() {
  if (lastUpdated) {
    document.getElementById('updated').textContent = formatRelative(lastUpdated);
  }
  if (lastSnapshot) {
    applyWindow('5h', lastSnapshot.fiveHour);
    applyWindow('7d', lastSnapshot.sevenDay);
  }
}

// --- タイトルバーのボタン ---
document.getElementById('btn-close').addEventListener('click', () => {
  fetch('/api/close', { method: 'GET' });
});
document.getElementById('btn-refresh').addEventListener('click', async () => {
  await fetch('/api/refresh');
  fetchUsage();
});

// --- 設定パネル ---
const mainView = document.getElementById('main-view');
const settingsView = document.getElementById('settings-view');

async function openSettings() {
  const res = await fetch('/api/settings');
  const s = await res.json();
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
