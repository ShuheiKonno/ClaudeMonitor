package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// notifyLogPath は診断ログの出力先。main で %LOCALAPPDATA%/ClaudeMonitor/notify.log にセットされる。
var (
	notifyLogPath string
	notifyLogMu   sync.Mutex
)

// notifyLog は通知パイプラインの入口/分岐/発火を append-only でファイルに記録する。
// GUI アプリは stderr が見えないためファイル経由で観測できるようにする。
// 失敗時は黙って捨てる（通知本来の動作を妨げない）。
func notifyLog(format string, args ...any) {
	if notifyLogPath == "" {
		return
	}
	notifyLogMu.Lock()
	defer notifyLogMu.Unlock()
	f, err := os.OpenFile(notifyLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s "+format+"\n", append([]any{time.Now().Format("2006-01-02 15:04:05")}, args...)...)
}

// 5 時間使用率の通知閾値。
const (
	notify5h60Threshold = 60.0
	notify5h80Threshold = 80.0
)

var (
	notifyMu sync.Mutex

	// 5h 使用率の通知済みフラグ。FiveHour.ResetsAt が 5h+ 飛ぶか pct=0 でリセット。
	// アプリ再起動を跨いで重複通知を防ぐため、起動時に config から復元される。
	notify5hResetsAt      time.Time
	notified5h60          bool
	notified5h80          bool
	overageResetsAt       time.Time
	notifiedOverage60     bool
	notifiedOverage80     bool
	codexNotify5hResetsAt time.Time
	codexNotified5h60     bool
	codexNotified5h80     bool

	// 通知済みインシデント ID。解決した ID は次回フェッチで削除する。
	notifiedIncidents            = map[string]bool{}
	notifyStatusInitialized      bool
	notifiedCodexIncidents       = map[string]bool{}
	notifyCodexStatusInitialized bool

	// テスト時に差し替えるための関数フック。本番は Win32 バルーン通知を呼ぶ。
	balloonFn = showBalloonNotification
)

// loadNotifyState は config から永続化された通知状態を読み込む。
// main が loadConfig() の後に呼ぶ。
func loadNotifyState() {
	cfg := snapshotConfig()
	notifyMu.Lock()
	defer notifyMu.Unlock()
	notify5hResetsAt = cfg.Notify5hResetsAt
	notified5h60 = cfg.Notified5h60
	notified5h80 = cfg.Notified5h80
	overageResetsAt = cfg.OverageResetsAt
	notifiedOverage60 = cfg.NotifiedOverage60
	notifiedOverage80 = cfg.NotifiedOverage80
	codexNotify5hResetsAt = cfg.CodexNotify5hResetsAt
	codexNotified5h60 = cfg.CodexNotified5h60
	codexNotified5h80 = cfg.CodexNotified5h80
}

// persistNotifyState は notifyMu を保持した呼び出し元から呼ばれる前提。
// 使用量通知のフラグ変化を config に書き出して再起動越しに維持する。
func persistNotifyState() {
	resetsAt := notify5hResetsAt
	n60 := notified5h60
	n80 := notified5h80
	overageAt := overageResetsAt
	overage60 := notifiedOverage60
	overage80 := notifiedOverage80
	codexAt := codexNotify5hResetsAt
	codex60 := codexNotified5h60
	codex80 := codexNotified5h80
	mutateConfig(func(c *Config) {
		c.Notify5hResetsAt = resetsAt
		c.Notified5h60 = n60
		c.Notified5h80 = n80
		c.OverageResetsAt = overageAt
		c.NotifiedOverage60 = overage60
		c.NotifiedOverage80 = overage80
		c.CodexNotify5hResetsAt = codexAt
		c.CodexNotified5h60 = codex60
		c.CodexNotified5h80 = codex80
	})
}

// handleCodexUsageNotification は Claude と同じ 60% / 80% しきい値を
// Codex の短期ウィンドウ（短期枠が無いプランでは長期枠）へ適用する。
// 状態はプロバイダー別に保持する。
func handleCodexUsageNotification(snap UsageSnapshot) {
	if snap.AuthState != "ok" {
		return
	}
	cfg := snapshotConfig()
	if !cfg.NotifyUsage || !providerEnabled(cfg, "codex") {
		return
	}
	notifyMu.Lock()
	defer notifyMu.Unlock()
	window := snap.FiveHour
	if window.Label == "" {
		window = snap.SevenDay
	}
	if window.Label == "" {
		return
	}
	var resetsAt time.Time
	if window.ResetsAt != nil {
		resetsAt = *window.ResetsAt
	}
	pct := window.Utilization
	stateChanged := false
	windowReset := resetsAt.Sub(codexNotify5hResetsAt) >= 5*time.Hour || pct == 0
	if !resetsAt.Equal(codexNotify5hResetsAt) {
		codexNotify5hResetsAt = resetsAt
		stateChanged = true
	}
	if windowReset && (codexNotified5h60 || codexNotified5h80) {
		codexNotified5h60 = false
		codexNotified5h80 = false
		stateChanged = true
	}
	if pct >= notify5h80Threshold && !codexNotified5h80 {
		codexNotified5h80, codexNotified5h60 = true, true
		balloonFn("Codex モニター — "+window.Label+"使用量 80%", fmt.Sprintf("現在 %d%% に到達しました。残量に注意してください。", clampPct(pct)), NIIF_WARNING)
		persistNotifyState()
		return
	}
	if pct >= notify5h60Threshold && !codexNotified5h60 {
		codexNotified5h60 = true
		balloonFn("Codex モニター — "+window.Label+"使用量 60%", fmt.Sprintf("現在 %d%% に到達しました。", clampPct(pct)), NIIF_INFO)
		persistNotifyState()
		return
	}
	if stateChanged {
		persistNotifyState()
	}
}

// handleUsageNotification は使用率スナップショットを受け取り、
// 5h 使用量が 60% / 80% を超えたタイミングで一度だけバルーン通知を出す。
// 通知済みフラグは config に永続化されるため、再起動を跨いでも重複通知しない。
// FiveHour.ResetsAt が 5h+ 飛んだ／pct が 0 に戻った時にフラグをクリアする。
func handleUsageNotification(snap UsageSnapshot) {
	if snap.AuthState != "ok" {
		notifyLog("usage skip auth=%s", snap.AuthState)
		return
	}
	cfg := snapshotConfig()
	if !cfg.NotifyUsage || !providerEnabled(cfg, "claude") {
		notifyLog("usage skip cfg.NotifyUsage=false")
		return
	}

	notifyMu.Lock()
	defer notifyMu.Unlock()

	var resetsAt time.Time
	if snap.FiveHour.ResetsAt != nil {
		resetsAt = *snap.FiveHour.ResetsAt
	}
	pct := snap.FiveHour.Utilization

	// 5h ローリングウィンドウは小幅に resetsAt がずれることがあるため、
	// 5 時間以上未来へジャンプした場合（= 真にウィンドウがリセットされた）
	// または利用率が 0% に戻った場合のみフラグをクリアする。
	stateChanged := false
	windowReset := resetsAt.Sub(notify5hResetsAt) >= 5*time.Hour || pct == 0
	if !resetsAt.Equal(notify5hResetsAt) {
		notify5hResetsAt = resetsAt
		stateChanged = true
	}
	if windowReset && (notified5h60 || notified5h80) {
		notified5h60 = false
		notified5h80 = false
		stateChanged = true
	}

	notifyLog("usage enter pct=%.2f resetsAt=%s reset=%v n60=%v n80=%v",
		pct, resetsAt.Format(time.RFC3339), windowReset, notified5h60, notified5h80)

	if pct >= notify5h80Threshold && !notified5h80 {
		notified5h80 = true
		notified5h60 = true
		notifyLog("usage fire 80%% pct=%.2f", pct)
		balloonFn(
			"Claude モニター — 5時間使用量 80%",
			fmt.Sprintf("現在 %d%% に到達しました。残量に注意してください。", clampPct(pct)),
			NIIF_WARNING,
		)
		persistNotifyState()
		return
	}
	if pct >= notify5h60Threshold && !notified5h60 {
		notified5h60 = true
		notifyLog("usage fire 60%% pct=%.2f", pct)
		balloonFn(
			"Claude モニター — 5時間使用量 60%",
			fmt.Sprintf("現在 %d%% に到達しました。", clampPct(pct)),
			NIIF_INFO,
		)
		persistNotifyState()
		return
	}
	notifyLog("usage no-fire pct=%.2f n60=%v n80=%v", pct, notified5h60, notified5h80)
	if stateChanged {
		persistNotifyState()
	}
}

func handleOverageNotification(snap UsageSnapshot) {
	if snap.AuthState != "ok" {
		return
	}
	if snap.Overage == nil || snap.Overage.SpendingLimit == nil || *snap.Overage.SpendingLimit <= 0 {
		return
	}
	cfg := snapshotConfig()
	if !cfg.NotifyOverage || !providerEnabled(cfg, "claude") {
		return
	}

	notifyMu.Lock()
	defer notifyMu.Unlock()

	var resetsAt time.Time
	if snap.Overage.ResetsAt != nil {
		resetsAt = *snap.Overage.ResetsAt
	}
	pct := snap.Overage.AmountUsed / *snap.Overage.SpendingLimit * 100

	stateChanged := false
	monthChanged := !resetsAt.IsZero() && (resetsAt.Year() != overageResetsAt.Year() || resetsAt.Month() != overageResetsAt.Month())
	if !resetsAt.Equal(overageResetsAt) {
		overageResetsAt = resetsAt
		stateChanged = true
	}
	if (monthChanged || pct == 0) && (notifiedOverage60 || notifiedOverage80) {
		notifiedOverage60 = false
		notifiedOverage80 = false
		stateChanged = true
	}

	notifyLog("overage enter pct=%.2f resetsAt=%s monthChanged=%v n60=%v n80=%v",
		pct, resetsAt.Format(time.RFC3339), monthChanged, notifiedOverage60, notifiedOverage80)

	if pct >= notify5h80Threshold && !notifiedOverage80 {
		notifiedOverage80 = true
		notifiedOverage60 = true
		notifyLog("overage fire 80%% pct=%.2f amount=%.2f", pct, snap.Overage.AmountUsed)
		balloonFn(
			"Claude モニター — 追加使用量 80%",
			fmt.Sprintf("現在 $%.2f に達しました。上限に注意してください。", snap.Overage.AmountUsed),
			NIIF_WARNING,
		)
		persistNotifyState()
		return
	}
	if pct >= notify5h60Threshold && !notifiedOverage60 {
		notifiedOverage60 = true
		notifyLog("overage fire 60%% pct=%.2f amount=%.2f", pct, snap.Overage.AmountUsed)
		balloonFn(
			"Claude モニター — 追加使用量 60%",
			fmt.Sprintf("現在 $%.2f に達しました。", snap.Overage.AmountUsed),
			NIIF_INFO,
		)
		persistNotifyState()
		return
	}
	notifyLog("overage no-fire pct=%.2f n60=%v n80=%v", pct, notifiedOverage60, notifiedOverage80)
	if stateChanged {
		persistNotifyState()
	}
}

// handleStatusNotification は status.claude.com のスナップショットを受け取り、
// 新規に検出された未解決インシデントごとに 1 度だけ通知する。
// 解決済みになった ID はフラグから削除し、再発時に再通知する。
// 起動直後の最初のスナップショットは通知抑制し、既存インシデントを基準として保持する。
func handleStatusNotification(snap StatusSnapshot) {
	cfg := snapshotConfig()
	if !cfg.NotifyStatus || !providerEnabled(cfg, "claude") {
		return
	}

	notifyMu.Lock()
	defer notifyMu.Unlock()

	currentIDs := make(map[string]bool, len(snap.Incidents))
	var newIncidents []IncidentSummary
	for _, inc := range snap.Incidents {
		if inc.ID == "" {
			continue
		}
		currentIDs[inc.ID] = true
		if !notifiedIncidents[inc.ID] {
			newIncidents = append(newIncidents, inc)
		}
	}
	for id := range notifiedIncidents {
		if !currentIDs[id] {
			delete(notifiedIncidents, id)
		}
	}

	if !notifyStatusInitialized {
		notifyStatusInitialized = true
		for id := range currentIDs {
			notifiedIncidents[id] = true
		}
		return
	}

	for _, inc := range newIncidents {
		notifiedIncidents[inc.ID] = true
		showStatusIncidentBalloon(inc)
	}
}

func showStatusIncidentBalloon(inc IncidentSummary) {
	showProviderStatusIncidentBalloon("Claude", inc)
}

func handleCodexStatusNotification(snap StatusSnapshot) {
	cfg := snapshotConfig()
	if !cfg.NotifyStatus || !providerEnabled(cfg, "codex") {
		return
	}
	notifyMu.Lock()
	defer notifyMu.Unlock()
	current := make(map[string]bool, len(snap.Incidents))
	var fresh []IncidentSummary
	for _, inc := range snap.Incidents {
		if inc.ID == "" {
			continue
		}
		current[inc.ID] = true
		if !notifiedCodexIncidents[inc.ID] {
			fresh = append(fresh, inc)
		}
	}
	for id := range notifiedCodexIncidents {
		if !current[id] {
			delete(notifiedCodexIncidents, id)
		}
	}
	if !notifyCodexStatusInitialized {
		notifyCodexStatusInitialized = true
		for id := range current {
			notifiedCodexIncidents[id] = true
		}
		return
	}
	for _, inc := range fresh {
		notifiedCodexIncidents[inc.ID] = true
		showProviderStatusIncidentBalloon("OpenAI", inc)
	}
}

func showProviderStatusIncidentBalloon(provider string, inc IncidentSummary) {
	impactLabel := ""
	flag := uint32(NIIF_INFO)
	switch inc.Impact {
	case "minor":
		impactLabel = "軽微な障害"
	case "major":
		impactLabel = "重大な障害"
		flag = NIIF_WARNING
	case "critical":
		impactLabel = "致命的な障害"
		flag = NIIF_ERROR
	case "maintenance":
		impactLabel = "メンテナンス"
	}
	title := provider + " Status — 障害検知"
	if impactLabel != "" {
		title = provider + " Status — " + impactLabel
	}
	msg := inc.Name
	if msg == "" {
		msg = "進行中のインシデントが検出されました"
	}
	balloonFn(title, msg, flag)
}
