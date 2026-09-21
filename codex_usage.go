package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"

var (
	codexUsageMu     sync.RWMutex
	cachedCodexUsage UsageSnapshot
	codexRefreshMu   sync.Mutex
	codexHTTPClient  = &http.Client{Timeout: 12 * time.Second}
)

type codexAuthFile struct {
	Tokens struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	} `json:"tokens"`
}

type codexUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow   *codexRateWindow `json:"primary_window"`
		SecondaryWindow *codexRateWindow `json:"secondary_window"`
	} `json:"rate_limit"`
	Credits struct {
		HasCredits bool   `json:"has_credits"`
		Balance    string `json:"balance"`
		Unlimited  bool   `json:"unlimited"`
	} `json:"credits"`
}

type codexRateWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

func codexAuthPath() (string, error) {
	if root := strings.TrimSpace(os.Getenv("CODEX_HOME")); root != "" {
		return filepath.Join(root, "auth.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "auth.json"), nil
}

func readCodexAuth() (codexAuthFile, error) {
	var auth codexAuthFile
	path, err := codexAuthPath()
	if err != nil {
		return auth, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return auth, fmt.Errorf("Codex CLI のログイン情報がありません")
		}
		return auth, err
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return auth, fmt.Errorf("auth.json の解析に失敗: %w", err)
	}
	if auth.Tokens.AccessToken == "" {
		return auth, fmt.Errorf("Codex CLI のアクセストークンがありません")
	}
	return auth, nil
}

func refreshCodexUsage() {
	codexRefreshMu.Lock()
	defer codexRefreshMu.Unlock()
	auth, err := readCodexAuth()
	if err != nil {
		updateCodexUsageError("needs_login", err.Error())
		return
	}
	req, err := http.NewRequest(http.MethodGet, codexUsageURL, nil)
	if err != nil {
		updateCodexUsageError("network_error", err.Error())
		return
	}
	req.Header.Set("Authorization", "Bearer "+auth.Tokens.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ClaudeCodexMonitor/"+AppVersion)
	if auth.Tokens.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", auth.Tokens.AccountID)
	}
	resp, err := codexHTTPClient.Do(req)
	if err != nil {
		updateCodexUsageError("network_error", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		updateCodexUsageError("needs_login", "Codex CLI の認証期限が切れています。codex login を実行してください")
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		updateCodexUsageError("network_error", fmt.Sprintf("usage fetch failed: status=%d %s", resp.StatusCode, strings.TrimSpace(string(body))))
		return
	}
	var raw codexUsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		updateCodexUsageError("network_error", "レスポンス解析失敗: "+err.Error())
		return
	}
	applyCodexUsage(raw, auth.Tokens.IDToken)
}

func updateCodexUsageError(state, msg string) {
	codexUsageMu.Lock()
	cachedCodexUsage.AuthState = state
	cachedCodexUsage.LastError = msg
	codexUsageMu.Unlock()
	fmt.Fprintln(os.Stderr, "[codex-usage]", state, msg)
	updateTrayFromSnapshot()
}

func applyCodexUsage(raw codexUsageResponse, idToken string) {
	five, seven := classifyCodexWindows(raw.RateLimit.PrimaryWindow, raw.RateLimit.SecondaryWindow)
	email, name := codexIdentity(idToken)
	snap := UsageSnapshot{
		Provider: "codex", FiveHour: five, SevenDay: seven,
		Email: email, DisplayName: name, SubscriptionType: codexPlanLabel(raw.PlanType),
		AuthState: "ok", UpdatedAt: time.Now(),
	}
	if raw.Credits.HasCredits || raw.Credits.Balance != "" {
		if balance, err := strconv.ParseFloat(raw.Credits.Balance, 64); err == nil {
			snap.CreditBalance = &balance
		}
	}
	codexUsageMu.Lock()
	cachedCodexUsage = snap
	codexUsageMu.Unlock()
	handleCodexUsageNotification(snap)
	updateTrayFromSnapshot()
	uiDispatch(func() {
		if mainWebViewInst != nil {
			mainWebViewInst.Eval("if (typeof fetchUsage === 'function') fetchUsage();")
		}
	})
}

func classifyCodexWindows(primary, secondary *codexRateWindow) (UsageWindow, UsageWindow) {
	var five, seven UsageWindow
	for _, w := range []*codexRateWindow{primary, secondary} {
		if w == nil {
			continue
		}
		mapped := mapCodexWindow(w)
		if w.LimitWindowSeconds >= 24*60*60 {
			if seven.Label == "" || w.LimitWindowSeconds > windowSeconds(seven.Label) {
				seven = mapped
			}
		} else if five.Label == "" {
			five = mapped
		}
	}
	return five, seven
}

func mapCodexWindow(w *codexRateWindow) UsageWindow {
	label := formatWindowLabel(w.LimitWindowSeconds)
	out := UsageWindow{Utilization: w.UsedPercent, Label: label}
	if w.ResetAt > 0 {
		t := time.Unix(w.ResetAt, 0)
		out.ResetsAt = &t
	}
	return out
}

func formatWindowLabel(sec int64) string {
	switch sec {
	case 5 * 60 * 60:
		return "5時間"
	case 7 * 24 * 60 * 60:
		return "7日"
	}
	if sec >= 24*60*60 && sec%(24*60*60) == 0 {
		return fmt.Sprintf("%d日", sec/(24*60*60))
	}
	if sec >= 60*60 && sec%(60*60) == 0 {
		return fmt.Sprintf("%d時間", sec/(60*60))
	}
	return "利用枠"
}

func windowSeconds(label string) int64 {
	if strings.HasSuffix(label, "日") {
		n, _ := strconv.ParseInt(strings.TrimSuffix(label, "日"), 10, 64)
		return n * 86400
	}
	return 0
}

func codexIdentity(token string) (string, string) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", ""
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ""
	}
	var claims map[string]any
	if json.Unmarshal(b, &claims) != nil {
		return "", ""
	}
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if profile, ok := claims["https://api.openai.com/profile"].(map[string]any); ok {
		if name == "" {
			name, _ = profile["name"].(string)
		}
	}
	return email, name
}

func codexPlanLabel(plan string) string {
	switch strings.ToLower(plan) {
	case "plus":
		return "ChatGPT Plus"
	case "pro", "prolite":
		return "ChatGPT Pro"
	case "team":
		return "ChatGPT Team"
	case "business":
		return "ChatGPT Business"
	case "enterprise":
		return "ChatGPT Enterprise"
	case "edu", "education":
		return "ChatGPT Edu"
	case "free", "guest":
		return "ChatGPT Free"
	}
	if plan == "" {
		return "Codex"
	}
	return "ChatGPT " + strings.ToUpper(plan[:1]) + plan[1:]
}

func getCodexUsageSnapshot() UsageSnapshot {
	codexUsageMu.RLock()
	defer codexUsageMu.RUnlock()
	return cachedCodexUsage
}
