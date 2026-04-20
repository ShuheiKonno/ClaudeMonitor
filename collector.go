package main

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type WindowSummary struct {
	Tokens           int64 `json:"tokens"`
	InputTokens      int64 `json:"inputTokens"`
	OutputTokens     int64 `json:"outputTokens"`
	CacheTokens      int64 `json:"cacheTokens"`
	CacheReadTokens  int64 `json:"cacheReadTokens"`
	Messages         int64 `json:"messages"`
	Sessions         int64 `json:"sessions"`
	LimitTokens      int64 `json:"limitTokens"`
	LimitMessages    int64 `json:"limitMessages"`
}

type UsageSnapshot struct {
	FiveHour  WindowSummary `json:"fiveHour"`
	SevenDay  WindowSummary `json:"sevenDay"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Plan      string        `json:"plan"`
}

type StatsCache struct {
	DailyActivity []struct {
		Date         string `json:"date"`
		MessageCount int64  `json:"messageCount"`
		SessionCount int64  `json:"sessionCount"`
	} `json:"dailyActivity"`
}

type logEntry struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"sessionId"`
	Message   *struct {
		ID    string `json:"id"`
		Role  string `json:"role"`
		Usage *struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type usageEvent struct {
	Timestamp time.Time
	SessionID string
	MessageID string   // 重複除外キー（アシスタント用）
	IsUser    bool
	Usage     struct { // アシスタント用
		Input, Output, CacheCreate, CacheRead int64
	}
}

var (
	usageMu        sync.RWMutex
	cachedUsage    UsageSnapshot
	projectsDir    string
	statsCachePath string
)

func initPaths() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	projectsDir = filepath.Join(home, ".claude", "projects")
	statsCachePath = filepath.Join(home, ".claude", "stats-cache.json")
}

// scanOne は1つの JSONL ファイルを読み、since 以降のイベントを返す。
// defer による close がファイル単位で発火するよう、走査処理から分離している。
func scanOne(path string, since time.Time) []usageEvent {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var events []usageEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var e logEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if e.Timestamp.IsZero() || e.Timestamp.Before(since) {
			continue
		}
		ev := usageEvent{Timestamp: e.Timestamp, SessionID: e.SessionID}
		switch e.Type {
		case "user":
			ev.IsUser = true
			events = append(events, ev)
		case "assistant":
			if e.Message == nil || e.Message.Usage == nil {
				continue
			}
			ev.MessageID = e.Message.ID
			ev.Usage.Input = e.Message.Usage.InputTokens
			ev.Usage.Output = e.Message.Usage.OutputTokens
			ev.Usage.CacheCreate = e.Message.Usage.CacheCreationInputTokens
			ev.Usage.CacheRead = e.Message.Usage.CacheReadInputTokens
			events = append(events, ev)
		}
	}
	return events
}

// scanProjectLogs は projects/*/*.jsonl を走査し、指定期間内のイベントを抽出する。
// 大きな JSONL を想定して行ストリーミングで読み、過去7日より古いイベントは早期破棄する。
func scanProjectLogs(since time.Time) []usageEvent {
	if projectsDir == "" {
		return nil
	}
	var events []usageEvent
	_ = filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}
		// ファイルの mtime が since より前ならスキップ（中身を開かない）
		info, err := d.Info()
		if err == nil && info.ModTime().Before(since) {
			return nil
		}
		events = append(events, scanOne(path, since)...)
		return nil
	})
	return events
}

func aggregateWindow(events []usageEvent, since time.Time) WindowSummary {
	var s WindowSummary
	seenMsg := make(map[string]bool)
	sessions := make(map[string]bool)
	for _, e := range events {
		if e.Timestamp.Before(since) {
			continue
		}
		sessions[e.SessionID] = true
		if e.IsUser {
			s.Messages++
			continue
		}
		// アシスタントイベントは同一 message.id で複数回記録されることがある（thinking + tool_use 等）ので重複除外
		if e.MessageID != "" {
			if seenMsg[e.MessageID] {
				continue
			}
			seenMsg[e.MessageID] = true
		}
		s.InputTokens += e.Usage.Input
		s.OutputTokens += e.Usage.Output
		s.CacheTokens += e.Usage.CacheCreate
		s.CacheReadTokens += e.Usage.CacheRead
	}
	// cache_read はコストが input の約1/10 でリミット計測対象外のため合算から除外
	s.Tokens = s.InputTokens + s.OutputTokens + s.CacheTokens
	s.Sessions = int64(len(sessions))
	return s
}

