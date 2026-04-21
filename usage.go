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
	usageMu      sync.RWMutex
	cachedUsage  UsageSnapshot
	usageTrigger = make(chan struct{}, 1)
)

// refreshUsage は認証ファイル読み込み → API 呼び出しを同期実行する。
// ネットワーク失敗時は直近の Utilization を保持したまま AuthState のみ更新。
func refreshUsage() {
	now := time.Now()

	auth, err := loadAuthData()
	if err != nil {
		updateSnapshotErr("missing", fmt.Sprintf("認証情報を取得できません: %v", err), now)
		return
	}
	if auth.isExpired() {
		updateSnapshotErr("expired", fmt.Sprintf("OAuth トークンの期限切れ（%s）", auth.ExpiresAt.Local().Format("2006-01-02 15:04")), now)
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
		// アカウント情報だけは更新しておく（UI の宛先表示用）
		usageMu.Lock()
		cachedUsage.Email = auth.Email
		cachedUsage.DisplayName = auth.DisplayName
		cachedUsage.SubscriptionType = auth.SubscriptionType
		cachedUsage.AuthState = state
		cachedUsage.LastError = err.Error()
		cachedUsage.UpdatedAt = now
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
		UpdatedAt:        now,
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

func updateSnapshotErr(state, msg string, now time.Time) {
	usageMu.Lock()
	cachedUsage.AuthState = state
	cachedUsage.LastError = msg
	cachedUsage.UpdatedAt = now
	usageMu.Unlock()
	fmt.Fprintln(os.Stderr, "[usage]", state, msg)
	updateTrayFromSnapshot()
}

// startCollector はバックグラウンドの定期取得ループを開始する。
// 5 分間隔 + 手動トリガ（triggerRefresh 経由）に反応する。
// プロセス寿命 = アプリ寿命なので明示的な停止は行わない（os.Exit でクリーンアップ）。
func startCollector() {
	usageMu.Lock()
	cachedUsage.AuthState = "init"
	usageMu.Unlock()

	go refreshUsage()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refreshUsage()
			case <-usageTrigger:
				refreshUsage()
			}
		}
	}()
}

// triggerRefresh は非同期で refreshUsage を 1 回走らせる（多重呼び出しは coalesce）。
func triggerRefresh() {
	select {
	case usageTrigger <- struct{}{}:
	default:
	}
}

func getUsageSnapshot() UsageSnapshot {
	usageMu.RLock()
	defer usageMu.RUnlock()
	return cachedUsage
}
