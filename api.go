package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"unsafe"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

type settingsPayload struct {
	Plan         string `json:"plan"`
	TokenLimit5h int64  `json:"tokenLimit5h"`
	TokenLimit7d int64  `json:"tokenLimit7d"`
	Topmost      bool   `json:"topmost"`
	Transparent  bool   `json:"transparent"`
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
		writeJSON(w, getUsageSnapshot())
	})

	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		cfg := snapshotConfig()
		writeJSON(w, settingsPayload{
			Plan:         cfg.Plan,
			TokenLimit5h: cfg.TokenLimit5h,
			TokenLimit7d: cfg.TokenLimit7d,
			Topmost:      cfg.Topmost,
			Transparent:  cfg.Transparent,
		})
	})

	mux.HandleFunc("/api/setoption", func(w http.ResponseWriter, r *http.Request) {
		var p settingsPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mutateConfig(func(c *Config) {
			if p.Plan != "" {
				c.Plan = p.Plan
			}
			c.TokenLimit5h = p.TokenLimit5h
			c.TokenLimit7d = p.TokenLimit7d
			c.Topmost = p.Topmost
			c.Transparent = p.Transparent
		})
		setTopmost(p.Topmost)
		setTransparent(p.Transparent)
		refreshUsage()
		updateTrayFromSnapshot()
		writeJSON(w, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshUsage()
		updateTrayFromSnapshot()
		writeJSON(w, getUsageSnapshot())
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

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "server error:", err)
		}
	}()
	return port, nil
}
