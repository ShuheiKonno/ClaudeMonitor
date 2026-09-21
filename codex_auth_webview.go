package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
)

// Codex 用の補助 WebView。chatgpt.com の Cookie を専用 DataPath に保存し、
// Codex CLI を介さずにログインと使用量取得を行う。
const codexAuthWindowTitle = "ClaudeMonitor Codex Auth"

var (
	codexAuthWebViewHandle   uintptr
	codexAuthWebViewInst     webview2.WebView
	codexAuthWebViewVisible  atomic.Bool
	codexOrigAuthWndProc     uintptr
	codexAuthWndProcCallback = syscall.NewCallback(codexAuthWndProc)

	codexRefreshMu       sync.Mutex
	codexRefreshNotifyMu sync.Mutex
	codexRefreshNotify   chan struct{}
)

type rawCodexUsagePayload struct {
	Usage       codexUsageResponse `json:"usage"`
	Email       string             `json:"email"`
	DisplayName string             `json:"displayName"`
}

func codexAuthWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if msg == WM_CLOSE {
		if codexAuthWebViewVisible.CompareAndSwap(true, false) {
			moveCodexAuthOffscreenInline()
		}
		return 0
	}
	r, _, _ := procCallWindowProc.Call(codexOrigAuthWndProc, hwnd, msg, wParam, lParam)
	return r
}

func subclassCodexAuthWindow() {
	if codexAuthWebViewHandle == 0 {
		return
	}
	gwlpWndProc := int32(GWLP_WNDPROC)
	r, _, _ := procSetWindowLongPtr.Call(codexAuthWebViewHandle, uintptr(gwlpWndProc), codexAuthWndProcCallback)
	codexOrigAuthWndProc = r
}

