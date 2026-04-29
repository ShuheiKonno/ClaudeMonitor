package main

import (
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

	// AuthState: "ok" | "needs_login" | "network_error" | "init"
	// "needs_login" は claude.ai の Cookie が無い／失効で、
	// 補助 WebView を可視化してユーザーにログインさせる必要がある状態。
	AuthState string    `json:"authState"`
	LastError string    `json:"lastError,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

var (
	usageMu     sync.RWMutex
	cachedUsage UsageSnapshot
	// refreshMu は並行する refreshUsage 呼び出しを直列化する。
	refreshMu sync.Mutex

	// refreshNotify は同期的な /api/refresh 呼び出しが Bind コールバックの完了を待つためのチャンネル。
	// refreshNotifyMu で保護され、refreshUsage 開始時に作成・終了時に nil 化される。
	refreshNotifyMu sync.Mutex
	refreshNotify   chan struct{}
)

// refreshUsage は補助 WebView の JS に取得をリクエストし、Bind コールバック完了まで待つ。
// 並行呼び出しは refreshMu で直列化される。タイムアウトは 15 秒。
// 補助 WebView がまだ生成されていない (起動初期) ときは何もしない。
func refreshUsage() {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	if authWebViewInst == nil {
		return
	}

	ch := make(chan struct{}, 1)
	refreshNotifyMu.Lock()
	refreshNotify = ch
	refreshNotifyMu.Unlock()
	defer func() {
		refreshNotifyMu.Lock()
		refreshNotify = nil
		refreshNotifyMu.Unlock()
	}()

	// 主 WebView の Dispatch キューだけが Run でドレインされるため必ずここを通す。
	// 両 WebView は同一スレッドにあるので auth.Eval を main 経由で呼んで問題ない。
	uiDispatch(func() {
		authWebViewInst.Eval("window.__fetchClaudeUsage && window.__fetchClaudeUsage()")
	})

	select {
	case <-ch:
	case <-time.After(15 * time.Second):
		updateUsageError("network_error", "fetch timeout")
	}
}

// signalRefreshDone は Bind コールバックが完了した時に呼ぶ。
// refreshUsage が select 待ち中ならアンブロックする。
func signalRefreshDone() {
	refreshNotifyMu.Lock()
	ch := refreshNotify
	refreshNotifyMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

// updateUsageError は AuthState とエラー文字列を更新する。
// UpdatedAt は触らない (最終成功フェッチ時刻を保持)。
func updateUsageError(state, msg string) {
	usageMu.Lock()
	cachedUsage.AuthState = state
	cachedUsage.LastError = msg
	usageMu.Unlock()
	fmt.Fprintln(os.Stderr, "[usage]", state, msg)
	updateTrayFromSnapshot()
}

// startCollector はバックグラウンドの定期取得ループを開始する。
// 補助 WebView の JS 側にも 5 分の setInterval があるが、Go 側からも明示的に
// トリガーすることでスロットル時の保険とする。
// プロセス寿命 = アプリ寿命なので明示的な停止は行わない。
func startCollector() {
	usageMu.Lock()
	cachedUsage.AuthState = "init"
	usageMu.Unlock()

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
