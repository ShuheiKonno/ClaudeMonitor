package main

func getHTML() string {
	return htmlTemplate
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Claude / Codex モニター</title>
<style>
  :root {
    --bg-grad: linear-gradient(135deg, #0a0a0d 0%, #15161c 100%);
    --fg: #ffffff;
    --fg-dim: #b8bcc8;
    --accent: #8b9eff;
    --card: rgba(255,255,255,0.035);
    --border: rgba(255,255,255,0.06);
    --bar-bg: rgba(255,255,255,0.07);
    --bar-ok: #4ade80;
    --bar-warn: #facc15;
    --bar-crit: #f87171;
    --err-bg: rgba(248,113,113,0.12);
    --err-border: rgba(248,113,113,0.35);
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
    background: rgba(0,0,0,0.45);
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
    gap: 5px;
    overflow: hidden;
  }

  .provider-tabs {
    display: flex;
    gap: 4px;
    flex-shrink: 0;
  }
  .provider-tab {
    flex: 1;
    height: 22px;
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--card);
    color: var(--fg-dim);
    font: inherit;
    font-size: 10px;
    cursor: pointer;
  }
  .provider-tab.active {
    color: var(--fg);
    border-color: rgba(139,158,255,0.55);
    background: rgba(139,158,255,0.16);
  }
  .provider-tab.provider-hidden, .provider-card.provider-hidden { display: none; }

  .overview-view {
    flex-direction: row;
    gap: 6px;
  }
  .provider-card {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 5px;
    background: rgba(255,255,255,0.025);
    border: 1px solid var(--border);
    border-radius: 5px;
  }
  .provider-card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 18px;
    gap: 5px;
  }
  .provider-card-name { font-size: 11px; font-weight: 700; }
  .provider-card-plan {
    min-width: 0;
    color: var(--fg-dim);
    font-size: 9px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .overview-message {
    display: none;
    flex-shrink: 0;
    padding: 3px 5px;
    border-radius: 3px;
    font-size: 9px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .overview-message.show { display: block; }
  .overview-message.auth { color: #fca5a5; background: var(--err-bg); cursor: pointer; }
  .overview-message.incident { color: #fcd34d; background: rgba(250,204,21,0.10); }
  .overview-window {
    display: flex;
    flex-direction: column;
    justify-content: center;
    min-height: 48px;
    padding: 4px 6px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 4px;
  }
  .overview-window .row-label { width: 34px; }
  .overview-extra {
    display: none;
    min-height: 31px;
    padding: 4px 6px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: 9px;
    color: var(--fg-dim);
  }
  .overview-extra.show { display: flex; flex-direction: column; justify-content: center; gap: 2px; }
  .overview-extra-top {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
  }
  .overview-extra-label { width: 34px; flex-shrink: 0; }
  .overview-extra-detail {
    display: flex;
    justify-content: space-between;
    gap: 4px;
    width: 100%;
    min-width: 0;
  }
  .overview-extra-detail span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .overview-extra strong { color: var(--fg); font-size: 10px; }
  .overview-status { margin-top: auto; }
  .overview-status .status-tile { height: 24px; padding: 0 2px; font-size: 9px; }
  .overview-links { display: flex; justify-content: center; gap: 12px; font-size: 9px; }
  .overview-account {
    display: flex;
    justify-content: space-between;
    gap: 4px;
    color: var(--fg-dim);
    font-size: 8px;
    white-space: nowrap;
    overflow: hidden;
  }
  .overview-account span { overflow: hidden; text-overflow: ellipsis; }

  .row-bar {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 2px;
    padding: 3px 8px;
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
    width: 36px;
    flex-shrink: 0;
  }
  .bar {
    flex: 1;
    height: 12px;
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
    font-size: 11px;
    color: var(--fg-dim);
    font-variant-numeric: tabular-nums;
    text-align: right;
  }
  #overage-limit { font-weight: 600; }

  /* ステータスタイル */
  .status-row {
    display: flex;
    gap: 4px;
    flex-shrink: 0;
  }
  .status-tile {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    height: 26px;
    padding: 0 4px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: 10px;
    color: var(--fg-dim);
    overflow: hidden;
  }
  .status-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
    background: rgba(255,255,255,0.2);
  }
  .status-tile.operational { border-color: rgba(74,222,128,0.25); }
  .status-tile.operational .status-dot { background: var(--bar-ok); }
  .status-tile.degraded_performance { border-color: rgba(250,204,21,0.3); }
  .status-tile.degraded_performance .status-dot { background: var(--bar-warn); }
  .status-tile.partial_outage { border-color: rgba(249,115,22,0.3); }
  .status-tile.partial_outage .status-dot { background: #f97316; }
  .status-tile.major_outage { border-color: rgba(248,113,113,0.35); }
  .status-tile.major_outage .status-dot { background: var(--bar-crit); }
  .status-tile.under_maintenance { border-color: rgba(96,165,250,0.3); }
  .status-tile.under_maintenance .status-dot { background: #60a5fa; }
  .status-name {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* リンク行 */
  .links-row {
    display: flex;
    justify-content: center;
    gap: 14px;
    flex-shrink: 0;
  }
  .detail-link {
    display: inline-block;
    font-size: 10px;
    color: var(--accent);
    text-decoration: none;
    padding: 1px 0;
    cursor: pointer;
  }
  .detail-link:hover { text-decoration: underline; }

  .footer {
    display: flex;
    justify-content: space-between;
    font-size: 9px;
    color: var(--fg-dim);
    padding: 1px 0 0;
    margin-top: auto;
  }

  .auth-banner {
    display: none;
    background: var(--err-bg);
    border: 1px solid var(--err-border);
    border-radius: 4px;
    padding: 5px 8px;
    font-size: 10px;
    line-height: 1.4;
    flex-shrink: 0;
  }
  .auth-banner.show { display: block; }
  .auth-banner strong { color: #fca5a5; }
  .auth-banner code {
    background: rgba(0,0,0,0.3);
    padding: 1px 4px;
    border-radius: 2px;
    font-size: 10px;
  }
  .auth-banner button {
    margin-top: 4px;
    padding: 3px 10px;
    font-size: 10px;
    background: rgba(255,255,255,0.12);
    color: var(--fg);
    border: 1px solid rgba(255,255,255,0.2);
    border-radius: 3px;
    cursor: pointer;
  }
  .auth-banner button:hover { background: rgba(255,255,255,0.2); }

  .status-banner {
    display: none;
    background: rgba(250,204,21,0.12);
    border: 1px solid rgba(250,204,21,0.4);
    border-radius: 4px;
    padding: 4px 7px;
    font-size: 10px;
    line-height: 1.35;
    flex-shrink: 0;
    cursor: pointer;
  }
  .status-banner.show { display: block; }
  .status-banner.major { background: rgba(249,115,22,0.15); border-color: rgba(249,115,22,0.45); }
  .status-banner.critical { background: rgba(248,113,113,0.15); border-color: rgba(248,113,113,0.45); }
  .status-banner .sb-title {
    font-weight: 600;
    color: #fcd34d;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .status-banner.major .sb-title { color: #fdba74; }
  .status-banner.critical .sb-title { color: #fca5a5; }
  .status-banner .sb-sub {
    color: var(--fg-dim);
    display: flex;
    justify-content: space-between;
    gap: 4px;
  }
  .status-banner .sb-sub .sb-more { flex-shrink: 0; }

  /* 設定パネル */
  .settings {
    display: none;
    flex: 1;
    padding: 6px 8px;
    flex-direction: column;
    gap: 4px;
    overflow: hidden;
    min-height: 0;
  }
  .settings-scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
    scrollbar-width: thin;
    padding-right: 2px;
  }
  .settings-scroll::-webkit-scrollbar { width: 6px; }
  .settings-scroll::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.18); border-radius: 3px; }
  .settings-footer {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex-shrink: 0;
  }
  .settings.active { display: flex; }
  .content.hidden { display: none; }

  .settings .group {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 5px 8px;
  }
  .settings .group-title {
    font-size: 10px;
    color: var(--fg-dim);
    margin-bottom: 3px;
    letter-spacing: 0.04em;
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
  .settings label + label { margin-top: 3px; }
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
  .settings .poll-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
  }
  .settings .poll-row + .poll-row { margin-top: 3px; }
  .settings .poll-label { font-size: 11px; }
  .settings .poll-input {
    width: 64px;
    padding: 2px 4px;
    font-size: 11px;
    font-family: inherit;
    color: var(--fg);
    background: rgba(255,255,255,0.06);
    border: 1px solid var(--border);
    border-radius: 3px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .settings .poll-note {
    font-size: 9px;
    color: var(--fg-dim);
    margin-top: 3px;
  }
</style>
</head>
<body>
  <div class="titlebar" id="titlebar">
    <div class="title" id="title-text">Claude / Codex モニター</div>
    <div class="title-actions">
      <button class="title-btn" id="btn-refresh" title="更新">⟳</button>
      <button class="title-btn" id="btn-settings" title="設定">⚙</button>
      <button class="title-btn" id="btn-close" title="閉じる">✕</button>
    </div>
  </div>

  <div class="content" id="main-view">
    <div class="provider-tabs">
      <button class="provider-tab active" id="provider-claude" type="button">Claude</button>
      <button class="provider-tab" id="provider-codex" type="button">Codex</button>
    </div>
    <div class="auth-banner" id="auth-banner">
      <strong id="auth-banner-title">認証エラー</strong>
      <div id="auth-banner-body"></div>
      <button id="auth-banner-action" type="button" style="display:none;">ログイン</button>
    </div>

    <div class="status-banner" id="status-banner" title="サービスステータスを開く">
      <div class="sb-title" id="status-banner-title"></div>
      <div class="sb-sub">
        <span id="status-banner-impact"></span>
        <span class="sb-more" id="status-banner-more"></span>
      </div>
    </div>

    <div class="row-bar">
      <div class="row-top">
        <span class="row-label" id="label-5h">5時間</span>
        <div class="bar"><div class="bar-fill" id="bar-5h" style="width:0%"></div></div>
        <span class="bar-pct" id="pct-5h">—</span>
      </div>
      <div class="bar-reset" id="reset-5h"></div>
    </div>

    <div class="row-bar">
      <div class="row-top">
        <span class="row-label" id="label-7d">7日</span>
        <div class="bar"><div class="bar-fill" id="bar-7d" style="width:0%"></div></div>
        <span class="bar-pct" id="pct-7d">—</span>
      </div>
      <div class="bar-reset" id="reset-7d"></div>
    </div>

    <div class="row-bar" id="overage-section" style="display:none">
      <div class="row-top">
        <span class="row-label">追加</span>
        <div class="bar" id="overage-bar" style="display:none">
          <div class="bar-fill" id="bar-overage" style="width:0%"></div>
        </div>
        <span id="overage-amount" class="bar-pct" style="flex:1;text-align:left;min-width:0"></span>
      </div>
      <div style="display:flex;justify-content:space-between">
        <span class="bar-reset" id="overage-limit"></span>
        <span class="bar-reset" id="overage-reset"></span>
      </div>
    </div>

    <div class="status-row">
      <div class="status-tile" id="status-svc-0">
        <span class="status-dot"></span>
        <span class="status-name">claude.ai</span>
      </div>
      <div class="status-tile" id="status-svc-1">
        <span class="status-dot"></span>
        <span class="status-name">Cowork</span>
      </div>
      <div class="status-tile" id="status-svc-2">
        <span class="status-dot"></span>
        <span class="status-name">Code</span>
      </div>
    </div>

    <div class="links-row">
      <span class="detail-link" id="detail-link">詳細を確認 →</span>
      <span class="detail-link" id="status-link">ステータス →</span>
    </div>

    <div class="footer">
      <span id="account-label"></span>
      <span id="updated">未更新</span>
    </div>
  </div>

  <div class="content overview-view hidden" id="overview-view">
    <div class="provider-card" data-provider="claude">
      <div class="provider-card-header"><span class="provider-card-name">Claude</span><span class="provider-card-plan" id="ov-claude-plan"></span></div>
      <div class="overview-message auth" id="ov-claude-auth"></div>
      <div class="overview-message incident" id="ov-claude-incident"></div>
      <div class="overview-window"><div class="row-top"><span class="row-label" id="ov-claude-label-5h">5時間</span><div class="bar"><div class="bar-fill" id="ov-claude-bar-5h"></div></div><span class="bar-pct" id="ov-claude-pct-5h">—</span></div><div class="bar-reset" id="ov-claude-reset-5h"></div></div>
      <div class="overview-window"><div class="row-top"><span class="row-label" id="ov-claude-label-7d">7日</span><div class="bar"><div class="bar-fill" id="ov-claude-bar-7d"></div></div><span class="bar-pct" id="ov-claude-pct-7d">—</span></div><div class="bar-reset" id="ov-claude-reset-7d"></div></div>
      <div class="overview-extra" id="ov-claude-extra"><div class="overview-extra-top"><span class="overview-extra-label" id="ov-claude-extra-label"></span><div class="bar" id="ov-claude-extra-bar" style="display:none"><div class="bar-fill" id="ov-claude-extra-bar-fill"></div></div><strong id="ov-claude-extra-value"></strong></div><div class="overview-extra-detail"><span id="ov-claude-extra-detail"></span><span id="ov-claude-extra-reset"></span></div></div>
      <div class="status-row overview-status" id="ov-claude-status"></div>
      <div class="overview-links"><span class="detail-link overview-usage-link" data-provider="claude">使用量 →</span><span class="detail-link overview-status-link" data-provider="claude">Status →</span></div>
      <div class="overview-account"><span id="ov-claude-account"></span><span id="ov-claude-updated">未更新</span></div>
    </div>
    <div class="provider-card" data-provider="codex">
      <div class="provider-card-header"><span class="provider-card-name">Codex</span><span class="provider-card-plan" id="ov-codex-plan"></span></div>
      <div class="overview-message auth" id="ov-codex-auth"></div>
      <div class="overview-message incident" id="ov-codex-incident"></div>
      <div class="overview-window"><div class="row-top"><span class="row-label" id="ov-codex-label-5h">5時間</span><div class="bar"><div class="bar-fill" id="ov-codex-bar-5h"></div></div><span class="bar-pct" id="ov-codex-pct-5h">—</span></div><div class="bar-reset" id="ov-codex-reset-5h"></div></div>
      <div class="overview-window"><div class="row-top"><span class="row-label" id="ov-codex-label-7d">7日</span><div class="bar"><div class="bar-fill" id="ov-codex-bar-7d"></div></div><span class="bar-pct" id="ov-codex-pct-7d">—</span></div><div class="bar-reset" id="ov-codex-reset-7d"></div></div>
      <div class="overview-extra" id="ov-codex-extra"><div class="overview-extra-top"><span class="overview-extra-label" id="ov-codex-extra-label"></span><div class="bar" id="ov-codex-extra-bar" style="display:none"><div class="bar-fill" id="ov-codex-extra-bar-fill"></div></div><strong id="ov-codex-extra-value"></strong></div><div class="overview-extra-detail"><span id="ov-codex-extra-detail"></span><span id="ov-codex-extra-reset"></span></div></div>
      <div class="status-row overview-status" id="ov-codex-status"></div>
      <div class="overview-links"><span class="detail-link overview-usage-link" data-provider="codex">使用量 →</span><span class="detail-link overview-status-link" data-provider="codex">Status →</span></div>
      <div class="overview-account"><span id="ov-codex-account"></span><span id="ov-codex-updated">未更新</span></div>
    </div>
  </div>

  <div class="settings" id="settings-view">
    <div class="settings-scroll">
      <div class="group">
        <div class="account" id="account-info">取得中…</div>
      </div>

      <div class="group">
        <div class="row" style="gap:12px">
          <label><input type="checkbox" id="topmost"> 最前面</label>
          <label><input type="checkbox" id="transparent"> 半透明</label>
        </div>
      </div>

      <div class="group">
        <div class="group-title">表示するサービス</div>
        <div class="row" style="gap:12px">
          <label><input type="checkbox" id="provider-enable-claude"> Claude</label>
          <label><input type="checkbox" id="provider-enable-codex"> Codex</label>
        </div>
        <div class="poll-note" id="provider-selection-note">1つ以上選択してください。</div>
      </div>

      <div class="group">
        <div class="group-title">表示レイアウト</div>
        <label><input type="radio" name="layout-mode" value="tabs" id="layout-tabs"> タブで切り替え</label>
        <label><input type="radio" name="layout-mode" value="overview" id="layout-overview"> 1画面に並べて表示</label>
      </div>

      <div class="group">
        <div class="group-title">通知</div>
        <label><input type="checkbox" id="notify-usage"> 使用量 60% / 80%</label>
        <label><input type="checkbox" id="notify-overage"> 追加使用量 60% / 80%</label>
        <label><input type="checkbox" id="notify-status"> Claude / OpenAI Status 障害検知</label>
      </div>

      <div class="group">
        <div class="group-title">追加使用量の表示形式</div>
        <label><input type="radio" name="overage-tip-fmt" value="dollar" id="overage-tip-dollar"> ドル表示（$4.69）</label>
        <label><input type="radio" name="overage-tip-fmt" value="percent" id="overage-tip-percent"> パーセント表示（9%）</label>
      </div>

      <div class="group">
        <div class="group-title">5時間枠のリセット表示</div>
        <label><input type="radio" name="reset-time-fmt" value="datetime" id="reset-time-fmt-datetime"> 日時表示</label>
        <label><input type="radio" name="reset-time-fmt" value="relative" id="reset-time-fmt-relative"> 残り時間表示（あと3時間20分）</label>
      </div>

      <div class="group">
        <div class="group-title">7日使用量アイコンの配色ペース</div>
        <label><input type="radio" name="tray-split" value="7" id="tray-split-7"> 7日按分（N/7日で警告色）</label>
        <label><input type="radio" name="tray-split" value="5" id="tray-split-5"> 5日按分（N/5日で警告色）</label>
        <label><input type="radio" name="tray-split" value="0" id="tray-split-0"> 分割なし（固定 60% / 80%）</label>
      </div>

      <div class="group">
        <div class="group-title">ポーリング間隔（秒）</div>
        <div class="poll-row">
          <span class="poll-label">使用量取得</span>
          <input type="number" id="poll-usage" class="poll-input" min="60" max="3600" step="10">
        </div>
        <div class="poll-row">
          <span class="poll-label">障害監視</span>
          <input type="number" id="poll-status" class="poll-input" min="60" max="3600" step="10">
        </div>
        <div class="poll-note">60〜3600秒。範囲外は自動調整されます。</div>
      </div>
    </div>

    <div class="settings-footer">
      <div class="row" style="justify-content: flex-end; gap: 6px;">
        <button id="btn-cancel" style="background: rgba(255,255,255,0.1); color: var(--fg);">戻る</button>
        <button id="btn-save">保存</button>
      </div>
    </div>
  </div>

<script>
const numFmt = new Intl.NumberFormat('ja-JP');

function formatRelative(iso) {
  // Go の time.Time ゼロ値は "0001-01-01T00:00:00Z" にマーシャルされるため、
  // 「一度も成功していない」を未取得として表示する。
  if (!iso || iso.startsWith('0001-')) return '未取得';
  const t = new Date(iso);
  const diff = (Date.now() - t.getTime()) / 1000;
  if (diff < 5) return 'たった今';
  if (diff < 60) return Math.floor(diff) + '秒前';
  if (diff < 3600) return Math.floor(diff/60) + '分前';
  if (diff < 86400) return Math.floor(diff/3600) + '時間前';
  return Math.floor(diff/86400) + '日前';
}

const JP_WEEKDAYS = ['日','月','火','水','木','金','土'];

function formatResetDateTime(iso) {
  if (!iso) return '';
  const t = new Date(iso);
  if (isNaN(t.getTime())) return '';
  const mo = t.getMonth() + 1;
  const d = t.getDate();
  const w = JP_WEEKDAYS[t.getDay()];
  const hh = String(t.getHours()).padStart(2, '0');
  const mm = String(t.getMinutes()).padStart(2, '0');
  return '↻ ' + mo + '月' + d + '日（' + w + '）' + hh + ':' + mm;
}

// あと◯時間◯分でリセット、という相対表示。過去時刻・不正日時はガードする。
function formatResetRelative(iso) {
  if (!iso) return '';
  const t = new Date(iso);
  if (isNaN(t.getTime())) return '';
  let diffMin = Math.round((t.getTime() - Date.now()) / 60000);
  if (diffMin <= 0) return '↻ まもなくリセット';
  const h = Math.floor(diffMin / 60);
  const m = diffMin % 60;
  if (h <= 0) return '↻ あと' + m + '分';
  return '↻ あと' + h + '時間' + m + '分';
}

function applyWindow(prefix, win) {
  const pctEl = document.getElementById('pct-' + prefix);
  const barEl = document.getElementById('bar-' + prefix);
  const resetEl = document.getElementById('reset-' + prefix);
	const labelEl = document.getElementById('label-' + prefix);
	labelEl.textContent = (win && win.label) || (prefix === '5h' ? '5時間' : '7日');
  if (!win || typeof win.utilization !== 'number' || (win.utilization === 0 && !win.resetsAt)) {
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
  // 0-60% 緑 (default) / 61-80% 黄 (warn) / 81-100% 赤 (crit)
  if (pct >= 81) barEl.classList.add('crit');
  else if (pct >= 61) barEl.classList.add('warn');
  resetEl.textContent = (prefix === '5h' && resetTimeFormat === 'relative')
    ? formatResetRelative(win.resetsAt)
    : formatResetDateTime(win.resetsAt);
}

function formatResetDate(iso) {
  if (!iso) return '';
  const t = new Date(iso);
  if (isNaN(t.getTime())) return '';
  return '↻ ' + (t.getMonth() + 1) + '月' + t.getDate() + '日';
}

function applyOverage(overage, creditBalance) {
  const section = document.getElementById('overage-section');
  if (typeof creditBalance === 'number') {
    section.style.display = '';
    document.getElementById('overage-bar').style.display = 'none';
    const amtEl = document.getElementById('overage-amount');
    amtEl.style.flex = '1';
    amtEl.style.textAlign = 'left';
    amtEl.textContent = 'クレジット残高';
    document.getElementById('overage-limit').textContent = '$' + creditBalance.toFixed(2);
    document.getElementById('overage-reset').textContent = '';
    return;
  }
  if (!overage || typeof overage.amountUsed !== 'number') {
    section.style.display = 'none';
    return;
  }
  // amountUsed=0 かつ spendingLimit 未設定（無制限）は実質無効とみなして非表示
  if (overage.amountUsed === 0 && overage.spendingLimit == null) {
    section.style.display = 'none';
    return;
  }
  section.style.display = '';
  const amtEl = document.getElementById('overage-amount');
  const limitEl = document.getElementById('overage-limit');
  const resetEl = document.getElementById('overage-reset');
  const barContainer = document.getElementById('overage-bar');
  const barFill = document.getElementById('bar-overage');

  if (overage.spendingLimit != null && overage.spendingLimit > 0) {
    // 上限あり: プログレスバーを表示
    barContainer.style.display = '';
    amtEl.style.flex = '';
    amtEl.style.textAlign = 'right';
    amtEl.style.minWidth = '';
    const pctDisplay = Math.round((overage.amountUsed / overage.spendingLimit) * 100);
    amtEl.textContent = pctDisplay + '%';
    const pct = Math.min(100, (overage.amountUsed / overage.spendingLimit) * 100);
    barFill.style.width = pct + '%';
    barFill.className = 'bar-fill' + (pct >= 81 ? ' crit' : pct >= 61 ? ' warn' : '');
    limitEl.textContent = '$' + overage.amountUsed.toFixed(2) + ' / $' + overage.spendingLimit.toFixed(2);
  } else {
    // 無制限: バーなしのテキスト表示
    barContainer.style.display = 'none';
    amtEl.style.flex = '1';
    amtEl.style.textAlign = 'left';
    amtEl.style.minWidth = '';
    amtEl.textContent = '$' + overage.amountUsed.toFixed(2) + ' 使用中';
    limitEl.textContent = '上限: 無制限';
  }
  resetEl.textContent = formatResetDate(overage.resetsAt);
}

let lastUpdated = null;
let lastSnapshot = null;
let lastStatusSnap = null;
let allUsageSnapshots = null;
let allStatusSnapshots = null;
let activeProvider = 'claude';
let layoutMode = 'tabs';
let resetTimeFormat = 'datetime';
let claudeEnabled = true;
let codexEnabled = true;

function overviewId(provider, suffix) {
  return document.getElementById('ov-' + provider + '-' + suffix);
}

function renderOverviewWindow(provider, slot, win) {
  const label = overviewId(provider, 'label-' + slot);
  const pctEl = overviewId(provider, 'pct-' + slot);
  const bar = overviewId(provider, 'bar-' + slot);
  const reset = overviewId(provider, 'reset-' + slot);
  label.textContent = (win && win.label) || (slot === '5h' ? '5時間' : '7日');
  if (!win || typeof win.utilization !== 'number' || (win.utilization === 0 && !win.resetsAt)) {
    pctEl.textContent = '—';
    bar.style.width = '0%';
    bar.className = 'bar-fill';
    reset.textContent = '';
    return;
  }
  const pct = Math.max(0, Math.min(100, Math.round(win.utilization)));
  pctEl.textContent = pct + '%';
  bar.style.width = pct + '%';
  bar.className = 'bar-fill' + (pct >= 81 ? ' crit' : pct >= 61 ? ' warn' : '');
  reset.textContent = (slot === '5h' && resetTimeFormat === 'relative')
    ? formatResetRelative(win.resetsAt)
    : formatResetDateTime(win.resetsAt);
}

function renderOverviewStatus(provider, services) {
  const row = overviewId(provider, 'status');
  row.replaceChildren();
  for (let i = 0; i < 3; i++) {
    const svc = services && services[i];
    const tile = document.createElement('div');
    tile.className = 'status-tile' + (svc && svc.status && svc.status !== 'unknown' ? ' ' + svc.status : '');
    const dot = document.createElement('span');
    dot.className = 'status-dot';
    const name = document.createElement('span');
    name.className = 'status-name';
    name.textContent = svc ? svc.name.replace('Codex in ChatGPT Desktop', 'Desktop').replace('Codex Web', 'Web').replace('Claude ', '') : '—';
    tile.append(dot, name);
    row.appendChild(tile);
  }
}

function renderOverviewProvider(provider) {
  const snap = (allUsageSnapshots && allUsageSnapshots[provider]) || {};
  const status = (allStatusSnapshots && allStatusSnapshots[provider]) || {};
  overviewId(provider, 'plan').textContent = snap.subscriptionType || '';
  const account = snap.displayName || snap.email || '';
  overviewId(provider, 'account').textContent = account || '未ログイン';
  overviewId(provider, 'updated').textContent = snap.updatedAt ? formatRelative(snap.updatedAt) : '未更新';
  renderOverviewWindow(provider, '5h', snap.fiveHour);
  renderOverviewWindow(provider, '7d', snap.sevenDay);

  const auth = overviewId(provider, 'auth');
  auth.classList.remove('show');
  if (snap.authState && snap.authState !== 'ok') {
    auth.textContent = snap.authState === 'needs_login' ? '未ログイン（クリックでログイン）' : snap.authState === 'init' ? '取得中…' : '取得失敗';
    auth.title = snap.lastError || '';
    auth.classList.add('show');
  }

  const extra = overviewId(provider, 'extra');
  const extraLabel = overviewId(provider, 'extra-label');
  const extraValue = overviewId(provider, 'extra-value');
  const extraBar = overviewId(provider, 'extra-bar');
  const extraBarFill = overviewId(provider, 'extra-bar-fill');
  const extraDetail = overviewId(provider, 'extra-detail');
  const extraReset = overviewId(provider, 'extra-reset');
  extra.classList.remove('show');
  extraBar.style.display = 'none';
  extraBarFill.style.width = '0%';
  extraBarFill.className = 'bar-fill';
  extraDetail.textContent = '';
  extraReset.textContent = '';
  if (typeof snap.creditBalance === 'number') {
    extraLabel.textContent = '残高';
    extraValue.textContent = '$' + snap.creditBalance.toFixed(2);
    extra.classList.add('show');
  } else if (snap.overage && typeof snap.overage.amountUsed === 'number' &&
      !(snap.overage.amountUsed === 0 && snap.overage.spendingLimit == null)) {
    extraLabel.textContent = '追加';
    if (snap.overage.spendingLimit != null && snap.overage.spendingLimit > 0) {
      const pctDisplay = Math.round((snap.overage.amountUsed / snap.overage.spendingLimit) * 100);
      const pct = Math.max(0, Math.min(100, (snap.overage.amountUsed / snap.overage.spendingLimit) * 100));
      extraBar.style.display = '';
      extraBarFill.style.width = pct + '%';
      extraBarFill.className = 'bar-fill' + (pct >= 81 ? ' crit' : pct >= 61 ? ' warn' : '');
      extraValue.textContent = pctDisplay + '%';
      extraDetail.textContent = '$' + snap.overage.amountUsed.toFixed(2) + ' / $' + snap.overage.spendingLimit.toFixed(2);
    } else {
      extraValue.textContent = '$' + snap.overage.amountUsed.toFixed(2);
      extraDetail.textContent = '上限: 無制限';
    }
    extraReset.textContent = formatResetDate(snap.overage.resetsAt);
    extra.classList.add('show');
  }

  const incident = overviewId(provider, 'incident');
  incident.classList.remove('show');
  const incidents = status.incidents || [];
  if (snap.authState === 'ok' && incidents.length) {
    const weight = {critical:4, major:3, minor:2, maintenance:1, none:0};
    let head = incidents[0];
    for (const item of incidents) if ((weight[item.impact] || 0) > (weight[head.impact] || 0)) head = item;
    incident.textContent = head.name || '進行中のインシデント';
    incident.title = incident.textContent;
    incident.classList.add('show');
  }
  renderOverviewStatus(provider, status.services);
}

function renderOverview() {
  if (claudeEnabled) renderOverviewProvider('claude');
  if (codexEnabled) renderOverviewProvider('codex');
}

function renderAuthBanner(snap) {
  const banner = document.getElementById('auth-banner');
  const titleEl = document.getElementById('auth-banner-title');
  const bodyEl = document.getElementById('auth-banner-body');
  const btn = document.getElementById('auth-banner-action');
  if (!snap || snap.authState === 'ok' || snap.authState === 'init') {
    banner.classList.remove('show');
    return;
  }
  banner.classList.add('show');
  btn.style.display = 'none';
  switch (snap.authState) {
    case 'needs_login':
      titleEl.textContent = '未ログイン';
	  bodyEl.textContent = activeProvider === 'codex'
	    ? 'ChatGPT にサインインしてください。'
	    : 'Claude にサインインしてください。';
	  btn.style.display = 'inline-block';
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

document.getElementById('auth-banner-action').addEventListener('click', async () => {
  try {
    await fetch('/api/relogin?provider=' + activeProvider, {method: 'POST'});
  } catch (e) {}
});

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

function applySnapshot(snap) {
  const prevAuthState = lastSnapshot && lastSnapshot.authState;
  lastSnapshot = snap;
  applyWindow('5h', snap.fiveHour);
  applyWindow('7d', snap.sevenDay);
  applyOverage(snap.overage, snap.creditBalance);
  renderAuthBanner(snap);
  renderAccount(snap);
  lastUpdated = snap.updatedAt;
  updateFooter();
  // 認証状態が変化したら status-banner の表示可否を再評価する
  // （ログイン完了時に即座に障害バナーを出す / ログアウト時に隠す）
  if (snap.authState !== prevAuthState && lastStatusSnap) {
    renderStatusBanner(lastStatusSnap);
  }
}

async function fetchUsage() {
  try {
    const res = await fetch('/api/usage');
	allUsageSnapshots = await res.json();
	applySnapshot(allUsageSnapshots[activeProvider] || {});
	renderOverview();
  } catch (e) {
    document.getElementById('updated').textContent = '取得エラー';
  }
}

function updateFooter() {
  if (lastUpdated) {
    document.getElementById('updated').textContent = formatRelative(lastUpdated);
  } else {
    document.getElementById('updated').textContent = '未更新';
  }
  if (lastSnapshot) {
    applyWindow('5h', lastSnapshot.fiveHour);
    applyWindow('7d', lastSnapshot.sevenDay);
  }
	renderOverview();
}

function renderStatusTiles(services) {
	for (let i = 0; i < 3; i++) {
	const tile = document.getElementById('status-svc-' + i);
    if (!tile) continue;
	const svc = services && services[i];
	tile.querySelector('.status-name').textContent = svc ? svc.name.replace('Codex in ChatGPT Desktop', 'Desktop').replace('Codex Web', 'Web') : '—';
    // Remove all status classes then apply new one
    tile.classList.remove(
      'operational','degraded_performance','partial_outage','major_outage','under_maintenance'
    );
	if (svc && svc.status && svc.status !== 'unknown') {
      tile.classList.add(svc.status);
    }
  }
}

const INCIDENT_IMPACT_LABEL = {
  minor: '軽微な障害',
  major: '重大な障害',
  critical: '致命的な障害',
  maintenance: 'メンテナンス中',
  none: '',
};

function renderStatusBanner(snap) {
  const banner = document.getElementById('status-banner');
  const titleEl = document.getElementById('status-banner-title');
  const impactEl = document.getElementById('status-banner-impact');
  const moreEl = document.getElementById('status-banner-more');
  if (!banner) return;
  const incidents = (snap && snap.incidents) || [];
  // 未認証中（needs_login / network_error 等）は auth-banner と同時表示すると
  // ウィンドウ高さ 260px に収まらず下部が切れるため非表示にする。
  // authState が 'init'（初期化中）または不明な場合は判定を保留し表示しない。
  const authOk = lastSnapshot && lastSnapshot.authState === 'ok';
  if (incidents.length === 0 || !authOk) {
    banner.classList.remove('show', 'major', 'critical');
    return;
  }
  // Pick the worst incident by impact.
  const weight = { critical: 4, major: 3, minor: 2, maintenance: 1, none: 0 };
  let head = incidents[0];
  for (const inc of incidents) {
    if ((weight[inc.impact] || 0) > (weight[head.impact] || 0)) head = inc;
  }
  titleEl.textContent = head.name || '進行中のインシデント';
  impactEl.textContent = INCIDENT_IMPACT_LABEL[head.impact] || head.impact || '';
  moreEl.textContent = incidents.length > 1 ? '他' + (incidents.length - 1) + '件 →' : '詳細 →';
  banner.classList.remove('major', 'critical');
  if (head.impact === 'major') banner.classList.add('major');
  else if (head.impact === 'critical') banner.classList.add('critical');
  banner.classList.add('show');
}

async function fetchStatus() {
  try {
    const res = await fetch('/api/status');
	allStatusSnapshots = await res.json();
	lastStatusSnap = allStatusSnapshots[activeProvider] || {};
    renderStatusTiles(lastStatusSnap.services);
    renderStatusBanner(lastStatusSnap);
	renderOverview();
  } catch (e) {
    // silently fail — tiles stay gray (unknown)
  }
}

async function switchProvider(provider, persist = true) {
	if (provider !== 'claude' && provider !== 'codex') return;
	if ((provider === 'claude' && !claudeEnabled) || (provider === 'codex' && !codexEnabled)) return;
	activeProvider = provider;
	document.getElementById('provider-claude').classList.toggle('active', provider === 'claude');
	document.getElementById('provider-codex').classList.toggle('active', provider === 'codex');
	if (allUsageSnapshots) applySnapshot(allUsageSnapshots[provider] || {});
	if (allStatusSnapshots) {
	  lastStatusSnap = allStatusSnapshots[provider] || {};
	  renderStatusTiles(lastStatusSnap.services);
	  renderStatusBanner(lastStatusSnap);
	}
	if (persist) {
	  try { await fetch('/api/provider', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({provider})}); } catch(e) {}
	}
}

document.getElementById('provider-claude').addEventListener('click', () => switchProvider('claude'));
document.getElementById('provider-codex').addEventListener('click', () => switchProvider('codex'));

// --- タイトルバーのボタン ---
document.getElementById('btn-close').addEventListener('click', () => {
  fetch('/api/close', { method: 'GET' });
});
document.getElementById('btn-refresh').addEventListener('click', async () => {
  const btn = document.getElementById('btn-refresh');
  btn.disabled = true;
  btn.style.opacity = '0.5';
  try {
    await fetch('/api/refresh');
    await fetchUsage();
    fetchStatus();
  } catch (e) {
    document.getElementById('updated').textContent = '取得エラー';
  } finally {
    btn.disabled = false;
    btn.style.opacity = '';
  }
});
document.getElementById('detail-link').addEventListener('click', (e) => {
  e.preventDefault();
  fetch('/api/open-usage?provider=' + activeProvider);
});
document.getElementById('status-link').addEventListener('click', (e) => {
  e.preventDefault();
  fetch('/api/open-status?provider=' + activeProvider);
});
document.getElementById('status-banner').addEventListener('click', () => {
  fetch('/api/open-status?provider=' + activeProvider);
});
document.querySelectorAll('.overview-usage-link').forEach(el => el.addEventListener('click', () => {
  fetch('/api/open-usage?provider=' + el.dataset.provider);
}));
document.querySelectorAll('.overview-status-link').forEach(el => el.addEventListener('click', () => {
  fetch('/api/open-status?provider=' + el.dataset.provider);
}));
document.querySelectorAll('.overview-message.auth').forEach(el => el.addEventListener('click', () => {
  const provider = el.id.includes('codex') ? 'codex' : 'claude';
  const snap = allUsageSnapshots && allUsageSnapshots[provider];
  if (snap && snap.authState === 'needs_login') {
    fetch('/api/relogin?provider=' + provider, {method: 'POST'});
  }
}));

// --- 設定パネル ---
const mainView = document.getElementById('main-view');
const overviewView = document.getElementById('overview-view');
const settingsView = document.getElementById('settings-view');

function applyProviderVisibility(s) {
  claudeEnabled = s.claudeEnabled !== false;
  codexEnabled = s.codexEnabled !== false;
  if (!claudeEnabled && !codexEnabled) claudeEnabled = true;
  document.getElementById('provider-claude').classList.toggle('provider-hidden', !claudeEnabled);
  document.getElementById('provider-codex').classList.toggle('provider-hidden', !codexEnabled);
  document.querySelector('.provider-card[data-provider="claude"]').classList.toggle('provider-hidden', !claudeEnabled);
  document.querySelector('.provider-card[data-provider="codex"]').classList.toggle('provider-hidden', !codexEnabled);
  document.querySelector('.provider-tabs').style.display = claudeEnabled && codexEnabled ? 'flex' : 'none';
  if ((activeProvider === 'claude' && !claudeEnabled) || (activeProvider === 'codex' && !codexEnabled)) {
    activeProvider = claudeEnabled ? 'claude' : 'codex';
  }
}

function applyLayoutModeUI(mode) {
  layoutMode = mode === 'overview' ? 'overview' : 'tabs';
  if (settingsView.classList.contains('active')) return;
  mainView.classList.toggle('hidden', layoutMode !== 'tabs');
  overviewView.classList.toggle('hidden', layoutMode !== 'overview');
  if (layoutMode === 'overview') renderOverview();
}

async function openSettings() {
  const res = await fetch('/api/settings');
  const s = await res.json();
	applyProviderVisibility(s);
	await switchProvider(s.activeProvider || activeProvider, false);
	document.getElementById('provider-enable-claude').checked = claudeEnabled;
	document.getElementById('provider-enable-codex').checked = codexEnabled;
	const lm = s.layoutMode === 'overview' ? 'overview' : 'tabs';
	document.getElementById('layout-tabs').checked = lm === 'tabs';
	document.getElementById('layout-overview').checked = lm === 'overview';
  document.getElementById('topmost').checked = !!s.topmost;
  document.getElementById('transparent').checked = !!s.transparent;
  document.getElementById('notify-usage').checked = !!s.notifyUsage;
  document.getElementById('notify-overage').checked = !!s.notifyOverage;
  document.getElementById('notify-status').checked = !!s.notifyStatus;
  const fmt = s.overageTipFormat || 'dollar';
  document.getElementById('overage-tip-dollar').checked = fmt === 'dollar';
  document.getElementById('overage-tip-percent').checked = fmt === 'percent';
  const rtf = s.resetTimeFormat === 'relative' ? 'relative' : 'datetime';
  document.getElementById('reset-time-fmt-datetime').checked = rtf === 'datetime';
  document.getElementById('reset-time-fmt-relative').checked = rtf === 'relative';
  resetTimeFormat = rtf;
  const traySplit = s.traySplitDays ?? 7;
  document.getElementById('tray-split-7').checked = traySplit === 7;
  document.getElementById('tray-split-5').checked = traySplit === 5;
  document.getElementById('tray-split-0').checked = traySplit === 0;
  document.getElementById('poll-usage').value = s.usagePollSeconds || 300;
  document.getElementById('poll-status').value = s.statusPollSeconds || 300;
  mainView.classList.add('hidden');
	overviewView.classList.add('hidden');
  settingsView.classList.add('active');
}
function closeSettings() {
  settingsView.classList.remove('active');
	applyLayoutModeUI(layoutMode);
}
document.getElementById('btn-settings').addEventListener('click', openSettings);
document.getElementById('btn-cancel').addEventListener('click', closeSettings);
// clampPoll は秒値を [60,3600] に収める（サーバ側と同じ規則の UX 用フロント検証）。
function clampPoll(v) {
  v = parseInt(v, 10);
  if (isNaN(v) || v < 60) return 60;
  if (v > 3600) return 3600;
  return v;
}
document.getElementById('btn-save').addEventListener('click', async () => {
  const enableClaude = document.getElementById('provider-enable-claude').checked;
  const enableCodex = document.getElementById('provider-enable-codex').checked;
  const providerNote = document.getElementById('provider-selection-note');
  if (!enableClaude && !enableCodex) {
    providerNote.textContent = 'Claude または Codex を1つ以上選択してください。';
    providerNote.style.color = '#fca5a5';
    return;
  }
  providerNote.textContent = '1つ以上選択してください。';
  providerNote.style.color = '';
  const payload = {
    topmost: document.getElementById('topmost').checked,
    transparent: document.getElementById('transparent').checked,
    notifyUsage: document.getElementById('notify-usage').checked,
    notifyOverage: document.getElementById('notify-overage').checked,
    notifyStatus: document.getElementById('notify-status').checked,
	layoutMode: document.querySelector('input[name="layout-mode"]:checked')?.value || 'tabs',
    claudeEnabled: enableClaude,
    codexEnabled: enableCodex,
    overageTipFormat: document.querySelector('input[name="overage-tip-fmt"]:checked')?.value || 'dollar',
    resetTimeFormat: document.querySelector('input[name="reset-time-fmt"]:checked')?.value || 'datetime',
    traySplitDays: parseInt(document.querySelector('input[name="tray-split"]:checked')?.value ?? '7', 10),
    usagePollSeconds: clampPoll(document.getElementById('poll-usage').value),
    statusPollSeconds: clampPoll(document.getElementById('poll-status').value),
  };
  let applied = payload;
  try {
    const res = await fetch('/api/setoption', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload),
    });
    // 戻り値（サーバでクランプ済）を正として障害ポーリングを張り直す。
    applied = await res.json();
  } catch (e) {}
  resetTimeFormat = applied.resetTimeFormat === 'relative' ? 'relative' : 'datetime';
	applyProviderVisibility(applied);
	await switchProvider(applied.activeProvider || activeProvider, false);
	applyLayoutModeUI(applied.layoutMode);
  startStatusPolling(applied.statusPollSeconds || 300);
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

// 起動直後は authState=="init" の空スナップショットを返すので短間隔でポーリングし、
// 初回取得が済んだら通常の 60 秒間隔に戻す。
function schedulePoll() {
  const fast = !lastSnapshot || !lastSnapshot.authState || lastSnapshot.authState === 'init';
  setTimeout(() => {
    fetchUsage().finally(schedulePoll);
  }, fast ? 2000 : 60000);
}
// 障害監視ポーリングは設定値で間隔可変。clearInterval で張り直せるよう ID を保持する。
let statusTimer = null;
function startStatusPolling(sec) {
  if (statusTimer) clearInterval(statusTimer);
  statusTimer = setInterval(fetchStatus, (sec || 300) * 1000);
}
async function initStatusPolling() {
  let sec = 300;
  try {
    const s = await (await fetch('/api/settings')).json();
    if (s.statusPollSeconds) sec = s.statusPollSeconds;
    resetTimeFormat = s.resetTimeFormat === 'relative' ? 'relative' : 'datetime';
	applyProviderVisibility(s);
	await switchProvider(s.activeProvider || activeProvider, false);
	applyLayoutModeUI(s.layoutMode);
  } catch (e) {}
  startStatusPolling(sec);
}

fetchUsage().finally(schedulePoll);
fetchStatus();
initStatusPolling();
setInterval(updateFooter, 5000);
</script>
</body>
</html>`

