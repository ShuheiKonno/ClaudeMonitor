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
	Topmost      bool `json:"topmost"`
	Transparent  bool `json:"transparent"`
	NotifyUsage  bool `json:"notifyUsage"`
	NotifyStatus bool `json:"notifyStatus"`

	// 5h 使用量通知の状態を再起動越しに保持する。
	// 同一ウィンドウ (= 同じ ResetsAt) なら既に通知した閾値を再通知しないため。
	Notify5hResetsAt time.Time `json:"notify5hResetsAt,omitempty"`
	Notified5h60    bool      `json:"notified5h60,omitempty"`
	Notified5h80    bool      `json:"notified5h80,omitempty"`
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
	c.NotifyStatus = true
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
