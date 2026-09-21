package main

import (
	"testing"
	"time"
)

func TestClampPollSeconds(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, minPollSeconds},     // present-and-0 / 欠落 → 下限へ（panic 防止の要）
		{-5, minPollSeconds},    // 負値 → 下限へ
		{59, minPollSeconds},    // 下限未満 → 下限
		{60, 60},                // 下限ちょうど
		{300, 300},              // 既定値はそのまま
		{3600, 3600},            // 上限ちょうど
		{99999, maxPollSeconds}, // 上限超過 → 上限
	}
	for _, c := range cases {
		if got := clampPollSeconds(c.in); got != c.want {
			t.Errorf("clampPollSeconds(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNormalizeResetTimeFormat(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "datetime"},         // 旧config・欠損キー → 既定
		{"datetime", "datetime"}, // 既定値はそのまま
		{"relative", "relative"}, // 相対表示
		{"invalid", "datetime"},  // 不正値 → 既定
	}
	for _, c := range cases {
		if got := normalizeResetTimeFormat(c.in); got != c.want {
			t.Errorf("normalizeResetTimeFormat(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeLayoutMode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "tabs"},
		{"tabs", "tabs"},
		{"overview", "overview"},
		{"invalid", "tabs"},
	}
	for _, c := range cases {
		if got := normalizeLayoutMode(c.in); got != c.want {
			t.Errorf("normalizeLayoutMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWindowSizeForLayout(t *testing.T) {
	if w, h := windowSizeForLayout("tabs"); w != tabWindowWidth || h != windowHeight {
		t.Fatalf("tabs size = %dx%d", w, h)
	}
	if w, h := windowSizeForLayout("overview"); w != overviewWindowWidth || h != windowHeight {
		t.Fatalf("overview size = %dx%d", w, h)
	}
}

func TestNormalizeProviderSelection(t *testing.T) {
	claude, codex, active := normalizeProviderSelection(false, false, "codex")
	if !claude || codex || active != "claude" {
		t.Fatalf("empty selection = (%v,%v,%q), want Claude only", claude, codex, active)
	}
	claude, codex, active = normalizeProviderSelection(false, true, "claude")
	if claude || !codex || active != "codex" {
		t.Fatalf("Codex only = (%v,%v,%q)", claude, codex, active)
	}
	claude, codex, active = normalizeProviderSelection(true, true, "codex")
	if !claude || !codex || active != "codex" {
		t.Fatalf("both = (%v,%v,%q)", claude, codex, active)
	}
}

func TestWindowSizeForConfig(t *testing.T) {
	c := defaultConfig()
	c.LayoutMode = "overview"
	if w, h := windowSizeForConfig(c); w != overviewWindowWidth || h != windowHeight {
		t.Fatalf("both providers overview size = %dx%d", w, h)
	}
	c.CodexEnabled = false
	if w, h := windowSizeForConfig(c); w != tabWindowWidth || h != windowHeight {
		t.Fatalf("single provider overview size = %dx%d", w, h)
	}
}

func TestRightAnchoredX(t *testing.T) {
	const rightEdge int32 = 1920
	if got := rightAnchoredX(rightEdge, 595); got != 1325 {
		t.Fatalf("overview x = %d, want 1325", got)
	}
	if got := rightAnchoredX(rightEdge, 308); got != 1612 {
		t.Fatalf("tabs x = %d, want 1612", got)
	}
}

// TestPollIntervalsNeverZero は config が 0/範囲外でも間隔が常に正であることを保証する。
// time.NewTicker / Ticker.Reset は間隔 <= 0 で panic するため、この不変条件が重要。
func TestPollIntervalsNeverZero(t *testing.T) {
	withConfig(t, func(c *Config) {
		c.UsagePollSeconds = 0
		c.StatusPollSeconds = 0
	})
	if d := usagePollInterval(); d <= 0 {
		t.Errorf("usagePollInterval() = %v, want > 0", d)
	}
	if d := statusPollInterval(); d <= 0 {
		t.Errorf("statusPollInterval() = %v, want > 0", d)
	}
	if d := statusCacheTTL(); d <= 0 {
		t.Errorf("statusCacheTTL() = %v, want > 0", d)
	}
}

// TestStatusCacheTTLBelowInterval は TTL が監視間隔より短いことを確認する。
// 同値だと strict 比較 + 位相ずれで実効再取得間隔が約2倍になるため、
// 毎回の JS ポーリングが確実に再取得できるよう TTL < 間隔 を保つ。
func TestStatusCacheTTLBelowInterval(t *testing.T) {
	withConfig(t, func(c *Config) { c.StatusPollSeconds = 300 })
	interval := statusPollInterval()
	ttl := statusCacheTTL()
	if ttl >= interval {
		t.Errorf("statusCacheTTL() = %v, want < interval %v", ttl, interval)
	}
	if want := 270 * time.Second; ttl != want {
		t.Errorf("statusCacheTTL() = %v, want %v (= 300s * 0.9)", ttl, want)
	}
}
