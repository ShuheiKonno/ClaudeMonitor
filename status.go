package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type IncidentSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Impact    string `json:"impact"`
	Status    string `json:"status"`
	Shortlink string `json:"shortlink"`
	UpdatedAt string `json:"updatedAt"`
}

type StatusSnapshot struct {
	Services    []ServiceStatus   `json:"services"`
	Indicator   string            `json:"indicator"`
	Description string            `json:"description"`
	Incidents   []IncidentSummary `json:"incidents"`
	FetchedAt   string            `json:"fetchedAt"`
}

var (
	statusMu        sync.RWMutex
	cachedStatus    StatusSnapshot
	lastStatusFetch time.Time
)

// statusPollInterval は設定された障害監視間隔を返す（clampPollSeconds で [60,3600] 秒）。
func statusPollInterval() time.Duration {
	return time.Duration(clampPollSeconds(snapshotConfig().StatusPollSeconds)) * time.Second
}

// statusCacheTTL はキャッシュ有効期限を返す。間隔そのものではなく 90% にするのが要点。
// JS の setInterval(間隔) と同値にすると、time.Since(lastStatusFetch) < TTL の strict 比較 +
// フェッチ完了後に lastStatusFetch を打つ位相ずれで、JS ポーリングの約半数がキャッシュ返却となり
// 実効再取得間隔が約2倍になる。間隔よりわずかに短くすれば毎回の JS ポーリングが確実に再取得する。
// TTL の役割は「手動更新と定期ポーリングが近接したときの重複取得防止」に限定される。
func statusCacheTTL() time.Duration {
	return statusPollInterval() * 9 / 10
}

// exact component names from https://status.claude.com/api/v2/summary.json
var statusTargets = []string{
	"claude.ai",
	"Claude Code",
	"Claude Cowork",
}

type statuspageSummary struct {
	Status struct {
		Indicator   string `json:"indicator"`
		Description string `json:"description"`
	} `json:"status"`
	Components []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"components"`
	Incidents []struct {
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
	} `json:"incidents"`
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

	return buildSnapshot(raw, time.Now().UTC()), nil
}

func buildSnapshot(raw statuspageSummary, now time.Time) StatusSnapshot {
	nameToStatus := make(map[string]string, len(raw.Components))
	for _, c := range raw.Components {
		nameToStatus[c.Name] = c.Status
	}

	// Collect unresolved incidents and index worst impact per affected component.
	activeIncidents := make([]IncidentSummary, 0, len(raw.Incidents))
	componentIncidentImpact := make(map[string]string)

	for _, inc := range raw.Incidents {
		if inc.ResolvedAt != "" || inc.Status == "resolved" || inc.Status == "postmortem" {
			continue
		}
		activeIncidents = append(activeIncidents, IncidentSummary{
			ID:        inc.ID,
			Name:      inc.Name,
			Impact:    inc.Impact,
			Status:    inc.Status,
			Shortlink: inc.Shortlink,
			UpdatedAt: inc.UpdatedAt,
		})
		incTileStatus := impactToTileStatus(inc.Impact)
		if incTileStatus == "" {
			continue
		}
		for _, c := range inc.Components {
			key := strings.ToLower(c.Name)
			if severity(incTileStatus) > severity(componentIncidentImpact[key]) {
				componentIncidentImpact[key] = incTileStatus
			}
		}
	}

	services := make([]ServiceStatus, len(statusTargets))
	for i, name := range statusTargets {
		seed, ok := nameToStatus[name]
		if !ok {
			// Fall back to case-insensitive match.
			for k, v := range nameToStatus {
				if strings.EqualFold(k, name) {
					seed = v
					ok = true
					break
				}
			}
		}
		if !ok {
			seed = "unknown"
		}
		final := seed
		if override, has := componentIncidentImpact[strings.ToLower(name)]; has {
			if severity(override) > severity(final) {
				final = override
			}
		}
		services[i] = ServiceStatus{Name: name, Status: final}
	}

	return StatusSnapshot{
		Services:    services,
		Indicator:   raw.Status.Indicator,
		Description: raw.Status.Description,
		Incidents:   activeIncidents,
		FetchedAt:   now.Format(time.RFC3339),
	}
}

// impactToTileStatus maps a Statuspage incident impact to a tile-status string.
// Returns "" for impacts that should not override a component's own status
// (e.g. "none").
func impactToTileStatus(impact string) string {
	switch impact {
	case "minor":
		return "degraded_performance"
	case "major":
		return "partial_outage"
	case "critical":
		return "major_outage"
	case "maintenance":
		return "under_maintenance"
	}
	return ""
}

// severity assigns a numeric weight so we can keep the worst status after
// merging component state with active-incident overrides.
func severity(status string) int {
	switch status {
	case "major_outage":
		return 4
	case "partial_outage":
		return 3
	case "degraded_performance":
		return 2
	case "under_maintenance":
		return 1
	case "operational":
		return 0
	}
	return -1
}

func getStatusSnapshot() StatusSnapshot {
	statusMu.RLock()
	if !lastStatusFetch.IsZero() && time.Since(lastStatusFetch) < statusCacheTTL() {
		snap := cachedStatus
		statusMu.RUnlock()
		return snap
	}
	statusMu.RUnlock()

	// 取得「開始」時刻を基準にする。完了時刻で記録すると、フェッチ所要時間の分だけ
	// 次回 TTL 判定が後ろにずれ、設定間隔と同程度の遅延（例: 60秒設定で取得に数秒）でも
	// キャッシュが返り実効再取得間隔が約2倍になりうる。開始時刻基準なら所要時間に依存せず、
	// 設定間隔ごとに確実に再取得される。
	fetchStart := time.Now()
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
	lastStatusFetch = fetchStart
	statusMu.Unlock()
	handleStatusNotification(snap)
	return snap
}

// invalidateStatusCache forces the next getStatusSnapshot call to refetch.
func invalidateStatusCache() {
	statusMu.Lock()
	lastStatusFetch = time.Time{}
	statusMu.Unlock()
}
