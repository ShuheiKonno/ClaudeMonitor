package main

import (
	"strings"
	"testing"
	"time"
)

type balloonCall struct {
	title string
	msg   string
	flag  uint32
}

// resetNotifyState は通知のグローバル状態をテスト前に初期化する。
func resetNotifyState() {
	notifyMu.Lock()
	notify5hResetsAt = time.Time{}
	notified5h60 = false
	notified5h80 = false
	overageResetsAt = time.Time{}
	notifiedOverage60 = false
	notifiedOverage80 = false
	notifiedIncidents = map[string]bool{}
	notifyStatusInitialized = false
	notifyMu.Unlock()
}

// withMockBalloon は balloonFn を差し替え、呼び出しを記録するフックをテストに渡す。
func withMockBalloon(t *testing.T) *[]balloonCall {
	t.Helper()
	calls := &[]balloonCall{}
	prev := balloonFn
	balloonFn = func(title, msg string, flag uint32) {
		*calls = append(*calls, balloonCall{title, msg, flag})
	}
	t.Cleanup(func() { balloonFn = prev })
	return calls
}

// withConfig は config を一時的に書き換える（テスト後に復元）。
func withConfig(t *testing.T, modify func(c *Config)) {
	t.Helper()
	configMu.Lock()
	prev := config
	modify(&config)
	configMu.Unlock()
	t.Cleanup(func() {
		configMu.Lock()
		config = prev
		configMu.Unlock()
	})
}

func snap5h(util float64, resetsAt time.Time) UsageSnapshot {
	return UsageSnapshot{
		AuthState: "ok",
		FiveHour:  UsageWindow{Utilization: util, ResetsAt: &resetsAt},
	}
}

func overageLimit(v float64) *float64 {
	return &v
}

func snapOverage(amount float64, limit *float64, resetsAt time.Time) UsageSnapshot {
	return UsageSnapshot{
		AuthState: "ok",
		Overage: &OverageInfo{
			AmountUsed:    amount,
			SpendingLimit: limit,
			ResetsAt:      &resetsAt,
		},
	}
}

func TestUsageNotify_SuppressOnInitialSnapshot(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	handleUsageNotification(snap5h(45, r))

	if len(*calls) != 0 {
		t.Fatalf("初回スナップショットでは通知抑制されるべき: got %d calls", len(*calls))
	}
}

func TestUsageNotify_60PctCrossingFiresOnce(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	handleUsageNotification(snap5h(45, r)) // 初期化
	handleUsageNotification(snap5h(65, r)) // 60% 跨ぎ
	handleUsageNotification(snap5h(70, r)) // 既通知
	handleUsageNotification(snap5h(75, r)) // 既通知

	if len(*calls) != 1 {
		t.Fatalf("60%% 跨ぎは 1 回だけ通知されるべき: got %d calls", len(*calls))
	}
	if !strings.Contains((*calls)[0].title, "60") {
		t.Fatalf("60%% 通知タイトル想定外: %q", (*calls)[0].title)
	}
	if (*calls)[0].flag != NIIF_INFO {
		t.Fatalf("60%% は NIIF_INFO 想定: got %d", (*calls)[0].flag)
	}
}

func TestUsageNotify_80PctCrossingFiresWarningOnce(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	handleUsageNotification(snap5h(50, r)) // init
	handleUsageNotification(snap5h(65, r)) // 60 fired
	handleUsageNotification(snap5h(85, r)) // 80 fired
	handleUsageNotification(snap5h(95, r)) // 既通知

	if len(*calls) != 2 {
		t.Fatalf("60 と 80 で計 2 回通知されるべき: got %d", len(*calls))
	}
	if !strings.Contains((*calls)[1].title, "80") {
		t.Fatalf("2 回目は 80%% 通知のはず: %q", (*calls)[1].title)
	}
	if (*calls)[1].flag != NIIF_WARNING {
		t.Fatalf("80%% は NIIF_WARNING 想定: got %d", (*calls)[1].flag)
	}
}