// planPresetLimits はプラン名に対応するプリセット上限を返す。
// トークン値は Claude Desktop の表示比率から逆算した実測相当値。
func planPresetLimits(plan string) (tok5h, tok7d, msg5h, msg7d int64) {
	switch plan {
	case "pro":
		return 500000, 2000000, 45, 300
	case "max100":
		return 2500000, 10000000, 225, 1500
	case "max200":
		return 9500000, 35000000, 900, 6000
	}
	return
}

// estimateLimitsAuto は過去30日の最大日次メッセージ数からプランを推定し、
// 該当プリセット値を返す。閾値はコミュニティ観測値。
func estimateLimitsAuto() (tok5h, tok7d, msg5h, msg7d int64) {
	if statsCachePath == "" {
		return
	}
	data, err := os.ReadFile(statsCachePath)
	if err != nil {
		return
	}
	var sc StatsCache
	if err := json.Unmarshal(data, &sc); err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	var maxMsgPerDay int64
	for _, d := range sc.DailyActivity {
		if d.Date < cutoff {
			continue
		}
		if d.MessageCount > maxMsgPerDay {
			maxMsgPerDay = d.MessageCount
		}
	}
	switch {
	case maxMsgPerDay > 500:
		return planPresetLimits("max200")
	case maxMsgPerDay > 100:
		return planPresetLimits("max100")
	case maxMsgPerDay > 0:
		return planPresetLimits("pro")
	}
	return
}

func computeLimits(cfg Config) (tok5h, tok7d, msg5h, msg7d int64) {
	// 1. 手動上書きが優先
	tok5h = cfg.TokenLimit5h
	tok7d = cfg.TokenLimit7d
	msg5h = cfg.MsgLimit5h
	msg7d = cfg.MsgLimit7d

	// 2. プランプリセット
	if cfg.Plan != "auto" && cfg.Plan != "custom" && cfg.Plan != "" {
		pTok5h, pTok7d, pMsg5h, pMsg7d := planPresetLimits(cfg.Plan)
		if tok5h == 0 {
			tok5h = pTok5h
		}
		if tok7d == 0 {
			tok7d = pTok7d
		}
		if msg5h == 0 {
			msg5h = pMsg5h
		}
		if msg7d == 0 {
			msg7d = pMsg7d
		}
	}

	// 3. 自動推定にフォールバック
	aTok5h, aTok7d, aMsg5h, aMsg7d := estimateLimitsAuto()
	if tok5h == 0 {
		tok5h = aTok5h
	}
	if tok7d == 0 {
		tok7d = aTok7d
	}
	if msg5h == 0 {
		msg5h = aMsg5h
	}
	if msg7d == 0 {
		msg7d = aMsg7d
	}
	return
}

func refreshUsage() {
	now := time.Now()
	since7d := now.AddDate(0, 0, -7)
	since5h := now.Add(-5 * time.Hour)

	events := scanProjectLogs(since7d)
	w7d := aggregateWindow(events, since7d)
	w5h := aggregateWindow(events, since5h)

	cfg := snapshotConfig()
	tok5h, tok7d, msg5h, msg7d := computeLimits(cfg)
	w5h.LimitTokens = tok5h
	w5h.LimitMessages = msg5h
	w7d.LimitTokens = tok7d
	w7d.LimitMessages = msg7d

	snap := UsageSnapshot{
		FiveHour:  w5h,
		SevenDay:  w7d,
		UpdatedAt: now,
		Plan:      cfg.Plan,
	}
	usageMu.Lock()
	cachedUsage = snap
	usageMu.Unlock()
}

func startCollector() {
	initPaths()
	refreshUsage()
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			refreshUsage()
		}
	}()
}

func getUsageSnapshot() UsageSnapshot {
	usageMu.RLock()
	defer usageMu.RUnlock()
	return cachedUsage
}
