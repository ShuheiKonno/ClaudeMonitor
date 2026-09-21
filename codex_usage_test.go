package main

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestClassifyCodexWindowsFiveHourAndSevenDay(t *testing.T) {
	primary := &codexRateWindow{UsedPercent: 33, LimitWindowSeconds: 5 * 60 * 60, ResetAt: 1_800_000_000}
	secondary := &codexRateWindow{UsedPercent: 78, LimitWindowSeconds: 7 * 24 * 60 * 60, ResetAt: 1_800_500_000}
	five, seven := classifyCodexWindows(primary, secondary)
	if five.Label != "5時間" || five.Utilization != 33 {
		t.Fatalf("unexpected short window: %+v", five)
	}
	if seven.Label != "7日" || seven.Utilization != 78 {
		t.Fatalf("unexpected long window: %+v", seven)
	}
	if five.ResetsAt == nil || five.ResetsAt.Unix() != primary.ResetAt {
		t.Fatalf("short reset was not mapped: %+v", five.ResetsAt)
	}
}

func TestClassifyCodexWindowsWeeklyOnly(t *testing.T) {
	primary := &codexRateWindow{UsedPercent: 81, LimitWindowSeconds: 7 * 24 * 60 * 60}
	five, seven := classifyCodexWindows(primary, nil)
	if five.Label != "" {
		t.Fatalf("weekly limit must not be presented as short window: %+v", five)
	}
	if seven.Label != "7日" || seven.Utilization != 81 {
		t.Fatalf("unexpected weekly window: %+v", seven)
	}
}

func TestCodexIdentityReadsDisplayClaims(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"u@example.com","name":"User"}`))
	email, name := codexIdentity("x." + payload + ".y")
	if email != "u@example.com" || name != "User" {
		t.Fatalf("got email=%q name=%q", email, name)
	}
}

func TestMapCodexWindowReset(t *testing.T) {
	w := mapCodexWindow(&codexRateWindow{UsedPercent: 12.5, LimitWindowSeconds: 3600, ResetAt: 1_700_000_000})
	if w.Label != "1時間" || w.ResetsAt == nil || !w.ResetsAt.Equal(time.Unix(1_700_000_000, 0)) {
		t.Fatalf("unexpected window: %+v", w)
	}
}

func TestCodexPlanLabel(t *testing.T) {
	if got := codexPlanLabel("plus"); got != "ChatGPT Plus" {
		t.Fatalf("got %q", got)
	}
	if got := codexPlanLabel("enterprise"); got != "ChatGPT Enterprise" {
		t.Fatalf("got %q", got)
	}
}
