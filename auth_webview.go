package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
)

// === claude.ai 用補助 WebView ===
//
// 主 UI 用 WebView (127.0.0.1) とは別ウィンドウとして claude.ai を抱える。
// 通常は -32000,-32000 のオフスクリーン位置に置き、JS を Eval して
// /api/organizations/{orgId}/usage と /edge-api/bootstrap/.../app_start を叩く。
// Cookie はこの WebView の DataPath に保持されるため、ユーザーは初回のみ
// アプリ内でログインすれば以降は自動再認証される。
//
// 設計判断:
//   - SW_HIDE は使わずオフスクリーンに置く: 隠し窓だと WebView2 が
//     setInterval / requestAnimationFrame をスロットルする可能性があるため
//   - WS_EX_TOOLWINDOW でタスクバーから消す
//   - ログイン要求時のみオンスクリーンへ復帰 (showAuthWebView)

const (
	authWindowTitle = "ClaudeMonitor Auth"

	WS_EX_APPWINDOW  = 0x00040000
	WS_EX_TOOLWINDOW = 0x00000080

	WM_CLOSE     = 0x0010
	GWLP_WNDPROC = -4
)

// 定数で書くと const 評価で uintptr オーバーフローになるため var で保持。
var (
	authOffscreenX int32 = -32000
	authOffscreenY int32 = -32000

	procSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc   = user32.NewProc("CallWindowProcW")

	origAuthWndProc     uintptr
	authWndProcCallback = syscall.NewCallback(authWndProc)
)

// authWndProc は補助 WebView ウィンドウのサブクラス化プロシージャ。
// Xボタンクリック (WM_CLOSE) でアプリ全体が終了するのを防ぎ、非表示処理に置き換える。
// それ以外のメッセージは元のプロシージャに委譲する。
func authWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if msg == WM_CLOSE {
		if authWebViewVisible.CompareAndSwap(true, false) {
			moveAuthOffscreenInline()
		}
		return 0
	}
	r, _, _ := procCallWindowProc.Call(origAuthWndProc, hwnd, msg, wParam, lParam)
	return r
}

// subclassAuthWindow は補助 WebView ウィンドウをサブクラス化して WM_CLOSE をフックする。
// startAuthWebView の HWND 取得後に呼び出す。
func subclassAuthWindow() {
	if authWebViewHandle == 0 {
		return
	}
	gwlpWndProc := int32(GWLP_WNDPROC)
	r, _, _ := procSetWindowLongPtr.Call(authWebViewHandle, uintptr(gwlpWndProc), authWndProcCallback)
	origAuthWndProc = r
}

var (
	authWebViewHandle  uintptr
	authWebViewInst    webview2.WebView
	authWebViewVisible atomic.Bool
	authDataPath       string

	debugLogPath string

	// mainWebViewInst は主 UI WebView。auth 側で Dispatch すると関数が
	// 実行されない (go-webview2 の Run ループは自分の dispatchq だけを
	// ドレインするため) ので、UI スレッドへ寄せる用途は全てこちらの
	// Dispatch を使う。両 WebView は同一スレッドにあるので、main 経由で
	// auth の Win32 / Eval / Navigate を呼ぶのは安全。
	mainWebViewInst webview2.WebView
)

// debugLog はAPIレスポンス調査用の一時デバッグログ。
// %LOCALAPPDATA%\ClaudeMonitor\debug.log に追記する。
func debugLog(msg string) {
	if debugLogPath == "" {
		return
	}
	f, err := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

// uiDispatch は UI (主 WebView) スレッドへ関数を寄せる。
// 主 WebView 未生成 (起動最序盤) の場合はインライン実行する
// (起動経路では既に主 goroutine 上にいるため安全)。
func uiDispatch(f func()) {
	if mainWebViewInst == nil {
		f()
		return
	}
	mainWebViewInst.Dispatch(f)
}

// rawClaudeUsagePayload は JS が JSON.stringify して Bind に渡してくる構造体。
// claude.ai の /usage と /edge-api/bootstrap/.../app_start から JS 側で抽出済み。
type rawClaudeUsagePayload struct {
	FiveHour      *rawClaudeWindow   `json:"fiveHour"`
	SevenDay      *rawClaudeWindow   `json:"sevenDay"`
	Overage       *rawOveragePayload `json:"overage"`
	Email         string             `json:"email"`
	DisplayName   string             `json:"displayName"`
	Capabilities  []string           `json:"capabilities"`
	RateLimitTier string             `json:"rateLimitTier"`
	BillingType   string             `json:"billingType"`
}

type rawClaudeWindow struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    *string `json:"resetsAt"`
}

