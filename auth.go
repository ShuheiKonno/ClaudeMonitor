package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type AuthData struct {
	AccessToken      string
	Email            string
	DisplayName      string
	SubscriptionType string
	ExpiresAt        time.Time // UTC
}

// loadAuthData は ~/.claude/.credentials.json から OAuth トークンを、
// ~/.claude.json から表示用アカウント情報を読み出す。
// Windows / Linux 共通で credentials.json を使う（macOS Keychain 非対応）。
func loadAuthData() (*AuthData, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	credPath := filepath.Join(home, ".claude", ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("credentials.json 読み込み失敗: %w", err)
	}

	var creds struct {
		ClaudeAiOauth struct {
			AccessToken      string `json:"accessToken"`
			ExpiresAt        int64  `json:"expiresAt"` // ミリ秒 Unix
			SubscriptionType string `json:"subscriptionType"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("credentials.json パース失敗: %w", err)
	}
	if creds.ClaudeAiOauth.AccessToken == "" {
		return nil, errors.New("claudeAiOauth.accessToken が見つかりません")
	}

	auth := &AuthData{
		AccessToken:      creds.ClaudeAiOauth.AccessToken,
		SubscriptionType: creds.ClaudeAiOauth.SubscriptionType,
	}
	if creds.ClaudeAiOauth.ExpiresAt > 0 {
		auth.ExpiresAt = time.Unix(0, creds.ClaudeAiOauth.ExpiresAt*int64(time.Millisecond)).UTC()
	}

	// ~/.claude.json からユーザー情報を補強（失敗しても致命傷ではない）
	cfgPath := filepath.Join(home, ".claude.json")
	if cfgBytes, err := os.ReadFile(cfgPath); err == nil {
		var cfg struct {
			OauthAccount *struct {
				EmailAddress string `json:"emailAddress"`
				DisplayName  string `json:"displayName"`
			} `json:"oauthAccount"`
		}
		if json.Unmarshal(cfgBytes, &cfg) == nil && cfg.OauthAccount != nil {
			auth.Email = cfg.OauthAccount.EmailAddress
			auth.DisplayName = cfg.OauthAccount.DisplayName
		}
	}
	return auth, nil
}

// isExpired は トークンが期限切れかどうか。expiresAt が 0 の場合は有効扱い。
func (a *AuthData) isExpired() bool {
	return !a.ExpiresAt.IsZero() && time.Now().UTC().After(a.ExpiresAt)
}
