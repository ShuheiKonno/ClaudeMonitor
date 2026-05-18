---
name: release
description: ClaudeMonitor のリリースフロー。バージョン番号を引数に受け取り、versioninfo.json 更新 → goversioninfo 実行 → ビルド確認 → コミット → PR 作成までを順番に行う。
---

ユーザーが指定したバージョン番号（例: `v0.9.3`）で以下のステップを順番に実行してください。
各ステップの完了後に確認してから次へ進んでください。

## Step 1: versioninfo.json を更新

`versioninfo.json` を読み込み、以下のフィールドをすべて新バージョンに合わせて更新する:
- `"ProductVersion"` 内の `"Major"`, `"Minor"`, `"Patch"` の各フィールド
- `"FileVersion"` 内の同フィールド
- `"StringFileInfo"` の `"ProductVersion"` と `"FileVersion"` の文字列値

## Step 2: CLAUDE.md のビルドコマンドを更新

`CLAUDE.md` 内に記載されているビルドコマンドの `-X main.AppVersion=vX.X.X` 部分を新バージョンに更新する。

## Step 3: goversioninfo でリソースファイルを再生成

```bash
goversioninfo -icon assets/icon.ico -o rsrc_windows_amd64.syso
```

エラーが出た場合は `go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest` を先に実行すること。

## Step 4: ビルド確認

```bash
go build -ldflags "-H windowsgui -X main.AppVersion=<新バージョン>" -o ClaudeMonitor.exe .
```

ビルド成功 (`ClaudeMonitor.exe` が生成される) を確認する。

## Step 5: コミット & PR 作成

変更ファイル: `versioninfo.json`, `rsrc_windows_amd64.syso`, `CLAUDE.md`

コミットメッセージ例:
```
chore: bump version to <新バージョン>
```

`git-helper` サブエージェントを使いコミットメッセージを作成し、PR を作成する。
