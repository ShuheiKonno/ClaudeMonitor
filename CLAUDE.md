# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 言語

すべての応答は日本語で行うこと。

## プロジェクト概要

ClaudeMonitor は Claude AI のトークン使用量をリアルタイムで監視するWindows専用デスクトップウィジェット。Go + WebView2 で実装されており、5時間ウィンドウ・7日間ウィンドウの使用率と次回リセット時刻を表示する。

## ビルドコマンド

```bash
# 本番ビルド（コンソール非表示）
go build -ldflags "-H windowsgui -X main.AppVersion=v0.9.6" -o ClaudeMonitor.exe .

# デバッグビルド（コンソール出力あり）
go build -o ClaudeMonitor-debug.exe .

# テスト実行
go test ./...

# アイコン再生成（assets/icon.ico と assets/icon-preview.png を更新）
go run ./cmd/genicon

# Windows リソースファイル再生成（アイコン変更時・バージョン更新時に必要）
# versioninfo.json のバージョン番号を更新してから実行する
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
goversioninfo -icon assets/icon.ico -o rsrc_windows_amd64.syso
go build -ldflags "-H windowsgui -X main.AppVersion=v0.9.6" -o ClaudeMonitor.exe .
```

## 作業フロー（コーディング〜リリース）

機能追加・修正を行ったら以下を順番に実施すること。特に **②Codexレビュー・③バージョン更新・④リリース** を忘れない（過去に①の直後にコミット／PRだけ行い、②〜④を飛ばして古いバージョン番号のままになった事例がある）。

1. **実装・テスト**: `go test ./...` と `go build` が通ることを確認。可能ならアプリを起動して実機確認する（GUI 動作は WebView2 + claude.ai ログインが必要なため、`/api/settings` 等の localhost API で確認できる部分は curl 相当で確認するとよい）。
2. **Codexレビュー**: `/codex:review`（より厳しめなら `/codex:adversarial-review`）でレビューし、指摘（特に P1/P2）を修正してから次へ進む。レビューは変更をコミット済みの状態で実行する。
3. **バージョン番号の更新**: リリースするなら必ずバージョンを上げる。`release` スキル（`.claude/skills/release/`）を使うのが基本。手動の場合は以下 3 箇所を**同じ番号**に揃える:
   - `versioninfo.json` の `FileVersion` / `ProductVersion`（数値構造体と文字列の両方）
   - `CLAUDE.md` のビルドコマンド `-X main.AppVersion=vX.Y.Z`（2 箇所）
   - `goversioninfo` で `rsrc_windows_amd64.syso` を再生成
   - 注: `version.go` の `var AppVersion = "dev"` は ldflags で上書きされる既定値なので変更しない。
   - **バンプは機能の PR に含めるか、機能 PR のマージ前に行う**（別 PR に分かれて main に取り込まれ損ねるのを防ぐ）。
4. **リリース**: バージョンを上げた main から実施する。
   - リリースビルド: `go build -ldflags "-H windowsgui -X main.AppVersion=vX.Y.Z" -o ClaudeMonitor.exe .`
   - 生成 exe の埋め込みバージョンが目的の番号になっているか確認（`(Get-Item ClaudeMonitor.exe).VersionInfo`）。
   - タグ `vX.Y.Z` と GitHub Release を作成し、**`ClaudeMonitor.exe` を必ず添付**する。
     例: `gh release create vX.Y.Z --target main --title "vX.Y.Z — <短い説明>" --notes-file <notes> ClaudeMonitor.exe`
   - Release タイトルは `vX.Y.Z — <短い説明>`、本文は `## 変更内容` 形式（既存リリースを踏襲）。

## アーキテクチャ

### 全体構成

2つのWebView2インスタンスを使う二層構造：

1. **メインUI WebView** — `http://127.0.0.1:{random_port}/` を表示するウィジェット（230×260px、フレームレス）
2. **認証WebView** — オフスクリーン（-32000,-32000）に配置。claude.ai のセッションクッキーを保持し、JS注入で内部APIを呼び出す

```
認証WebView (claude.ai cookies)
    ↓ JS fetch → claude.ai /api/organizations/{orgId}/usage
    ↓ JS fetch → claude.ai /edge-api/bootstrap/.../app_start
    ↓ window.claude.setUsage() ← Go Bind callback
Go バックグラウンド (usage.go)
    ↓ cachedUsage 更新 + refreshNotify チャネル
    ↓ トレイアイコン再描画 (tray.go)
    ↓ 通知チェック (notify.go)
メインUI WebView (html.go)
    → GET /api/usage, /api/settings (localhost HTTP)
```

### 状態管理

スナップショット + RWMutex パターン。グローバル状態は各モジュールで mutex 保護：