func TestUsageNotify_DirectJumpOver80Skips60Notification(t *testing.T) {
	// 60% を経由せず一気に 80% を超えた場合、80 通知だけが出る。
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	handleUsageNotification(snap5h(40, r)) // init
	handleUsageNotification(snap5h(85, r)) // 80 (60 もマーク済扱い)

	if len(*calls) != 1 {
		t.Fatalf("80%% 通知 1 回だけのはず: got %d", len(*calls))
	}
	if !strings.Contains((*calls)[0].title, "80") {
		t.Fatalf("80%% タイトルでないと不正: %q", (*calls)[0].title)
	}
}

func TestUsageNotify_ResetClearsFlagsAndRefires(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r1 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	handleUsageNotification(snap5h(50, r1)) // init
	handleUsageNotification(snap5h(65, r1)) // 60 fired

	r2 := r1.Add(5 * time.Hour)             // ResetsAt 変化 = 新セッション
	handleUsageNotification(snap5h(70, r2)) // 60 再通知されるべき

	if len(*calls) != 2 {
		t.Fatalf("リセット後の 60%% 跨ぎで再通知すべき: got %d", len(*calls))
	}
}

func TestUsageNotify_ResetsAtMinorDriftPreservesFlags(t *testing.T) {
	// claude.ai の resets_at はフェッチ毎に小幅にずれることがある。
	// 5h 未満のドリフトでフラグがクリアされて再通知されないこと。
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r1 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	handleUsageNotification(snap5h(50, r1)) // init
	handleUsageNotification(snap5h(65, r1)) // 60 fired

	handleUsageNotification(snap5h(67, r1.Add(2*time.Minute)))
	handleUsageNotification(snap5h(68, r1.Add(-1*time.Minute)))
	handleUsageNotification(snap5h(70, r1.Add(1*time.Hour)))

	if len(*calls) != 1 {
		t.Fatalf("resetsAt 微小ドリフトでは再通知すべきでない: got %d", len(*calls))
	}
}

func TestUsageNotify_ZeroPctClearsFlagsAndRefires(t *testing.T) {
	// 利用率が 0% に戻ったらフラグをクリアし、再び閾値を跨いだら再通知する。
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	handleUsageNotification(snap5h(50, r)) // init
	handleUsageNotification(snap5h(85, r)) // 80 fired
	handleUsageNotification(snap5h(0, r))  // 0% でフラグクリア
	handleUsageNotification(snap5h(65, r)) // 60 再通知されるべき

	if len(*calls) != 2 {
		t.Fatalf("0%% 復帰後の 60%% 跨ぎで再通知すべき: got %d", len(*calls))
	}
	if !strings.Contains((*calls)[1].title, "60") {
		t.Fatalf("2 回目は 60%% 通知のはず: %q", (*calls)[1].title)
	}
}

func TestUsageNotify_ResetsAtSubFiveHourJumpPreservesFlags(t *testing.T) {
	// 5h 未満（例: 1h）の前進ではフラグをクリアしない。
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r1 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	handleUsageNotification(snap5h(50, r1)) // init
	handleUsageNotification(snap5h(65, r1)) // 60 fired

	r2 := r1.Add(1 * time.Hour) // 5h 未満
	handleUsageNotification(snap5h(70, r2))

	if len(*calls) != 1 {
		t.Fatalf("5h 未満のジャンプでは再通知すべきでない: got %d", len(*calls))
	}
}

func TestUsageNotify_RestartAboveThresholdFires(t *testing.T) {
	// 再起動直後でも、未通知のウィンドウで pct が閾値超過していれば発火する。
	// 旧実装は init suppression で永久に抑制していた回帰のリグレッションテスト。
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r := time.Date(2026, 5, 2, 22, 0, 0, 0, time.UTC)
	handleUsageNotification(snap5h(95, r)) // 起動直後の最初のフェッチが既に 95%

	if len(*calls) != 1 {
		t.Fatalf("再起動直後でも 80%% 跨ぎは 1 回通知すべき: got %d", len(*calls))
	}
	if !strings.Contains((*calls)[0].title, "80") {
		t.Fatalf("80%% 通知のはず: %q", (*calls)[0].title)
	}
}

