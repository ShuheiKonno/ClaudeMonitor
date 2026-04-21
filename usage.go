package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)


// UsageWindow は 1 つの使用率ウィンドウ（%）と次回リセット時刻。
type UsageWindow struct {
	Utilization float64    `json:"utilization"`
	ResetsAt    *time.Time `json:"resetsAt"`
}

// UsageSnapshot はウィジェット/トレイが参照する統合ビュー。
// AuthState により UI がエラーバナーを出すか判断する。
type UsageSnapshot struct {
	FiveHour UsageWindow `json:"fiveHour"`
	SevenDay UsageWindow `json:"sevenDay"`

	Email            string `json:"email"`
	DisplayName      string `json:"displayName"`
	SubscriptionType string `json:"subscriptionType"`

	// AuthState: "ok" | "expired" | "missing" | "network_error" | "init"
	AuthState string    `json:"authState"`
	LastError string    `json:"lastError,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

var (
	usageMu     sync.RWMutex
	cachedUsage UsageSnapshot
	// refreshMu は並行する refreshUsage 呼び出しを直列化して API への
	// 重複フェッチを防ぐ（ticker / 手動更新 / トレイ更新 が同時に走っても 1 回にまとめる）。
	refreshMu sync.Mutex
)

// refreshUsage は認証ファイル読み込み → API 呼び出しを同期実行する。
// ネットワーク失敗時は直近の Utilization と UpdatedAt を保持したまま AuthState のみ更新する
// （UpdatedAt は「最終成功フェッチ時刻」の意味を保つ — 失敗時に now で上書きしない）。
func refreshUsage() {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	auth, err := loadAuthData()
	if err != nil {
		updateSnapshotErr("missing", fmt.Sprintf("認証情報を取得できません: %v", err))
		return
	}
	if auth.isExpired() {
		updateSnapshotErr("expired", fmt.Sprintf("OAuth トークンの期限切れ（%s）", auth.ExpiresAt.Local().Format("2006-01-02 15:04")))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	usage, err := fetchUsage(ctx, auth)
	if err != nil {
		state := "network_error"
		if errors.Is(err, ErrAuthExpired) {
			state = "expired"
		}
		// アカウント情報だけは更新しておく（UI の宛先表示用）。
		// UpdatedAt は成功時のスナップショット時刻を保持する（失敗時は更新しない）。
		usageMu.Lock()
		cachedUsage.Email = auth.Email
		cachedUsage.DisplayName = auth.DisplayName
		cachedUsage.SubscriptionType = auth.SubscriptionType
		cachedUsage.AuthState = state
		cachedUsage.LastError = err.Error()
		usageMu.Unlock()
		updateTrayFromSnapshot()
		return
	}

	snap := UsageSnapshot{
		FiveHour:         mapWindow(usage.FiveHour),
		SevenDay:         mapWindow(usage.SevenDay),
		Email:            auth.Email,
		DisplayName:      auth.DisplayName,
		SubscriptionType: auth.SubscriptionType,
		AuthState:        "ok",
		UpdatedAt:        time.Now(),
	}
	usageMu.Lock()
	cachedUsage = snap
	usageMu.Unlock()
	// 取得完了後にトレイアイコンも反映（30秒 ticker の到来を待たない）。
	// トレイ未初期化ならガードにより no-op。
	updateTrayFromSnapshot()
}

func mapWindow(raw *rawUsageWindow) UsageWindow {
	if raw == nil {
		return UsageWindow{}
	}
	u := UsageWindow{Utilization: raw.Utilization}
	if raw.ResetsAt != nil && *raw.ResetsAt != "" {
		if t, err := time.Parse(time.RFC3339, *raw.ResetsAt); err == nil {
			u.ResetsAt = &t
		}
	}
	return u
}

// updateSnapshotErr は認証失敗などデータ取得以前のエラーを記録する。
// UpdatedAt は触らない（最終成功フェッチ時刻を保持）。
func updateSnapshotErr(state, msg string) {
	usageMu.Lock()
	cachedUsage.AuthState = state
	cachedUsage.LastError = msg
	usageMu.Unlock()
	fmt.Fprintln(os.Stderr, "[usage]", state, msg)
	updateTrayFromSnapshot()
}

// startCollector はバックグラウンドの定期取得ループを開始する。
// 初回フェッチは goroutine で非同期実行（起動をブロックしない）。
// WebView フロントは authState=="init" の間だけ短間隔ポーリングし、
// 取得完了後に 5 分 / 60 秒インターバルに切り替える想定。
// プロセス寿命 = アプリ寿命なので明示的な停止は行わない（os.Exit でクリーンアップ）。
func startCollector() {
	usageMu.Lock()
	cachedUsage.AuthState = "init"
	usageMu.Unlock()

	go refreshUsage()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			refreshUsage()
		}
	}()
}

func getUsageSnapshot() UsageSnapshot {
	usageMu.RLock()
	defer usageMu.RUnlock()
	return cachedUsage
}