type rawOveragePayload struct {
	AmountUsed    float64  `json:"amountUsed"`
	SpendingLimit *float64 `json:"spendingLimit"`
	ResetsAt      *string  `json:"resetsAt"`
}

// startAuthWebView は補助 WebView を生成し、オフスクリーン配置 + JS 注入を行う。
// 必ず w.Run() を呼ぶ前 (LockOSThread 済み主 goroutine) に呼ぶこと。
func startAuthWebView(dataPath string) {
	authDataPath = dataPath
	aw := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:    false,
		DataPath: dataPath,
		WindowOptions: webview2.WindowOptions{
			Title:  authWindowTitle,
			Width:  500,
			Height: 700,
			Center: false,
		},
	})
	if aw == nil {
		fmt.Fprintln(os.Stderr, "[auth] WebView 生成失敗")
		return
	}
	authWebViewInst = aw

	titlePtr, _ := syscall.UTF16PtrFromString(authWindowTitle)
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		fmt.Fprintln(os.Stderr, "[auth] HWND 取得失敗")
		return
	}
	authWebViewHandle = hwnd

	// この呼び出しは主 goroutine (LockOSThread 済み) から起きるため、
	// Dispatch を経由せず直接 Win32 を叩いてよい。これで起動時に
	// auth ウィンドウが一瞬画面に出る "フラッシュ" を回避できる。
	moveAuthOffscreenInline()

	// Xボタン (WM_CLOSE) によるアプリ終了を防ぐためウィンドウプロシージャをサブクラス化する。
	subclassAuthWindow()

	if err := aw.Bind("__debugLog", func(msg string) {
		debugLog(msg)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[auth] Bind __debugLog 失敗:", err)
	}
	if err := aw.Bind("__postUsageData", func(jsonStr string) {
		var p rawClaudeUsagePayload
		if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
			updateUsageError("network_error", fmt.Sprintf("レスポンスパース失敗: %v", err))
			signalRefreshDone()
			return
		}
		applyUsagePayload(p)
		signalRefreshDone()
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[auth] Bind __postUsageData 失敗:", err)
	}
	if err := aw.Bind("__postLoginRequired", func(reason string) {
		updateUsageError("needs_login", reason)
		signalRefreshDone()
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[auth] Bind __postLoginRequired 失敗:", err)
	}
	if err := aw.Bind("__postFetchError", func(msg string) {
		updateUsageError("network_error", msg)
		signalRefreshDone()
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[auth] Bind __postFetchError 失敗:", err)
	}
	// バックアップタイマーの間隔（ミリ秒）を JS から問い合わせるための Bind。
	// Init スクリプトはドキュメント生成（再ナビゲーション含む）毎に再実行されるため、
	// 固定値を埋め込むと再ログイン後に起動時の値へ戻ってしまう。常に最新の設定値を
	// 返すことで、新規ドキュメントでも変更後の間隔を維持する。
	if err := aw.Bind("__getUsageIntervalMs", func() int64 {
		return int64(usagePollInterval() / time.Millisecond)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "[auth] Bind __getUsageIntervalMs 失敗:", err)
	}

	aw.Init(authFetcherScript)
	aw.Navigate("https://claude.ai/settings/usage")
}

// authFetcherScript は claude.ai のページコンテキストで動く取得ループ。
// __fetchClaudeUsage を window に公開し、Go 側から Eval で呼び出せるようにする。
const authFetcherScript = `
(function() {
  async function fetchClaudeUsage() {
    try {
      const orgMatch = document.cookie.match(/lastActiveOrg=([^;]+)/);
      if (!orgMatch) {
        // 通常はログインページにリダイレクトされている状態
        window.__postLoginRequired && window.__postLoginRequired('lastActiveOrg cookie 未取得 (未ログイン)');
        return;
      }
      const orgId = decodeURIComponent(orgMatch[1]);
      const headers = {'Content-Type': 'application/json'};
      const [usageR, bootR] = await Promise.all([
        fetch('/api/organizations/' + orgId + '/usage', {credentials: 'include', headers}),
        fetch('/edge-api/bootstrap/' + orgId + '/app_start?statsig_hashing_algorithm=djb2&growthbook_format=sdk&include_system_prompts=false', {credentials: 'include', headers})
      ]);
      if (usageR.status === 401 || usageR.status === 403) {
        window.__postLoginRequired && window.__postLoginRequired('Cookie 失効 (status=' + usageR.status + ')');
        return;
      }
      if (!usageR.ok) {
        window.__postFetchError && window.__postFetchError('usage fetch failed: status=' + usageR.status);
        return;
      }
      const usage = await usageR.json();
      let email = '', display = '', caps = [], tier = '', billing = '';
      if (bootR.ok) {
        try {
          const boot = await bootR.json();
          email = (boot.account && boot.account.email_address) || '';
          display = (boot.account && (boot.account.display_name || boot.account.full_name)) || '';
          const memberships = (boot.account && boot.account.memberships) || [];
          const m = memberships.find(function(x) { return x && x.organization && x.organization.uuid === orgId; });
          if (m && m.organization) {
            caps = m.organization.capabilities || [];
            tier = m.organization.rate_limit_tier || '';
            billing = m.organization.billing_type || '';
          }
        } catch (e) { /* bootstrap パース失敗は致命傷ではない */ }
      }

      // 追加使用量（従量課金 / extra_usage）を抽出する。
      // /api/organizations/{orgId}/usage の extra_usage は次の構造:
      //   {is_enabled, monthly_limit, used_credits, utilization, currency}
      // ・used_credits / monthly_limit はセント単位 (347 = $3.47)
      // ・monthly_limit が 0 / null の場合は「上限なし（無制限）」
      // ・resets_at フィールドは無いため月初リセットを Go 側で計算する
      let overage = null;
      const eu = usage.extra_usage;
      if (eu && typeof eu === 'object' && eu.is_enabled && typeof eu.used_credits === 'number') {
        const amount = eu.used_credits / 100;
        let limit = null;
        if (typeof eu.monthly_limit === 'number' && eu.monthly_limit > 0) {
          limit = eu.monthly_limit / 100;
        }
        // API はリセット日を返さないため、月初 (UTC) を計算で入れる。
        const now = new Date();
        const reset = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 1)).toISOString();
        overage = {amountUsed: amount, spendingLimit: limit, resetsAt: reset};
      }

      const payload = {
        fiveHour: usage.five_hour ? {utilization: usage.five_hour.utilization, resetsAt: usage.five_hour.resets_at} : null,
        sevenDay: usage.seven_day ? {utilization: usage.seven_day.utilization, resetsAt: usage.seven_day.resets_at} : null,
        overage: overage,
        email: email,
        displayName: display,
        capabilities: caps,
        rateLimitTier: tier,
        billingType: billing
      };
      window.__postUsageData && window.__postUsageData(JSON.stringify(payload));
    } catch (e) {
      window.__postFetchError && window.__postFetchError(String((e && e.message) || e));
    }
  }
  window.__fetchClaudeUsage = fetchClaudeUsage;
  // ナビゲーション直後はリダイレクトと cookie 反映を待つために 1.5s 遅延
  setTimeout(fetchClaudeUsage, 1500);
  // バックアップタイマー (Go 側 ticker と二重保険)。
  // まず安全な既定値で起動し、直後に Go へ最新の設定間隔を問い合わせて上書きする。
  // この Init はドキュメント生成（再ナビゲーション含む）毎に走るため、再ログイン後も
  // 常に最新値へ収束する。設定変更時は Go が __setUsageInterval(ms) を Eval で呼ぶ。
  let __usageTimer = setInterval(fetchClaudeUsage, 5 * 60 * 1000);
  window.__setUsageInterval = function(ms) {
    if (__usageTimer) clearInterval(__usageTimer);
    __usageTimer = setInterval(fetchClaudeUsage, ms);
  };
  if (window.__getUsageIntervalMs) {
    window.__getUsageIntervalMs().then(function(ms) {
      if (ms > 0) window.__setUsageInterval(ms);
    }).catch(function() {});
  }
})();
`

// moveAuthOffscreenInline は補助ウィンドウをタスクバーから消し、画面外へ移動する。
// Win32 ウィンドウ操作は WebView UI スレッド上で同期実行する。
// startAuthWebView の初期化時など、既に主 goroutine にいる場面で使用する。
func moveAuthOffscreenInline() {
	if authWebViewHandle == 0 {
		return
	}
	gwlExStyle := int32(GWL_EXSTYLE)
	exStyle, _, _ := procGetWindowLong.Call(authWebViewHandle, uintptr(gwlExStyle))
	newExStyle := (exStyle &^ WS_EX_APPWINDOW) | WS_EX_TOOLWINDOW
	procSetWindowLong.Call(authWebViewHandle, uintptr(gwlExStyle), newExStyle)
	procSetWindowPos.Call(authWebViewHandle, 0,
		uintptr(uint32(authOffscreenX)), uintptr(uint32(authOffscreenY)),
		500, 700, SWP_NOZORDER|SWP_FRAMECHANGED)
}

// moveAuthOffscreen は別スレッドからの呼び出し向け。UI スレッドに寄せる。
func moveAuthOffscreen() {
	uiDispatch(moveAuthOffscreenInline)
}

// showAuthWebView はログインを促すために補助ウィンドウをオンスクリーンへ復帰させる。
// 既に表示中なら前面に上げるだけ。Win32 呼び出しは UI スレッドに寄せる。
func showAuthWebView() {
	if authWebViewHandle == 0 || authWebViewInst == nil {
		return
	}
	wasHidden := authWebViewVisible.CompareAndSwap(false, true)
	uiDispatch(func() {
		if !wasHidden {
			procSetForegroundWindow.Call(authWebViewHandle)
			return
		}
		gwlExStyle := int32(GWL_EXSTYLE)
		exStyle, _, _ := procGetWindowLong.Call(authWebViewHandle, uintptr(gwlExStyle))
		newExStyle := (exStyle &^ WS_EX_TOOLWINDOW) | WS_EX_APPWINDOW
		procSetWindowLong.Call(authWebViewHandle, uintptr(gwlExStyle), newExStyle)
		procSetWindowPos.Call(authWebViewHandle, 0, 200, 100, 800, 800, SWP_NOZORDER|SWP_FRAMECHANGED)
		procShowWindow.Call(authWebViewHandle, SW_SHOW)
		procSetForegroundWindow.Call(authWebViewHandle)
		authWebViewInst.Navigate("https://claude.ai/login")
	})
}

// logoutUser はHttpOnly Cookieを含む全セッションをクリアするため、
// 認証データディレクトリの削除を新プロセスに委ねてアプリを再起動する。
// WebView2が動いている間はデータディレクトリをロックするため、
// 旧プロセス側では削除できず、新プロセスが旧プロセス終了後に削除する。
func logoutUser() {
	go func() {
		exe, err := os.Executable()
		if err == nil {
			exec.Command(exe, "--restarted").Start()
		}
		removeTrayIcon()
		os.Exit(0)
	}()
}

// hideAuthWebView はログイン完了後にオフスクリーンへ戻す。
func hideAuthWebView() {
	if authWebViewHandle == 0 {
		return
	}
	if !authWebViewVisible.CompareAndSwap(true, false) {
		return
	}
	moveAuthOffscreen()
}

// applyUsagePayload は JS から受け取った正常レスポンスをスナップショットへ反映する。
func applyUsagePayload(p rawClaudeUsagePayload) {
	snap := UsageSnapshot{
		FiveHour:         mapClaudeWindow(p.FiveHour),
		SevenDay:         mapClaudeWindow(p.SevenDay),
		Overage:          mapOverage(p.Overage),
		Email:            p.Email,
		DisplayName:      p.DisplayName,
		SubscriptionType: deriveSubscriptionType(p.Capabilities, p.RateLimitTier),
		AuthState:        "ok",
		UpdatedAt:        time.Now(),
	}
	usageMu.Lock()
	cachedUsage = snap
	usageMu.Unlock()

	if authWebViewVisible.Load() {
		hideAuthWebView()
	}
	updateTrayFromSnapshot()
	handleUsageNotification(snap)
	handleOverageNotification(snap)
	// 主 UI のポーリング待ち (最大60秒) を回避するため、Eval で即時更新を促す。
	// 同時に topmost / transparent をログイン直後に再適用する
	// (WebView2 初期化やナビゲーションで z-order が外れるケースの保険)。
	if mainWebViewInst != nil {
		uiDispatch(func() {
			mainWebViewInst.Eval("if (typeof fetchUsage === 'function') fetchUsage();")
			c := snapshotConfig()
			setTopmost(c.Topmost)
			setTransparent(c.Transparent)
		})
	}
}

func mapClaudeWindow(w *rawClaudeWindow) UsageWindow {
	if w == nil {
		return UsageWindow{}
	}
	out := UsageWindow{Utilization: w.Utilization}
	if w.ResetsAt != nil && *w.ResetsAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, *w.ResetsAt); err == nil {
			out.ResetsAt = &t
		} else if t, err := time.Parse(time.RFC3339, *w.ResetsAt); err == nil {
			out.ResetsAt = &t
		}
	}
	return out
}

