package main

// AppVersion はビルド時に -ldflags "-X main.AppVersion=vX.Y.Z" で上書きされる。
var AppVersion = "dev"

// Copyright は UI のフッターに表示する著作権表記。
const Copyright = "© 2026 Shuhei Konno"

// trayVersionLabel はトレイメニューに表示するアプリ名 + バージョン文字列。
var trayVersionLabel = "ClaudeMonitor " + AppVersion
