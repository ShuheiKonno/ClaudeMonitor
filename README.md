# Claude Monitor

Claude Code（サブスクリプションプラン）のトークン使用量を常時モニタするための Windows デスクトップウィジェット。

- **直近5時間** と **直近7日間** のトークン消費量を Claude Desktop とほぼ同じ基準で算出
- **システムトレイアイコン**が 5h 使用率を数字で、7d 使用率を背景色（緑 / 橙 / 赤）で表示
- フレームレスで常に最前面に置けるコンパクトな表示
- `~/.claude/projects/*/*.jsonl` のローカルログを直接解析するため、外部 API アクセス不要

## ダウンロード

Windows 用のビルド済みバイナリは [Releases](https://github.com/ShuheiKonno/ClaudeMonitor/releases) から取得できます。`claude-monitor.exe` をダブルクリックするだけで起動します（インストール不要）。

## 使い方

1. `claude-monitor.exe` を起動すると右下に半透明ウィジェットが表示されます
2. 通知領域（タスクトレイ）にアイコンが常駐します
3. 設定（⚙）でご利用中のプラン（Pro / Max $100 / Max $200 / 自動推定）を選択
4. 閉じる（✕）で**タスクトレイにしまう**、終了はトレイアイコンの右クリック → 終了

### タスクトレイアイコンの見かた

| 背景色 | 7日間使用率 |
|---|---|
| ライトグリーン | 0〜50% |
| ライトオレンジ | 51〜80% |
| レッド | 81〜100% |

アイコン中央の数字は直近5時間の使用率（%）。

### 操作

- **✕**: ウィジェットをトレイへしまう
- **⚙**: 設定画面（プラン・最前面・半透明）
- **⟳**: 手動で使用量を再計測
- **トレイ左クリック**: ウィジェットを再表示
- **トレイ右クリック**: メニュー（表示 / 終了）
- **タイトルバーをドラッグ**: ウィジェット移動（位置は保存されます）

## 仕組み

### データソース

Claude Code が `~/.claude/projects/<project>/<session>.jsonl` に書き出す生ログの `message.usage` フィールドをストリーミング解析し、セッションをまたいで集計します。

### トークン計測

Anthropic の使用量リミットはコスト換算で計測されるため、以下の式で概算しています:

```
使用トークン = input_tokens + output_tokens + cache_creation_input_tokens
```

`cache_read_input_tokens` は billing 単価が input の約 1/10 かつ会話履歴の再読込で毎ターン数百万トークン発生するため、リミット比較から除外しています。

### プランプリセット（2026-04 時点の観測値）

| プラン | 5h リミット | 7d リミット |
|---|---|---|
| Pro | 500K tokens | 2M tokens |
| Max $100 (5x) | 2.5M tokens | 10M tokens |
| Max $200 (20x) | 9.5M tokens | 35M tokens |

Anthropic は公式のしきい値を公開していないため、Claude Desktop の表示から逆算した推定値です。手元の環境で Claude Desktop の使用率とほぼ一致することを確認しています。

「自動推定」を選ぶと `~/.claude/stats-cache.json` の過去30日の日次最大メッセージ数からプランを推定してプリセットを適用します。

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
- Win32 API 直叩き（Shell_NotifyIcon / SetLayeredWindowAttributes / Monitor API）
- `golang.org/x/image` でトレイアイコンの動的描画

## ライセンス

[MIT License](LICENSE)
