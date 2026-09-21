package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	codexStatusMu        sync.RWMutex
	cachedCodexStatus    StatusSnapshot
	lastCodexStatusFetch time.Time
)

var codexStatusTargets = []string{"Codex Web", "Codex in ChatGPT Desktop", "CLI"}

func fetchCodexServiceStatus() (StatusSnapshot, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://status.openai.com/api/v2/summary.json")
	if err != nil {
		return StatusSnapshot{}, err
	}
	defer resp.Body.Close()
	var raw statuspageSummary
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return StatusSnapshot{}, err
	}
	return buildTargetedStatusSnapshot(raw, codexStatusTargets, time.Now().UTC()), nil
}

func buildTargetedStatusSnapshot(raw statuspageSummary, targets []string, now time.Time) StatusSnapshot {
	targetSet := make(map[string]bool, len(targets))
	componentStatus := make(map[string]string, len(raw.Components))
	for _, target := range targets {
		targetSet[strings.ToLower(target)] = true
	}
	for _, c := range raw.Components {
		componentStatus[strings.ToLower(c.Name)] = c.Status
	}
	incidents := make([]IncidentSummary, 0)
	incidentImpact := make(map[string]string)
	for _, inc := range raw.Incidents {
		if inc.ResolvedAt != "" || inc.Status == "resolved" || inc.Status == "postmortem" {
			continue
		}
		relevant := false
		for _, c := range inc.Components {
			key := strings.ToLower(c.Name)
			if !targetSet[key] {
				continue
			}
			relevant = true
			mapped := impactToTileStatus(inc.Impact)
			if severity(mapped) > severity(incidentImpact[key]) {
				incidentImpact[key] = mapped
			}
		}
		if relevant {
			incidents = append(incidents, IncidentSummary{ID: inc.ID, Name: inc.Name, Impact: inc.Impact, Status: inc.Status, Shortlink: inc.Shortlink, UpdatedAt: inc.UpdatedAt})
		}
	}
	services := make([]ServiceStatus, len(targets))
	for i, target := range targets {
		key := strings.ToLower(target)
		state := componentStatus[key]
		if state == "" {
			state = "unknown"
		}
		if override := incidentImpact[key]; severity(override) > severity(state) {
			state = override
		}
		services[i] = ServiceStatus{Name: target, Status: state}
	}
	return StatusSnapshot{Services: services, Indicator: raw.Status.Indicator, Description: raw.Status.Description, Incidents: incidents, FetchedAt: now.Format(time.RFC3339)}
}

func getCodexStatusSnapshot() StatusSnapshot {
	codexStatusMu.RLock()
	if !lastCodexStatusFetch.IsZero() && time.Since(lastCodexStatusFetch) < statusCacheTTL() {
		out := cachedCodexStatus
		codexStatusMu.RUnlock()
		return out
	}
	codexStatusMu.RUnlock()
	fetchStart := time.Now()
	snap, err := fetchCodexServiceStatus()
	if err != nil {
		codexStatusMu.RLock()
		old := cachedCodexStatus
		codexStatusMu.RUnlock()
		if old.FetchedAt == "" {
			for _, name := range codexStatusTargets {
				old.Services = append(old.Services, ServiceStatus{Name: name, Status: "unknown"})
			}
		}
		return old
	}
	codexStatusMu.Lock()
	cachedCodexStatus = snap
	lastCodexStatusFetch = fetchStart
	codexStatusMu.Unlock()
	handleCodexStatusNotification(snap)
	return snap
}

func invalidateCodexStatusCache() {
	codexStatusMu.Lock()
	lastCodexStatusFetch = time.Time{}
	codexStatusMu.Unlock()
}
