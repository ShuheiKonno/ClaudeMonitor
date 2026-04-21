package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

//go:embed assets/icon.ico
var embeddedIconICO []byte

var (
	procLoadImageW   = user32.NewProc("LoadImageW")
	procSendMessageW = user32.NewProc("SendMessageW")
)

const (
	WM_SETICON      = 0x0080
	ICON_SMALL      = 0
	ICON_BIG        = 1
	IMAGE_ICON      = 1
	LR_LOADFROMFILE = 0x00000010
	LR_DEFAULTSIZE  = 0x00000040
	LR_SHARED       = 0x00008000
)

// applyWindowIcon は assets/icon.ico を一時ファイルに展開し、
// WM_SETICON でウィンドウ（タイトルバー・Alt+Tab・タスクバー）にアイコンを適用する。
// PE リソース埋め込み（.syso）を持たないビルドでもタイトルバー等の見た目は整う。
func applyWindowIcon(hwnd uintptr) {
	if hwnd == 0 || len(embeddedIconICO) == 0 {
		return
	}
	path := filepath.Join(os.TempDir(), "claude-monitor-icon.ico")
	// 既存のままで良いので書き込み失敗は許容しない（次回再試行）
	if err := os.WriteFile(path, embeddedIconICO, 0o644); err != nil {
		return
	}
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}

	hIconBig, _, _ := procLoadImageW.Call(
		0, uintptr(unsafe.Pointer(pathPtr)),
		IMAGE_ICON, 32, 32,
		LR_LOADFROMFILE,
	)
	hIconSmall, _, _ := procLoadImageW.Call(
		0, uintptr(unsafe.Pointer(pathPtr)),
		IMAGE_ICON, 16, 16,
		LR_LOADFROMFILE,
	)

	if hIconBig != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, hIconBig)
	}
	if hIconSmall != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, hIconSmall)
	}
}
