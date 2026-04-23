package main

import (
	"testing"
	"time"
)

func findService(snap StatusSnapshot, name string) ServiceStatus {
	for _, s := range snap.Services {
		if s.Name == name {
			return s
		}
	}
	return ServiceStatus{}
}

func TestBuildSnapshot_OperationalByDefault(t *testing.T) {
	raw := statuspageSummary{}
	raw.Components = []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}{
		{Name: "claude.ai", Status: "operational"},
		{Name: "Claude Code", Status: "operational"},
		{Name: "Claude Cowork", Status: "operational"},
	}

	snap := buildSnapshot(raw, time.Unix(0, 0).UTC())

	if len(snap.Incidents) != 0 {
		t.Fatalf("expected 0 incidents, got %d", len(snap.Incidents))
	}
	if got := findService(snap, "claude.ai").Status; got != "operational" {
		t.Fatalf("claude.ai: want operational, got %q", got)
	}
}

func TestBuildSnapshot_ActiveIncidentOverridesComponent(t *testing.T) {
	raw := statuspageSummary{}
	raw.Components = []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}{
		{Name: "claude.ai", Status: "operational"},
		{Name: "Claude Code", Status: "operational"},
		{Name: "Claude Cowork", Status: "operational"},
	}
	raw.Incidents = []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Impact     string `json:"impact"`
		Shortlink  string `json:"shortlink"`
		ResolvedAt string `json:"resolved_at"`
		UpdatedAt  string `json:"updated_at"`
		Components []struct {
			Name string `json:"name"`
		} `json:"components"`
	}{
		{
			ID:        "abc",
			Name:      "MCP apps unavailable on Claude.ai",
			Status:    "identified",
			Impact:    "major",
			Shortlink: "https://stspg.io/abc",
			Components: []struct {
				Name string `json:"name"`
			}{{Name: "claude.ai"}},
		},
	}

	snap := buildSnapshot(raw, time.Unix(0, 0).UTC())

	if got := findService(snap, "claude.ai").Status; got != "partial_outage" {
		t.Fatalf("claude.ai: want partial_outage, got %q", got)
	}
	if got := findService(snap, "Claude Code").Status; got != "operational" {
		t.Fatalf("Claude Code: want operational, got %q", got)
	}
	if len(snap.Incidents) != 1 {
		t.Fatalf("want 1 incident, got %d", len(snap.Incidents))
	}
	if snap.Incidents[0].Name != "MCP apps unavailable on Claude.ai" {
		t.Fatalf("unexpected incident name %q", snap.Incidents[0].Name)
	}
}

func TestBuildSnapshot_ResolvedIncidentIgnored(t *testing.T) {
	raw := statuspageSummary{}
	raw.Components = []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}{
		{Name: "claude.ai", Status: "operational"},
	}
	raw.Incidents = []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Impact     string `json:"impact"`
		Shortlink  string `json:"shortlink"`
		ResolvedAt string `json:"resolved_at"`
		UpdatedAt  string `json:"updated_at"`
		Components []struct {
			Name string `json:"name"`
		} `json:"components"`
	}{
		{
			ID:         "resolved-1",
			Name:       "Past incident",
			Status:     "resolved",
			Impact:     "major",
			ResolvedAt: "2026-04-23T00:57:00Z",
			Components: []struct {
				Name string `json:"name"`
			}{{Name: "claude.ai"}},
		},
	}

	snap := buildSnapshot(raw, time.Unix(0, 0).UTC())

	if len(snap.Incidents) != 0 {
		t.Fatalf("resolved incident must be filtered, got %d", len(snap.Incidents))
	}
	if got := findService(snap, "claude.ai").Status; got != "operational" {
		t.Fatalf("claude.ai: want operational, got %q", got)
	}
}

func TestBuildSnapshot_ComponentSeverityWins(t *testing.T) {
	raw := statuspageSummary{}
	raw.Components = []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}{
		{Name: "claude.ai", Status: "partial_outage"},
	}
	raw.Incidents = []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Impact     string `json:"impact"`
		Shortlink  string `json:"shortlink"`
		ResolvedAt string `json:"resolved_at"`
		UpdatedAt  string `json:"updated_at"`
		Components []struct {
			Name string `json:"name"`
		} `json:"components"`
	}{
		{
			ID:     "minor-1",
			Name:   "Minor blip",
			Status: "investigating",
			Impact: "minor", // degraded_performance < partial_outage
			Components: []struct {
				Name string `json:"name"`
			}{{Name: "claude.ai"}},
		},
	}

	snap := buildSnapshot(raw, time.Unix(0, 0).UTC())

	if got := findService(snap, "claude.ai").Status; got != "partial_outage" {
		t.Fatalf("claude.ai: want partial_outage (seed kept), got %q", got)
	}
}

func TestImpactToTileStatus(t *testing.T) {
	cases := map[string]string{
		"minor":       "degraded_performance",
		"major":       "partial_outage",
		"critical":    "major_outage",
		"maintenance": "under_maintenance",
		"none":        "",
		"":            "",
	}
	for impact, want := range cases {
		if got := impactToTileStatus(impact); got != want {
			t.Errorf("impact=%q: want %q, got %q", impact, want, got)
		}
	}
}
