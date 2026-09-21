package main

import (
	"testing"
	"time"
)

func TestBuildTargetedStatusSnapshotFiltersUnrelatedIncidents(t *testing.T) {
	raw := statuspageSummary{}
	raw.Components = append(raw.Components,
		struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}{Name: "Codex Web", Status: "operational"},
		struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}{Name: "CLI", Status: "operational"},
	)
	raw.Incidents = append(raw.Incidents, struct {
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
	}{ID: "unrelated", Name: "Images issue", Status: "investigating", Impact: "major", Components: []struct {
		Name string `json:"name"`
	}{{Name: "Images"}}})
	raw.Incidents = append(raw.Incidents, struct {
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
	}{ID: "codex", Name: "Codex issue", Status: "identified", Impact: "minor", Components: []struct {
		Name string `json:"name"`
	}{{Name: "Codex Web"}}})

	snap := buildTargetedStatusSnapshot(raw, []string{"Codex Web", "CLI"}, time.Unix(0, 0).UTC())
	if len(snap.Incidents) != 1 || snap.Incidents[0].ID != "codex" {
		t.Fatalf("unexpected incidents: %+v", snap.Incidents)
	}
	if snap.Services[0].Status != "degraded_performance" {
		t.Fatalf("unexpected Codex Web state: %+v", snap.Services[0])
	}
	if snap.Services[1].Status != "operational" {
		t.Fatalf("unexpected CLI state: %+v", snap.Services[1])
	}
}
