package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type StatusSnapshot struct {
	Services  []ServiceStatus `json:"services"`
	FetchedAt string          `json:"fetchedAt"`
}

var (
	statusMu        sync.RWMutex
	cachedStatus    StatusSnapshot
	lastStatusFetch time.Time
	statusCacheTTL  = 5 * time.Minute
)

// exact component names from https://status.claude.com/api/v2/summary.json
var statusTargets = []string{
	"claude.ai",
	"Claude Code",
	"Claude Cowork",
}

type statuspageSummary struct {
	Components []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"components"`
}

func fetchServiceStatus() (StatusSnapshot, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://status.claude.com/api/v2/summary.json")
	if err != nil {
		return StatusSnapshot{}, err
	}
	defer resp.Body.Close()

	var raw statuspageSummary
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return StatusSnapshot{}, err
	}

	nameToStatus := make(map[string]string, len(raw.Components))
	for _, c := range raw.Components {
		nameToStatus[c.Name] = c.Status
	}

	services := make([]ServiceStatus, len(statusTargets))
	for i, name := range statusTargets {
		st, ok := nameToStatus[name]
		if !ok {
			st = "unknown"
		}
		services[i] = ServiceStatus{Name: name, Status: st}
	}

	return StatusSnapshot{
		Services:  services,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func getStatusSnapshot() StatusSnapshot {
	statusMu.RLock()
	if !lastStatusFetch.IsZero() && time.Since(lastStatusFetch) < statusCacheTTL {
		snap := cachedStatus
		statusMu.RUnlock()
		return snap
	}
	statusMu.RUnlock()

	snap, err := fetchServiceStatus()
	if err != nil {
		statusMu.RLock()
		old := cachedStatus
		statusMu.RUnlock()
		if old.FetchedAt == "" {
			services := make([]ServiceStatus, len(statusTargets))
			for i, name := range statusTargets {
				services[i] = ServiceStatus{Name: name, Status: "unknown"}
			}
			old = StatusSnapshot{Services: services}
		}
		return old
	}

	statusMu.Lock()
	cachedStatus = snap
	lastStatusFetch = time.Now()
	statusMu.Unlock()
	return snap
}