func startCodexAuthWebView(dataPath string) {
	cw := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:    false,
		DataPath: dataPath,
		WindowOptions: webview2.WindowOptions{
			Title: codexAuthWindowTitle, Width: 500, Height: 700, Center: false,
		},
	})
	if cw == nil {
		fmt.Fprintln(os.Stderr, "[codex-auth] WebView 生成失敗")
		return
	}
	codexAuthWebViewInst = cw
	titlePtr, _ := syscall.UTF16PtrFromString(codexAuthWindowTitle)
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		fmt.Fprintln(os.Stderr, "[codex-auth] HWND 取得失敗")
		return
	}
	codexAuthWebViewHandle = hwnd
	moveCodexAuthOffscreenInline()
	subclassCodexAuthWindow()

	if err := cw.Bind("__postCodexUsageData", func(jsonStr string) {
		var p rawCodexUsagePayload
		if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
			updateCodexUsageError("network_error", fmt.Sprintf("レスポンス解析失敗: %v", err))
			signalCodexRefreshDone()
			return
		}
		applyCodexUsageIdentity(p.Usage, p.Email, p.DisplayName)
		if codexAuthWebViewVisible.Load() {
			hideCodexAuthWebView()
		}
		signalCodexRefreshDone()
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[codex-auth] Bind usage 失敗:", err)
	}
	if err := cw.Bind("__postCodexLoginRequired", func(reason string) {
		updateCodexUsageError("needs_login", reason)
		signalCodexRefreshDone()
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[codex-auth] Bind login 失敗:", err)
	}
	if err := cw.Bind("__postCodexFetchError", func(msg string) {
		updateCodexUsageError("network_error", msg)
		signalCodexRefreshDone()
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[codex-auth] Bind fetch 失敗:", err)
	}
	if err := cw.Bind("__getCodexUsageIntervalMs", func() int64 {
		return int64(usagePollInterval() / time.Millisecond)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[codex-auth] Bind interval 失敗:", err)
	}

	cw.Init(codexAuthFetcherScript)
	if snapshotConfig().CodexEnabled {
		cw.Navigate("https://chatgpt.com/codex/settings/usage")
	} else {
		cw.Navigate("about:blank")
	}
}

const codexAuthFetcherScript = `
(function() {
  function decodeJwtPayload(token) {
    try {
      const part = token.split('.')[1];
      if (!part) return {};
      const base64 = part.replace(/-/g, '+').replace(/_/g, '/');
      const padded = base64 + '='.repeat((4 - base64.length % 4) % 4);
      return JSON.parse(decodeURIComponent(Array.prototype.map.call(atob(padded), function(c) {
        return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
      }).join('')));
    } catch (_) {
      return {};
    }
  }

  async function fetchCodexUsage() {
    if (location.hostname !== 'chatgpt.com') return;
    try {
      // ChatGPT のログインCookieからWebセッションを取得する。wham APIはCookieだけでなく
      // Bearer tokenを要求するため、セッションのaccessTokenを同一オリジン内で利用する。
      const sessionR = await fetch('/api/auth/session', {credentials: 'include', headers: {'Accept': 'application/json'}});
      if (sessionR.status === 401 || sessionR.status === 403) {
        window.__postCodexLoginRequired && window.__postCodexLoginRequired('ChatGPT にログインしてください');
        return;
      }
      if (!sessionR.ok) {
        window.__postCodexFetchError && window.__postCodexFetchError('session fetch failed: status=' + sessionR.status);
        return;
      }
      const session = await sessionR.json();
      const accessToken = session.accessToken || session.access_token || '';
      if (!accessToken) {
        window.__postCodexLoginRequired && window.__postCodexLoginRequired('ChatGPT セッションを取得できません。再ログインしてください');
        return;
      }
      const claims = decodeJwtPayload(accessToken);
      const authClaims = claims['https://api.openai.com/auth'] || {};
      const accountId =
        (session.account && session.account.id) || session.accountId || session.account_id ||
        authClaims.chatgpt_account_id || authClaims.account_id || '';
      const usageHeaders = {'Accept': 'application/json', 'Authorization': 'Bearer ' + accessToken};
      if (accountId) usageHeaders['ChatGPT-Account-Id'] = accountId;
      const usageR = await fetch('/backend-api/wham/usage', {credentials: 'include', headers: usageHeaders});
      if (usageR.status === 401 || usageR.status === 403) {
        window.__postCodexLoginRequired && window.__postCodexLoginRequired('ChatGPT の認証期限が切れています。再ログインしてください');
        return;
      }
      if (!usageR.ok) {
        window.__postCodexFetchError && window.__postCodexFetchError('usage fetch failed: status=' + usageR.status);
        return;
      }
      const usage = await usageR.json();
      let email = (session.user && session.user.email) || claims.email || '';
      let displayName = (session.user && session.user.name) || claims.name || '';
      try {
        const meR = await fetch('/backend-api/me', {credentials: 'include', headers: {'Accept': 'application/json'}});
        if (meR.ok) {
          const me = await meR.json();
          email = email || me.email || (me.user && me.user.email) || '';
          displayName = displayName || me.name || (me.user && me.user.name) || '';
        }
      } catch (_) {}
      window.__postCodexUsageData && window.__postCodexUsageData(JSON.stringify({usage, email, displayName}));
    } catch (e) {
      window.__postCodexFetchError && window.__postCodexFetchError(String((e && e.message) || e));
    }
  }
  window.__fetchCodexUsage = fetchCodexUsage;
  setTimeout(fetchCodexUsage, 1500);
  let __codexUsageTimer = setInterval(fetchCodexUsage, 5 * 60 * 1000);
  window.__setCodexUsageInterval = function(ms) {
    if (__codexUsageTimer) clearInterval(__codexUsageTimer);
    __codexUsageTimer = setInterval(fetchCodexUsage, ms);
  };
  if (window.__getCodexUsageIntervalMs) {
    window.__getCodexUsageIntervalMs().then(function(ms) {
      if (ms > 0) window.__setCodexUsageInterval(ms);
    }).catch(function() {});
  }
})();
`

func moveCodexAuthOffscreenInline() {
	if codexAuthWebViewHandle == 0 {
		return
	}
	gwlExStyle := int32(GWL_EXSTYLE)
	exStyle, _, _ := procGetWindowLong.Call(codexAuthWebViewHandle, uintptr(gwlExStyle))
	newExStyle := (exStyle &^ WS_EX_APPWINDOW) | WS_EX_TOOLWINDOW
	procSetWindowLong.Call(codexAuthWebViewHandle, uintptr(gwlExStyle), newExStyle)
	procSetWindowPos.Call(codexAuthWebViewHandle, 0,
		uintptr(uint32(authOffscreenX)), uintptr(uint32(authOffscreenY)),
		500, 700, SWP_NOZORDER|SWP_FRAMECHANGED)
}

func showCodexAuthWebView() {
	if codexAuthWebViewHandle == 0 || codexAuthWebViewInst == nil {
		return
	}
	wasHidden := codexAuthWebViewVisible.CompareAndSwap(false, true)
	uiDispatch(func() {
		if !wasHidden {
			procSetForegroundWindow.Call(codexAuthWebViewHandle)
			return
		}
		gwlExStyle := int32(GWL_EXSTYLE)
		exStyle, _, _ := procGetWindowLong.Call(codexAuthWebViewHandle, uintptr(gwlExStyle))
		newExStyle := (exStyle &^ WS_EX_TOOLWINDOW) | WS_EX_APPWINDOW
		procSetWindowLong.Call(codexAuthWebViewHandle, uintptr(gwlExStyle), newExStyle)
		procSetWindowPos.Call(codexAuthWebViewHandle, 0, 220, 100, 800, 800, SWP_NOZORDER|SWP_FRAMECHANGED)
		procShowWindow.Call(codexAuthWebViewHandle, SW_SHOW)
		procSetForegroundWindow.Call(codexAuthWebViewHandle)
		codexAuthWebViewInst.Navigate("https://chatgpt.com/auth/login")
	})
}

func hideCodexAuthWebView() {
	if !codexAuthWebViewVisible.CompareAndSwap(true, false) {
		return
	}
	uiDispatch(moveCodexAuthOffscreenInline)
}

func refreshCodexUsage() {
	if !snapshotConfig().CodexEnabled || codexAuthWebViewInst == nil {
		return
	}
	codexRefreshMu.Lock()
	defer codexRefreshMu.Unlock()
	ch := make(chan struct{}, 1)
	codexRefreshNotifyMu.Lock()
	codexRefreshNotify = ch
	codexRefreshNotifyMu.Unlock()
	defer func() {
		codexRefreshNotifyMu.Lock()
		codexRefreshNotify = nil
		codexRefreshNotifyMu.Unlock()
	}()
	uiDispatch(func() {
		codexAuthWebViewInst.Eval("window.__fetchCodexUsage && window.__fetchCodexUsage()")
	})
	select {
	case <-ch:
	case <-time.After(15 * time.Second):
		updateCodexUsageError("network_error", "fetch timeout")
	}
}

func signalCodexRefreshDone() {
	codexRefreshNotifyMu.Lock()
	ch := codexRefreshNotify
	codexRefreshNotifyMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

// applyProviderSelection は設定変更後、無効側を休止し有効側の取得を開始する。
func applyProviderSelection() {
	cfg := snapshotConfig()
	uiDispatch(func() {
		if authWebViewInst != nil {
			if cfg.ClaudeEnabled {
				authWebViewInst.Navigate("https://claude.ai/settings/usage")
			} else {
				authWebViewInst.Navigate("about:blank")
				if authWebViewVisible.CompareAndSwap(true, false) {
					moveAuthOffscreenInline()
				}
			}
		}
		if codexAuthWebViewInst != nil {
			if cfg.CodexEnabled {
				codexAuthWebViewInst.Navigate("https://chatgpt.com/codex/settings/usage")
			} else {
				codexAuthWebViewInst.Navigate("about:blank")
				if codexAuthWebViewVisible.CompareAndSwap(true, false) {
					moveCodexAuthOffscreenInline()
				}
			}
		}
	})
}
