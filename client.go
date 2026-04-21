package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	anthropicUsageEndpoint = "https://api.anthropic.com/api/oauth/usage"
	anthropicBetaHeader    = "oauth-2025-04-20,fine-grained-tool-streaming-2025-05-14"
)

// UsageResponse はサーバからの生レスポンス。five_hour と seven_day のみ使用。
// seven_day_opus 等の他ウィンドウはサーバは返すが UI で表示しないため無視。
type UsageResponse struct {
	FiveHour *rawUsageWindow `json:"five_hour,omitempty"`
	SevenDay *rawUsageWindow `json:"seven_day,omitempty"`
}

type rawUsageWindow struct {
	// 実 API は 7.0 / 22.0 のような float を返す（int でも Unmarshal は通るが小数点以下があると失敗するため float64 で受ける）。
	Utilization float64 `json:"utilization"`
	ResetsAt    *string `json:"resets_at"`
}

// ErrAuthExpired は 401/403 を区別するための番兵エラー。
var ErrAuthExpired = errors.New("anthropic: auth expired or invalid")

// fetchUsage は Anthropic の非公開 OAuth エンドポイントに GET し、
// 使用率を取得する。429 など一時的エラーは単純にエラー返却。
func fetchUsage(ctx context.Context, auth *AuthData) (*UsageResponse, error) {
	if auth == nil || auth.AccessToken == "" {
		return nil, errors.New("auth token 未設定")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, anthropicUsageEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-beta", anthropicBetaHeader)
	req.Header.Set("User-Agent", "ClaudeMonitor/0.3")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrAuthExpired, resp.StatusCode, truncate(body, 200))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic: status=%d body=%s", resp.StatusCode, truncate(body, 200))
	}

	var usage UsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, fmt.Errorf("レスポンスパース失敗: %w", err)
	}
	return &usage, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
