package main

import (
	"testing"
	"time"
)

func TestExtractTierMultiplier(t *testing.T) {
	cases := map[string]string{
		"default_claude_max_20x": "20x",
		"default_claude_max_5x":  "5x",
		"auto_prepaid_tier_0":    "",
		"":                       "",
		"claude_pro":             "",
		"x":                      "", // suffix is "x" alone, no digits
		"_5x":                    "5x",
	}
	for tier, want := range cases {
		if got := extractTierMultiplier(tier); got != want {
			t.Errorf("extractTierMultiplier(%q): want %q, got %q", tier, want, got)
		}
	}
}

func TestDeriveSubscriptionType(t *testing.T) {
	cases := []struct {
		name string
		caps []string
		tier string
		want string
	}{
		{"max with 20x suffix", []string{"claude_max", "chat"}, "default_claude_max_20x", "Claude Max 20x"},
		{"max only", []string{"claude_max"}, "", "Claude Max"},
		{"pro", []string{"claude_pro"}, "default_claude_pro", "Claude Pro"},
		{"team", []string{"claude_team"}, "", "Claude Team"},
		{"api individual", []string{"api", "api_individual"}, "auto_prepaid_tier_0", "API"},
		{"unknown caps fallback", []string{"weird_cap"}, "", "weird_cap"},
		{"empty caps", []string{}, "", ""},
		{"empty caps with tier suffix", []string{}, "default_claude_max_5x", "5x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveSubscriptionType(tc.caps, tc.tier); got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestMapClaudeWindow(t *testing.T) {
	resetsAt := "2026-04-30T02:00:00.434264+00:00"
	w := mapClaudeWindow(&rawClaudeWindow{Utilization: 3.0, ResetsAt: &resetsAt})
	if w.Utilization != 3.0 {
		t.Fatalf("utilization: want 3.0, got %v", w.Utilization)
	}
	if w.ResetsAt == nil {
		t.Fatalf("resetsAt should be parsed, got nil")
	}
	wantUTC := time.Date(2026, 4, 30, 2, 0, 0, 434264000, time.UTC)
	if !w.ResetsAt.Equal(wantUTC) {
		t.Fatalf("resetsAt: want %v, got %v", wantUTC, w.ResetsAt.UTC())
	}
}

func TestMapClaudeWindow_NilAndEmpty(t *testing.T) {
	if got := mapClaudeWindow(nil); got.Utilization != 0 || got.ResetsAt != nil {
		t.Fatalf("nil input: want zero value, got %+v", got)
	}
	empty := ""
	got := mapClaudeWindow(&rawClaudeWindow{Utilization: 7.5, ResetsAt: &empty})
	if got.Utilization != 7.5 {
		t.Fatalf("util: want 7.5, got %v", got.Utilization)
	}
	if got.ResetsAt != nil {
		t.Fatalf("empty resetsAt should not parse, got %v", got.ResetsAt)
	}
}
