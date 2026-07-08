package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	Window struct {
		X      int32 `json:"x"`
		Y      int32 `json:"y"`
		Width  int32 `json:"width"`
		Height int32 `json:"height"`
		Saved  bool  `json:"saved"`
	} `json:"window"`
	Topmost          bool   `json:"topmost"`
	Transparent      bool   `json:"transparent"`
	NotifyUsage      bool   `json:"notifyUsage"`
	NotifyOverage    bool   `json:"notifyOverage"`
	NotifyStatus     bool   `json:"notifyStatus"`
	OverageTipFormat string `json:"overageTipFormat"` // "dollar" | "percent"

	// TraySplitDays は 7 日ウィンドウ配色のペース基準日数。
	// 7 = 7日フル（既定、現状動作）、5 = 5日で閾値到達（早期警告）、0 = 分割なし（固定 60/80%しきい値）。
	TraySplitDays int `json:"traySplitDays"`

	// ポーリング間隔（秒）。使用量取得と障害監視で個別に設定可能。
	// 範囲は [minPollSeconds, maxPollSeconds] にクランプされる（0/欠落は既定値扱い）。
	UsagePollSeconds  int `json:"usagePollSeconds"`
	StatusPollSeconds int `json:"statusPollSeconds"`

	// 5h 使用量通知の状態を再起動越しに保持する。
	// 同一ウィンドウ (= 同じ ResetsAt) なら既に通知した閾値を再通知しないため。
	Notify5hResetsAt  time.Time `json:"notify5hResetsAt,omitempty"`
	Notified5h60      bool      `json:"notified5h60,omitempty"`
	Notified5h80      bool      `json:"notified5h80,omitempty"`
	NotifiedOverage60 bool      `json:"notifiedOverage60,omitempty"`
	NotifiedOverage80 bool      `json:"notifiedOverage80,omitempty"`
	OverageResetsAt   time.Time `json:"overageResetsAt,omitempty"`
}

// ポーリング間隔の範囲（秒）。
// 60秒未満は通信負荷が増えるだけで 5h/7d ウィンドウの更新価値がほぼないため下限とする。
// 上限は実用的な範囲としての上限値。clampPollSeconds で範囲外・0・欠落を矯正し、
// Go の time.NewTicker / Ticker.Reset が間隔 <= 0 で panic するのを防ぐ。
const (
	minPollSeconds     = 60
	maxPollSeconds     = 3600
	defaultPollSeconds = 300
)

// clampPollSeconds は秒値を [minPollSeconds, maxPollSeconds] に収める。
// 0 や欠落（ゼロ値）は最小値ではなく既定値とみなす方が直感的だが、ここでは
// loadConfig のマージ前提で「present-and-0 / 範囲外」を安全な最小値へ寄せる。
func clampPollSeconds(v int) int {
	if v < minPollSeconds {
		return minPollSeconds
	}
	if v > maxPollSeconds {
		return maxPollSeconds
	}
	return v
}

// normalizeTraySplitDays は 7/5/0 以外の値（旧バージョンの欠損値や不正値）を既定の 7 に丸める。
func normalizeTraySplitDays(v int) int {
	switch v {
	case 0, 5, 7:
		return v
	default:
		return 7
	}
}

var (
	configMu   sync.Mutex
	configPath string
	config     Config
)

func defaultConfig() Config {
	var c Config
	c.Topmost = true
	c.NotifyUsage = true
	c.NotifyOverage = true
	c.NotifyStatus = true
	c.OverageTipFormat = "dollar"
	c.UsagePollSeconds = defaultPollSeconds
	c.StatusPollSeconds = defaultPollSeconds
	c.TraySplitDays = 7
	return c
}

func loadConfig() {
	configMu.Lock()
	defer configMu.Unlock()
	config = defaultConfig()
	if configPath == "" {
		return
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	tmp := config
	if err := json.Unmarshal(data, &tmp); err != nil {
		return
	}
	// 手編集や旧バージョンで present-and-0 / 範囲外になっていても安全側へ矯正する。
	tmp.UsagePollSeconds = clampPollSeconds(tmp.UsagePollSeconds)
	tmp.StatusPollSeconds = clampPollSeconds(tmp.StatusPollSeconds)
	tmp.TraySplitDays = normalizeTraySplitDays(tmp.TraySplitDays)
	config = tmp
}

func mutateConfig(f func(c *Config)) {
	configMu.Lock()
	defer configMu.Unlock()
	f(&config)
	if configPath == "" {
		return
	}
	data, err := json.MarshalIndent(&config, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	tmp := configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, configPath)
}

func snapshotConfig() Config {
	configMu.Lock()
	defer configMu.Unlock()
	return config
}
