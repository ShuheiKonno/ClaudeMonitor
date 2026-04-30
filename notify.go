package main

import (
	"fmt"
	"sync"
	"time"
)

// 5 時間使用率の通知閾値。
const (
	notify5h60Threshold = 60.0
	notify5h80Threshold = 80.0
)

var (
	notifyMu sync.Mutex

	// 5h 使用率の通知済みフラグ。FiveHour.ResetsAt が変わったらリセット。
	notify5hResetsAt    time.Time
	notified5h60        bool
	notified5h80        bool
	notify5hInitialized bool

	// 通知済みインシデント ID。解決した ID は次回フェッチで削除する。
	notifiedIncidents       = map[string]bool{}
	notifyStatusInitialized bool

	// テスト時に差し替えるための関数フック。本番は Win32 バルーン通知を呼ぶ。
	balloonFn = showBalloonNotification
)

// handleUsageNotification は使用率スナップショットを受け取り、
// 5h 使用量が 60% / 80% を超えたタイミングで一度だけバルーン通知を出す。
// 同じウィンドウ内では再通知しない（FiveHour.ResetsAt が変わったらリセット）。
// 起動直後の最初のスナップショットは通知抑制し、基準値として保持する。
func handleUsageNotification(snap UsageSnapshot) {
	if snap.AuthState != "ok" {
		return
	}
	cfg := snapshotConfig()
	if !cfg.NotifyUsage {
		return
	}

	notifyMu.Lock()
	defer notifyMu.Unlock()

	var resetsAt time.Time
	if snap.FiveHour.ResetsAt != nil {
		resetsAt = *snap.FiveHour.ResetsAt
	}
	if !resetsAt.Equal(notify5hResetsAt) {
		notify5hResetsAt = resetsAt
		notified5h60 = false
		notified5h80 = false
	}

	pct := snap.FiveHour.Utilization

	if !notify5hInitialized {
		notify5hInitialized = true
		// 起動時に既に閾値超過なら抑制（リセット後に正しく検知させる）。
		if pct >= notify5h60Threshold {
			notified5h60 = true
		}
		if pct >= notify5h80Threshold {
			notified5h80 = true
		}
		return
	}

	if pct >= notify5h80Threshold && !notified5h80 {
		notified5h80 = true
		notified5h60 = true
		balloonFn(
			"Claude モニター — 5時間使用量 80%",
			fmt.Sprintf("現在 %d%% に到達しました。残量に注意してください。", clampPct(pct)),
			NIIF_WARNING,
		)
		return
	}
	if pct >= notify5h60Threshold && !notified5h60 {
		notified5h60 = true
		balloonFn(
			"Claude モニター — 5時間使用量 60%",
			fmt.Sprintf("現在 %d%% に到達しました。", clampPct(pct)),
			NIIF_INFO,
		)
	}
}

// handleStatusNotification は status.claude.com のスナップショットを受け取り、
// 新規に検出された未解決インシデントごとに 1 度だけ通知する。
// 解決済みになった ID はフラグから削除し、再発時に再通知する。
// 起動直後の最初のスナップショットは通知抑制し、既存インシデントを基準として保持する。
func handleStatusNotification(snap StatusSnapshot) {
	cfg := snapshotConfig()
	if !cfg.NotifyStatus {
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
	title := "Claude Status — 障害検知"
	if impactLabel != "" {
		title = "Claude Status — " + impactLabel
	}
	msg := inc.Name
	if msg == "" {
		msg = "進行中のインシデントが検出されました"
	}
	balloonFn(title, msg, flag)
}