func TestUsageNotify_RestartSameWindowSuppressesViaPersistedState(t *testing.T) {
	// 永続化された通知済みフラグを config から読み込めば、同一ウィンドウ内の
	// 再起動で重複通知が出ない。
	resetNotifyState()
	calls := withMockBalloon(t)
	r := time.Date(2026, 5, 2, 22, 0, 0, 0, time.UTC)
	withConfig(t, func(c *Config) {
		c.NotifyUsage = true
		c.Notify5hResetsAt = r
		c.Notified5h60 = true
		c.Notified5h80 = true
	})
	loadNotifyState()

	handleUsageNotification(snap5h(95, r))

	if len(*calls) != 0 {
		t.Fatalf("永続化された通知済み状態では再通知しない: got %d", len(*calls))
	}
}

func TestUsageNotify_DisabledByConfig(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = false })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	handleUsageNotification(snap5h(50, r))
	handleUsageNotification(snap5h(85, r))

	if len(*calls) != 0 {
		t.Fatalf("無効時は通知ゼロ: got %d", len(*calls))
	}
}

func TestUsageNotify_AuthStateNotOkSkips(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyUsage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	snap := UsageSnapshot{
		AuthState: "needs_login",
		FiveHour:  UsageWindow{Utilization: 90, ResetsAt: &r},
	}
	handleUsageNotification(snap)

	if len(*calls) != 0 {
		t.Fatalf("AuthState != ok では通知抑制: got %d", len(*calls))
	}
}

func TestOverageNotify_NoSpendingLimitSkips(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyOverage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	handleOverageNotification(snapOverage(100, nil, r))

	if len(*calls) != 0 {
		t.Fatalf("上限なしでは通知しない: got %d", len(*calls))
	}
}

func TestOverageNotify_60PctFiresInfoOnce(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyOverage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	limit := overageLimit(100)
	handleOverageNotification(snapOverage(65, limit, r))
	handleOverageNotification(snapOverage(70, limit, r))

	if len(*calls) != 1 {
		t.Fatalf("60%% 超過は 1 回だけ通知されるべき: got %d", len(*calls))
	}
	if !strings.Contains((*calls)[0].title, "60") {
		t.Fatalf("60%% 通知タイトル想定外: %q", (*calls)[0].title)
	}
	if (*calls)[0].flag != NIIF_INFO {
		t.Fatalf("60%% は NIIF_INFO 想定: got %d", (*calls)[0].flag)
	}
}

func TestOverageNotify_80PctFiresWarningOnceAndMarks60(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyOverage = true })

	r := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	limit := overageLimit(100)
	handleOverageNotification(snapOverage(85, limit, r))
	handleOverageNotification(snapOverage(90, limit, r))

	if len(*calls) != 1 {
		t.Fatalf("80%% 超過は 1 回だけ通知されるべき: got %d", len(*calls))
	}
	if !strings.Contains((*calls)[0].title, "80") {
		t.Fatalf("80%% 通知タイトル想定外: %q", (*calls)[0].title)
	}
	if (*calls)[0].flag != NIIF_WARNING {
		t.Fatalf("80%% は NIIF_WARNING 想定: got %d", (*calls)[0].flag)
	}
	notifyMu.Lock()
	n60 := notifiedOverage60
	n80 := notifiedOverage80
	notifyMu.Unlock()
	if !n60 || !n80 {
		t.Fatalf("80%% 通知後は 60/80 両フラグが立つべき: n60=%v n80=%v", n60, n80)
	}
}

