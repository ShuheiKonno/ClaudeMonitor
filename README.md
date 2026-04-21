# Claude Monitor

Claude Code（サブスクリプションプラン）のトークン使用量を Anthropic サーバから直接取得して常時モニタする Windows デスクトップウィジェット。

- **5時間セッション / 7日** の 2 ウィンドウをサーバから取得して表示
- **システムトレイアイコン**が 5h 使用率を数字で、7d 使用率を背景色（緑 / 橙 / 赤）で表示
- 各ウィンドウの**次回リセット時刻**までのカウントダウンを表示
- フレームレスで常に最前面に置ける省スペース設計（230 × 180 px）
- Claude Code が保存した OAuth トークン（`~/.claude/.credentials.json`）を再利用するので別途ログイン不要

## ダウンロード

Windows 用のビルド済みバイナリは [Releases](https://github.com/ShuheiKonno/ClaudeMonitor/releases) から取得できます。`claude-monitor.exe` をダブルクリックするだけで起動します（インストール不要）。

## 前提

1. Claude Code CLI をインストール済みでログインしていること（`~/.claude/.credentials.json` が存在する状態）
2. 未ログインまたは期限切れの場合は、ウィジェット／トレイに案内が出ます。ターミナルで `claude` を実行してください

## 使い方

1. `claude-monitor.exe` を起動すると右下に半透明ウィジェットが表示されます
2. 通知領域（タスクトレイ）にアイコンが常駐します
3. 設定（⚙）で最前面・半透明を切替
4. 閉じる（✕）で**タスクトレイにしまう**、終了はトレイアイコンの右クリック → 終了

### タスクトレイアイコンの見かた

円形のディスク + 中央に直近 5 時間の使用率（%）。外周のリングは 7 日使用率を 12 時方向始点・時計回りに埋めるゲージで、使用率バンドに応じて全体の色味も変わります。認証エラー時はグレー "?" アイコンになります。

| ディスク色 | 7日間使用率 |
|---|---|
| セージグリーン | 0〜50% |
| アンバー | 51〜80% |
| テラコッタ | 81〜100% |

### 操作

- **✕**: ウィジェットをトレイへしまう
- **⚙**: 設定画面（最前面 / 半透明）
- **⟳**: 手動で使用量を再取得
- **トレイ左クリック**: ウィジェットを再表示
- **トレイ右クリック**: メニュー（表示 / 更新 / 再認証 / 終了）
- **タイトルバーをドラッグ**: ウィジェット移動（位置は保存されます）

## 仕組み

### データソース

[jjsmackay/claude-usage-vscode](https://github.com/jjsmackay/claude-usage-vscode) と同じアプローチで、Anthropic の非公開 OAuth エンドポイントに直接問い合わせます。

```
GET https://api.anthropic.com/api/oauth/usage
Authorization: Bearer <access_token>
anthropic-beta: oauth-2025-04-20,fine-grained-tool-streaming-2025-05-14
```

Access token は Claude Code CLI がログイン時に保存した `~/.claude/.credentials.json` から再利用します。ユーザー情報（表示名・メール）は `~/.claude.json` の `oauthAccount` を参照します。

### レスポンス

`five_hour`（5時間セッション）と `seven_day`（7日間）の `utilization` と `resets_at` をそのまま表示します。サーバは `seven_day_opus` / `seven_day_oauth_apps` など追加ウィンドウも返しますが、シンプルさ優先で UI では使用しません。上限トークン数やプラン判定はサーバ側で行われているため、クライアントは解釈不要です。

### ポーリング

既定 5 分間隔。認証エラー時は直近表示を保持しつつ UI にバナーを表示し、通信エラーは自動で回復を待ちます。

### 注意事項

この API は公式ドキュメント化されていないため、Anthropic による仕様変更で動作しなくなる可能性があります。

## アイコン

アプリアイコンは `assets/icon.ico`、長辺 512px のプレビューは `assets/icon-preview.png` に同梱しています。「円環ゲージ + 中央シンボル」を共通コンセプトに、アプリ側は Claude ブランドオレンジのフルリング + "C"、トレイ側は 7 日使用率に応じたリングフィル + 中央に 5 時間使用率（％）で動的描画します。

```bash
go run ./cmd/genicon   # assets/icon.ico と assets/icon-preview.png を再生成
```

Explorer の `.exe` 表示用アイコンは `rsrc_windows_amd64.syso` 経由で自動的に埋め込まれます。再生成する場合:

```bash
go install github.com/akavel/rsrc@latest
rsrc -ico assets/icon.ico -arch amd64 -o rsrc_windows_amd64.syso
go build -ldflags "-H windowsgui" -o claude-monitor.exe .
```

## ビルド

Go 1.25 以降 + Windows 10/11 + WebView2 ランタイムが必要です。

```bash
go build -ldflags "-H windowsgui" -o claude-monitor.exe .
```

### 開発用（コンソール付き）

```bash
go build -o claude-monitor-debug.exe .
./claude-monitor-debug.exe
```

### テスト

```bash
go test ./...
```

## 技術スタック

- Go 1.25.6
- [go-webview2](https://github.com/jchv/go-webview2)（WebView2 ラッパー）
- Win32 API 直叩き（Shell_NotifyIcon / SetLayeredWindowAttributes / Monitor API / MessageBoxW）
- 標準 `net/http` で Anthropic OAuth エンドポイントを呼び出し
- `golang.org/x/image` でトレイアイコンの動的描画

## ライセンス

[MIT License](LICENSE)
