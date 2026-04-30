# Claude Monitor

Claude Code（サブスクリプションプラン）のトークン使用量を claude.ai から直接取得して常時モニタする Windows デスクトップウィジェット。

- **5時間セッション / 7日** の 2 ウィンドウを claude.ai から取得して表示
- **システムトレイアイコン**が 5h 使用率を数字で、7d 使用率を背景色（緑 / 橙 / 赤）で表示
- 各ウィンドウの**次回リセット時刻**までのカウントダウンを表示
- フレームレスで常に最前面に置ける省スペース設計（230 × 210 px）
- 5 時間使用量の閾値到達 / claude.com Status の障害を **Windows バルーン通知** で知らせる（v0.7.0〜）
- 補助 WebView2 が claude.ai の Cookie を保持するので、初回のみアプリ内でログインすれば以降は自動で再認証されます

## ダウンロード

Windows 用のビルド済みバイナリは [Releases](https://github.com/ShuheiKonno/ClaudeMonitor/releases) から取得できます。`ClaudeMonitor.exe` をダブルクリックするだけで起動します（インストール不要）。

## 前提

1. Windows 10 / 11 + WebView2 ランタイム（最新の Microsoft Edge が入っていれば同梱されています）
2. claude.ai のアカウント

初回起動時はウィジェットに **「未ログイン」** バナーが表示されます。バナーの「ログイン」ボタンまたはトレイ右クリックメニューの「Claude にログイン…」を押すとアプリ内に claude.ai のログイン画面が開くのでサインインしてください。Cookie は `%LOCALAPPDATA%\ClaudeMonitor\AuthWebView2\` に保持され、次回以降は自動で再認証されます。

> Claude Code CLI のログイン状態には依存しません。`~/.claude/.credentials.json` も `~/.claude.json` も読み書きしないため、Claude Code 側に影響を与えず、影響も受けません。

## 使い方

1. `ClaudeMonitor.exe` を起動すると右下に半透明ウィジェットが表示されます
2. 通知領域（タスクトレイ）にアイコンが常駐します
3. 設定（⚙）で最前面・半透明・通知 ON/OFF を切替
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
- **⚙**: 設定画面（最前面 / 半透明 / 通知）
- **⟳**: 手動で使用量を再取得
- **トレイ左クリック**: ウィジェットを再表示
- **トレイ右クリック**: メニュー（表示 / 更新 / Claude にログイン… / 終了）
- **タイトルバーをドラッグ**: ウィジェット移動（位置は保存されます）

## 仕組み

### データソース

[jjsmackay/claude-usage-vscode](https://github.com/jjsmackay/claude-usage-vscode) と同じアプローチで、claude.ai の Web セッションを補助 WebView2 で抱え、内部 API に直接問い合わせます。

```
GET https://claude.ai/api/organizations/{orgId}/usage
GET https://claude.ai/edge-api/bootstrap/{orgId}/app_start?statsig_hashing_algorithm=djb2&growthbook_format=sdk&include_system_prompts=false
```

`orgId` は `lastActiveOrg` Cookie から取得します。前者から 5h / 7d の `utilization` と `resets_at`、後者からメール・表示名・サブスクリプションプラン（`Claude Pro` / `Claude Max 5x` 等）を得ます。サーバは `seven_day_opus` / `seven_day_oauth_apps` など追加ウィンドウも返しますが、シンプルさ優先で UI では使用しません。

### 認証

補助 WebView2 を `(-32000, -32000)` のオフスクリーンに常駐させ、claude.ai のセッション Cookie を保持します。`SW_HIDE` ではなくオフスクリーン配置にしているのは、隠し窓だと WebView2 が `setInterval` / `requestAnimationFrame` をスロットルする可能性があるためです。`WS_EX_TOOLWINDOW` でタスクバーから外し、Alt+Tab にも出ません。

Cookie が無効・期限切れの場合は使用率取得が `401` / `403` で失敗し、ウィジェットに「未ログイン」バナーが表示されます。「ログイン」ボタンを押すと補助 WebView2 がオンスクリーンに復帰して claude.ai のログイン画面が出ます。サインインが完了すると自動で再びオフスクリーンへ退避します。

### ポーリング

既定 5 分間隔。Go 側の ticker と補助 WebView の `setInterval` で二重保険にしています。Cookie 失効時は直近表示を保持しつつ UI にバナーを表示し、通信エラーは自動で回復を待ちます。

### 通知（v0.7.0〜）

設定でオンにすると、以下のタイミングで Windows のトースト通知（バルーン）を出します。設定パネルの「通知」セクションで個別に ON / OFF できます（既定オン）。

- **5 時間使用量** が **60% / 80%** を超えたとき（同セッション内では各 1 回。`five_hour.resets_at` の変化でフラグがリセット）
- **status.claude.com** で新規インシデントを検出したとき（`impact` に応じて NIIF_INFO / NIIF_WARNING / NIIF_ERROR）

起動直後の最初のスナップショットは基準値として保持し通知抑制するため、再起動時に過去の閾値到達やインシデントが再通知されることはありません。

### 注意事項

これらの API は公式ドキュメント化されていないため、Anthropic による仕様変更で動作しなくなる可能性があります。

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
- [go-webview2](https://github.com/jchv/go-webview2)（WebView2 ラッパー）— UI 用 + claude.ai 用補助の 2 つを生成
- Win32 API 直叩き（Shell_NotifyIcon / SetLayeredWindowAttributes / Monitor API / CreateIconIndirect / SetWindowLong）
- 注入 JS から `Bind` 経由で取得結果を Go へ受け渡し（claude.ai/usage と app_start を `fetch`）
- 標準 `net/http` で `status.claude.com/api/v2/summary.json` をポーリング
- `golang.org/x/image` でトレイアイコンの動的描画

## ライセンス

[MIT License](LICENSE)