func TestOverageNotify_MonthChangeClearsFlagsAndRefires(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyOverage = true })

	limit := overageLimit(100)
	may := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	june := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	handleOverageNotification(snapOverage(65, limit, may))
	handleOverageNotification(snapOverage(70, limit, may))
	handleOverageNotification(snapOverage(65, limit, june))

	if len(*calls) != 2 {
		t.Fatalf("月跨ぎ後は再通知されるべき: got %d", len(*calls))
	}
	if !strings.Contains((*calls)[1].title, "60") {
		t.Fatalf("月跨ぎ後の再通知は 60%% のはず: %q", (*calls)[1].title)
	}
}

func snapStatus(incidents []IncidentSummary) StatusSnapshot {
	return StatusSnapshot{Incidents: incidents}
}

func TestStatusNotify_SuppressInitialIncidents(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyStatus = true })

	handleStatusNotification(snapStatus([]IncidentSummary{
		{ID: "abc", Name: "起動時から進行中", Impact: "minor"},
	}))

	if len(*calls) != 0 {
		t.Fatalf("初回 fetch の既存 incident は抑制: got %d", len(*calls))
	}
}

func TestStatusNotify_NewIncidentFires(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyStatus = true })

	handleStatusNotification(snapStatus(nil)) // init: 既存ゼロ
	handleStatusNotification(snapStatus([]IncidentSummary{
		{ID: "new1", Name: "claude.ai outage", Impact: "major"},
	}))

	if len(*calls) != 1 {
		t.Fatalf("新規 incident は 1 回通知: got %d", len(*calls))
	}
	if (*calls)[0].flag != NIIF_WARNING {
		t.Fatalf("major は NIIF_WARNING: got %d", (*calls)[0].flag)
	}
	if !strings.Contains((*calls)[0].msg, "outage") {
		t.Fatalf("メッセージに incident 名が入っているはず: %q", (*calls)[0].msg)
	}
}

func TestStatusNotify_SameIncidentNotRefired(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyStatus = true })

	handleStatusNotification(snapStatus(nil))
	handleStatusNotification(snapStatus([]IncidentSummary{{ID: "x", Name: "a", Impact: "minor"}}))
	handleStatusNotification(snapStatus([]IncidentSummary{{ID: "x", Name: "a", Impact: "minor"}}))

	if len(*calls) != 1 {
		t.Fatalf("同一 ID は再通知しない: got %d", len(*calls))
	}
}

func TestStatusNotify_ResolvedThenReoccurringRefires(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyStatus = true })

	handleStatusNotification(snapStatus(nil))                                                      // init
	handleStatusNotification(snapStatus([]IncidentSummary{{ID: "x", Name: "a", Impact: "minor"}})) // 初通知
	handleStatusNotification(snapStatus(nil))                                                      // 解決
	handleStatusNotification(snapStatus([]IncidentSummary{{ID: "x", Name: "a", Impact: "minor"}})) // 再発 → 再通知

	if len(*calls) != 2 {
		t.Fatalf("再発時は再通知: got %d", len(*calls))
	}
}

func TestStatusNotify_CriticalImpactErrorIcon(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyStatus = true })

	handleStatusNotification(snapStatus(nil))
	handleStatusNotification(snapStatus([]IncidentSummary{{ID: "z", Name: "down", Impact: "critical"}}))

	if len(*calls) != 1 {
		t.Fatalf("通知 1 件想定: got %d", len(*calls))
	}
	if (*calls)[0].flag != NIIF_ERROR {
		t.Fatalf("critical は NIIF_ERROR: got %d", (*calls)[0].flag)
	}
	if !strings.Contains((*calls)[0].title, "致命的") {
		t.Fatalf("critical タイトルに「致命的」を含むはず: %q", (*calls)[0].title)
	}
}

func TestStatusNotify_DisabledByConfig(t *testing.T) {
	resetNotifyState()
	calls := withMockBalloon(t)
	withConfig(t, func(c *Config) { c.NotifyStatus = false })

	handleStatusNotification(snapStatus(nil))
	handleStatusNotification(snapStatus([]IncidentSummary{{ID: "y", Name: "n", Impact: "major"}}))

	if len(*calls) != 0 {
		t.Fatalf("無効時は通知ゼロ: got %d", len(*calls))
	}
}