func mapOverage(o *rawOveragePayload) *OverageInfo {
	if o == nil {
		return nil
	}
	ov := &OverageInfo{
		AmountUsed:    o.AmountUsed,
		SpendingLimit: o.SpendingLimit,
	}
	if o.ResetsAt != nil && *o.ResetsAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, *o.ResetsAt); err == nil {
			ov.ResetsAt = &t
		} else if t, err := time.Parse(time.RFC3339, *o.ResetsAt); err == nil {
			ov.ResetsAt = &t
		}
	}
	return ov
}

// deriveSubscriptionType は capabilities と rate_limit_tier から表示用文字列を導く。
// 例: ["claude_max", "chat"] + "default_claude_max_20x" → "Claude Max 20x"
func deriveSubscriptionType(caps []string, tier string) string {
	has := func(s string) bool {
		for _, c := range caps {
			if c == s {
				return true
			}
		}
		return false
	}
	base := ""
	switch {
	case has("claude_max"):
		base = "Claude Max"
	case has("claude_pro"):
		base = "Claude Pro"
	case has("claude_team") || has("team"):
		base = "Claude Team"
	case has("api"), has("api_individual"):
		base = "API"
	default:
		if len(caps) > 0 {
			base = caps[0]
		}
	}
	if suffix := extractTierMultiplier(tier); suffix != "" {
		if base != "" {
			return base + " " + suffix
		}
		return suffix
	}
	return base
}

// extractTierMultiplier は "default_claude_max_20x" → "20x" のように
// rate_limit_tier 文字列の末尾から N桁数字+x のサフィックスを抽出する。
// マッチしなければ空文字。
func extractTierMultiplier(tier string) string {
	if tier == "" {
		return ""
	}
	last := tier[strings.LastIndex(tier, "_")+1:]
	if len(last) < 2 || last[len(last)-1] != 'x' {
		return ""
	}
	for _, ch := range last[:len(last)-1] {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return last
}
