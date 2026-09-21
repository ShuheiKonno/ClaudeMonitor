package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// openExternalURL launches the default browser for the given URL via rundll32.
// CREATE_NO_WINDOW を指定しコンソールの瞬間表示を抑止する。
func openExternalURL(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_ = cmd.Start()
}

type settingsPayload struct {
	Topmost           bool   `json:"topmost"`
	Transparent       bool   `json:"transparent"`
	NotifyUsage       bool   `json:"notifyUsage"`
	NotifyOverage     bool   `json:"notifyOverage"`
	NotifyStatus      bool   `json:"notifyStatus"`
	OverageTipFormat  string `json:"overageTipFormat"`
	ResetTimeFormat   string `json:"resetTimeFormat"`
	UsagePollSeconds  int    `json:"usagePollSeconds"`
	StatusPollSeconds int    `json:"statusPollSeconds"`
	TraySplitDays     int    `json:"traySplitDays"`
	ActiveProvider    string `json:"activeProvider"`
	LayoutMode        string `json:"layoutMode"`
}

func startServer() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(getHTML()))
	})

	mux.HandleFunc("/api/usage", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, getAllUsageSnapshots())
	})

	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		cfg := snapshotConfig()
		writeJSON(w, settingsPayload{
			Topmost:           cfg.Topmost,
			Transparent:       cfg.Transparent,
			NotifyUsage:       cfg.NotifyUsage,
			NotifyOverage:     cfg.NotifyOverage,
			NotifyStatus:      cfg.NotifyStatus,
			OverageTipFormat:  cfg.OverageTipFormat,
			ResetTimeFormat:   cfg.ResetTimeFormat,
			UsagePollSeconds:  cfg.UsagePollSeconds,
			StatusPollSeconds: cfg.StatusPollSeconds,
			TraySplitDays:     cfg.TraySplitDays,
			ActiveProvider:    cfg.ActiveProvider,
			LayoutMode:        cfg.LayoutMode,
		})
	})

	mux.HandleFunc("/api/setoption", func(w http.ResponseWriter, r *http.Request) {
		var p settingsPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// ポーリング間隔はサーバ側クランプを正とする（panic 防止 + UX バリデーションの最終防衛線）。
		usageSec := clampPollSeconds(p.UsagePollSeconds)
		statusSec := clampPollSeconds(p.StatusPollSeconds)
		splitDays := normalizeTraySplitDays(p.TraySplitDays)
		resetFmt := normalizeResetTimeFormat(p.ResetTimeFormat)
		layoutMode := normalizeLayoutMode(p.LayoutMode)
		mutateConfig(func(c *Config) {
			c.Topmost = p.Topmost
			c.Transparent = p.Transparent
			c.NotifyUsage = p.NotifyUsage
			c.NotifyOverage = p.NotifyOverage
			c.NotifyStatus = p.NotifyStatus
			c.OverageTipFormat = p.OverageTipFormat
			c.ResetTimeFormat = resetFmt
			c.UsagePollSeconds = usageSec
			c.StatusPollSeconds = statusSec
			c.TraySplitDays = splitDays
			c.LayoutMode = layoutMode
		})
		setTopmost(p.Topmost)
		setTransparent(p.Transparent)
		applyUsagePollInterval(usageSec)
		applyWindowLayout(layoutMode)
		// 障害監視間隔は statusCacheTTL() が動的に config を読むため Go 側追加処理は不要。
		// JS 側が戻り値を使って setInterval を張り直す。
		p.UsagePollSeconds = usageSec
		p.StatusPollSeconds = statusSec
		p.TraySplitDays = splitDays
		p.ResetTimeFormat = resetFmt
		p.LayoutMode = layoutMode
		writeJSON(w, p)
	})

	mux.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) {
		// 同期でフェッチを実行し、完了後の最新 snapshot を返す。
		// 「更新」ボタンを押した直後に古い値を返さないために重要。
		// refreshMu により並行呼び出しは直列化（重複フェッチ防止）。
		refreshAllUsage()
		// ステータスキャッシュも失効させ、次の /api/status で再フェッチさせる。
		invalidateStatusCache()
		writeJSON(w, getAllUsageSnapshots())
	})

	mux.HandleFunc("/api/provider", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if p.Provider != "claude" && p.Provider != "codex" {
			http.Error(w, "invalid provider", http.StatusBadRequest)
			return
		}
		mutateConfig(func(c *Config) { c.ActiveProvider = p.Provider })
		updateTrayFromSnapshot()
		writeJSON(w, map[string]any{"ok": true, "provider": p.Provider})
	})

	mux.HandleFunc("/api/close", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"ok": true})
		// 完全終了ではなくタスクトレイへしまう。終了はトレイ右クリックメニューから
		go hideMainWindow()
	})

	// --- ウィンドウドラッグ ---
	mux.HandleFunc("/api/dragstart", func(w http.ResponseWriter, r *http.Request) {
		if windowHandle == 0 {
			writeJSON(w, map[string]any{"ok": false})
			return
		}
		var cp POINT
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&cp)))
		var rc RECT
		procGetWindowRect.Call(windowHandle, uintptr(unsafe.Pointer(&rc)))
		dragMu.Lock()
		dragStartCurX = cp.X
		dragStartCurY = cp.Y
		dragStartWinX = rc.Left
		dragStartWinY = rc.Top
		dragMu.Unlock()
		writeJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/dragmove", func(w http.ResponseWriter, r *http.Request) {
		if windowHandle == 0 {
			writeJSON(w, map[string]any{"ok": false})
			return
		}
		var cp POINT
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&cp)))
		dragMu.Lock()
		newX := dragStartWinX + (cp.X - dragStartCurX)
		newY := dragStartWinY + (cp.Y - dragStartCurY)
		dragMu.Unlock()
		procSetWindowPos.Call(windowHandle, 0, uintptr(newX), uintptr(newY), 0, 0, SWP_NOSIZE|SWP_NOZORDER)
		writeJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/persistwindow", func(w http.ResponseWriter, r *http.Request) {
		persistCurrentWindow()
		writeJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/open-usage", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "codex" {
			go openExternalURL("https://chatgpt.com/codex/settings/usage")
		} else {
			go openExternalURL("https://claude.ai/settings/usage")
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/open-status", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Query().Get("provider")
		if provider == "codex" {
			go openExternalURL("https://status.openai.com/")
		} else {
			go openExternalURL("https://status.claude.com/")
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// 補助 WebView をオンスクリーンに復帰させ、claude.ai のログインページを表示する。
	// バナーの「ログイン」ボタン or トレイの「Claude にログイン…」から呼ばれる。
	mux.HandleFunc("/api/relogin", func(w http.ResponseWriter, r *http.Request) {
		go showAuthWebView()
		writeJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, getAllStatusSnapshots())
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "server error:", err)
		}
	}()
	return port, nil
}
