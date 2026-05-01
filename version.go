package main

// AppVersion はビルド時に -ldflags "-X main.AppVersion=v0.7.1" で上書きされる。
var AppVersion = "dev"

// Copyright は UI のフッターに表示する著作権表記。
const Copyright = "© 2026 Shuhei Konno"