| 変数 | 保護 | モジュール |
|------|------|-----------|
| `cachedUsage` | `usageMu sync.RWMutex` | usage.go |
| `config` | `configMu sync.Mutex` | config.go |
| 通知フラグ | `notifyMu sync.Mutex` | notify.go |
| `cachedStatus` | `statusMu sync.RWMutex` | status.go |

設定ファイルの書き込みは `.tmp` への書き込み → atomic rename で行い、クラッシュ耐性を確保。

### 主要ファイル

| ファイル | 責務 |
|---------|------|
| `main.go` | エントリーポイント、Win32ウィンドウ管理、マルチモニター対応、DPIスケーリング |
| `api.go` | localhost HTTP REST サーバー（`/api/usage`, `/api/settings`, `/api/refresh` 等） |
| `auth_webview.go` | 認証WebView管理、JS注入、`setUsage` コールバック受信 |
| `config.go` | `%LOCALAPPDATA%\ClaudeMonitor\config.json` の読み書き |
| `tray.go` | システムトレイアイコンの動的レンダリング（golang.org/x/image で円グラフを描画） |
| `notify.go` | 使用量閾値通知（5h: 60%/80%）と status.claude.com インシデント通知 |
| `status.go` | status.claude.com/api/v2/summary.json をキャッシュ付きでポーリング（間隔は設定可能、既定5分） |
| `usage.go` | 使用量スナップショット管理、バックグラウンドコレクター（間隔は設定可能、既定5分） |
| `html.go` | ウィジェットUIのHTML/CSS/JS（バイナリに埋め込み） |
| `cmd/genicon/` | `assets/icon.ico` の生成ユーティリティ |

## データ永続化

- **設定**: `%LOCALAPPDATA%\ClaudeMonitor\config.json`
- **認証クッキー**: `%LOCALAPPDATA%\ClaudeMonitor\AuthWebView2\`
- **通知ログ**: `%LOCALAPPDATA%\ClaudeMonitor\notify.log`

ログアウト時は `AuthWebView2\` ディレクトリを削除して認証データをリセットする。

## テスト

```bash
# 通常テスト
go test ./...

# 視覚的プレビュー（opt-in）
DUMP_SIZES=1 go test -run TestDumpSizes ./cmd/genicon      # 各サイズのアイコンをPNGで出力
DUMP_TRAY_PREVIEW=1 go test -run TestDumpTrayPreview .     # トレイアイコン状態サンプルを出力
```

テストモック: `withMockBalloon()`, `withConfig()`, `resetNotifyState()` が各テストの分離に使用されている。

## QAチェックリスト

### バナー表示時のレイアウト確認（必須）

ウィンドウは **230 × 295px 固定**（`main.go` の `windowWidth`/`windowHeight` 定数）。バナーが増えると下部が `overflow: hidden` で切れる。
UI変更・バナー追加時は必ず以下の組み合わせでレイアウトが収まるか確認すること。

コンテンツ使用可能高さ: 295 − タイトルバー27 − padding12 = **256px**

| シナリオ | auth-banner | status-banner | overage行 | 期待結果 |
|---------|:-----------:|:-------------:|:--------:|---------|
| 通常（ログイン済・障害なし） | — | — | — | row-bar各93px、余裕あり |
| ログイン済・overage表示・障害あり | — | ✓ | ✓ | row-bar3本各47px、OK |
| 未ログイン | ✓ | — | — | row-bar各60px、OK |
| 未ログイン・障害あり | ✓ | **非表示** | — | auth優先でstatus-bannerを隠す（設計による制御） |

**設計ルール**: `authState !== 'ok'` のとき `status-banner` は非表示にする（`renderStatusBanner` 内で制御）。未ログイン時は「先にログインを」という UX 観点と、将来の高さ変更時に備えたレイアウト安全マージンの確保が目的。

新規バナー要素を追加するときは、既存バナーとの同時表示でも256px（コンテンツ使用可能高さ）に収まるかピクセル計算で確認すること。row-barの最小高さは約33px（padding6+bar12+gap2+テキスト9+border2）。

## 実装上の注意

- `runtime.LockOSThread()` を `main()` 冒頭で必ず呼ぶこと（COM初期化に必須）
- **単一インスタンス強制**: Named mutex `Global\claude-monitor-single-instance-mutex` で多重起動防止
- **オフスクリーンWebView**: 認証WebViewは (-32000,-32000) に配置。非表示にすると WebView2 のタイマーがスロットリングされるため
- **再起動プロトコル**: ログアウト時は `--restarted` フラグ付きで再起動、600ms 待機後にファイルロック解放
- **ポーリング二重化**: Goのtimerと JSの`setInterval`で冗長化（どちらかが止まっても動作継続）
