package main

import (
	"strings"
	"testing"
)

func TestOverviewContainsOverageProgressBars(t *testing.T) {
	html := getHTML()
	for _, id := range []string{
		`id="ov-claude-extra-bar"`,
		`id="ov-claude-extra-bar-fill"`,
		`id="ov-codex-extra-bar"`,
		`id="ov-codex-extra-bar-fill"`,
	} {
		if !strings.Contains(html, id) {
			t.Errorf("overview HTML is missing %s", id)
		}
	}

	for _, behavior := range []string{
		"extraBarFill.style.width = pct + '%'",
		"extraBarFill.className = 'bar-fill' + (pct >= 81 ? ' crit' : pct >= 61 ? ' warn' : '')",
		"extraValue.textContent = pctDisplay + '%'",
	} {
		if !strings.Contains(html, behavior) {
			t.Errorf("overview overage rendering is missing %q", behavior)
		}
	}
}
