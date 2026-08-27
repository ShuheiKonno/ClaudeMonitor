package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	procSendMessageTimeoutW      = user32.NewProc("SendMessageTimeoutW")
	procMessageBoxW              = user32.NewProc("MessageBoxW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procShowWindowAsync          = user32.NewProc("ShowWindowAsync")

	procOpenProcess         = kernel32.NewProc("OpenProcess")
	procTerminateProcess    = kernel32.NewProc("TerminateProcess")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
)

const (
	SMTO_BLOCK       = 0x0001
	SMTO_ABORTIFHUNG = 0x0002

	MB_OK            = 0x00000000
	MB_YESNO         = 0x00000004
	MB_ICONWARNING   = 0x00000030
	MB_DEFBUTTON2    = 0x00000100
	MB_SETFOREGROUND = 0x00010000
	MB_TOPMOST       = 0x00040000
	IDYES            = 6

	PROCESS_TERMINATE = 0x0001
	SYNCHRONIZE       = 0x00100000
	WAIT_OBJECT_0     = 0

	WM_NULL = 0x0000

	// showInstanceTimeoutMs は既存インスタンスの応答待ち時間。
	// これを超えたら「プロセスは生きているがウィンドウが応答しない」とみなす。
	showInstanceTimeoutMs = 3000
)

// 既存インスタンスのウィンドウ出現を待つ時間。
// 起動直後はミューテックス作成からウィンドウ生成までに時間があるため
// （startTray は windowHandle を最大 10 秒待つ）、
// この間に二重起動しても「ゾンビ」と誤判定しないよう待ってから判断する。
const (
	instanceLookupTimeout  = 15 * time.Second
	instanceLookupInterval = 300 * time.Millisecond
)

const dialogTitle = "Claude モニター"

// findTrayWindow は既存インスタンスの（非表示トップレベルの）トレイウィンドウを探す。
func findTrayWindow() uintptr {
	classPtr, err := syscall.UTF16PtrFromString(trayClassName)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(classPtr)), 0)
	return hwnd
}

// isWindowResponsive は WM_NULL を投げてウィンドウの UI スレッドが生きているか確認する。
// ハングしたウィンドウに同期的な API（ShowWindow など）を呼ぶと呼び出し側も止まるため、
// 操作の前に必ずこれで確認する。
func isWindowResponsive(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	var result uintptr
	r, _, _ := procSendMessageTimeoutW.Call(
		hwnd,
		WM_NULL,
		0, 0,
		SMTO_ABORTIFHUNG|SMTO_BLOCK,
		showInstanceTimeoutMs,
		uintptr(unsafe.Pointer(&result)),
	)
	return r != 0
}

// showExistingWindowByTitle はタイトルからメインウィンドウを探して前面に出す。
// 新プロセス側から呼ぶことで、既存プロセスより強いフォアグラウンド権限を利用できる。
// 対象がハングしている場合に巻き込まれないよう、生存確認してから
// 非同期版の ShowWindowAsync で表示する。
func showExistingWindowByTitle(windowTitle string) bool {
	hwnd := findMainWindow(windowTitle)
	if !isWindowResponsive(hwnd) {
		return false
	}
	procShowWindowAsync.Call(hwnd, SW_SHOW)
	procSetForegroundWindow.Call(hwnd)
	return true
}

// notifyExistingInstance は既存インスタンスに表示要求を送る。
// 応答があれば true（既存インスタンスが生きている）。
// PostMessage ではハングしたプロセスにも「成功」してしまうため、
// タイムアウト付きの SendMessageTimeout で生存確認を兼ねる。
//
// ウィンドウがまだ見つからない場合は「起動途中」の可能性があるため、
// instanceLookupTimeout まで再試行してから諦める（ゾンビとの誤判定を防ぐ）。
func notifyExistingInstance(windowTitle string) bool {
	msg := registerWindowMessage(showInstanceMessageName)
	deadline := time.Now().Add(instanceLookupTimeout)
	for {
		if trayHwndOther := findTrayWindow(); trayHwndOther != 0 && msg != 0 {
			var result uintptr
			r, _, _ := procSendMessageTimeoutW.Call(
				trayHwndOther,
				uintptr(msg),
				0, 0,
				SMTO_ABORTIFHUNG|SMTO_BLOCK,
				showInstanceTimeoutMs,
				uintptr(unsafe.Pointer(&result)),
			)
			if r != 0 {
				showExistingWindowByTitle(windowTitle)
				return true
			}
			// ウィンドウは存在するのに応答しない = ハング。待っても回復しないので即座に諦める
			return false
		}
		// トレイウィンドウが無い場合（旧バージョンが動作中など）はタイトルで探す
		if mainHwnd := findMainWindow(windowTitle); mainHwnd != 0 {
			if !isWindowResponsive(mainHwnd) {
				// ウィンドウはあるが応答しない = ハング。待っても回復しない
				return false
			}
			procShowWindowAsync.Call(mainHwnd, SW_SHOW)
			procSetForegroundWindow.Call(mainHwnd)
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(instanceLookupInterval)
	}
}

// messageBox は簡易 MessageBox ラッパー。戻り値は押されたボタン ID。
func messageBox(text, caption string, flags uintptr) uintptr {
	textPtr, err1 := syscall.UTF16PtrFromString(text)
	captionPtr, err2 := syscall.UTF16PtrFromString(caption)
	if err1 != nil || err2 != nil {
		return 0
	}
	r, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(captionPtr)),
		flags|MB_SETFOREGROUND|MB_TOPMOST,
	)
	return r
}

// terminateProcessByHwnd は指定ウィンドウを所有するプロセスを強制終了し、完全に終了するまで待つ。
func terminateProcessByHwnd(hwnd uintptr) bool {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return false
	}
	handle, _, _ := procOpenProcess.Call(PROCESS_TERMINATE|SYNCHRONIZE, 0, uintptr(pid))
	if handle == 0 {
		return false
	}
	defer procCloseHandle.Call(handle)
	if r, _, _ := procTerminateProcess.Call(handle, 1); r == 0 {
		return false
	}
	// WebView2 のファイルロックが解放されるまで待つ
	r, _, _ := procWaitForSingleObject.Call(handle, 5000)
	return r == WAIT_OBJECT_0
}

// handleUnresponsiveInstance は「プロセスは残っているが応答しない」状態をユーザーに伝え、
// 可能であれば強制終了して起動を続行するか確認する。
// 起動を続行してよい場合に true を返す。
func handleUnresponsiveInstance(windowTitle string) bool {
	// トレイウィンドウが無くてもメインウィンドウが残っていれば、そこから PID を特定できる
	hwnd := findTrayWindow()
	if hwnd == 0 {
		hwnd = findMainWindow(windowTitle)
	}
	if hwnd == 0 {
		// ウィンドウが一切見つからない（完全にゾンビ化している）ケース。
		// 終了対象を特定できないため、手動対処を案内する。
		messageBox(
			"Claude モニターは既に起動していますが、ウィンドウが見つかりません。\n\n"+
				"タスク マネージャーで ClaudeMonitor.exe を終了してから、もう一度起動してください。",
			dialogTitle,
			MB_OK|MB_ICONWARNING,
		)
		return false
	}
	answer := messageBox(
		"Claude モニターは既に起動していますが、応答していません。\n\n"+
			"実行中のプロセスを終了して起動し直しますか？",
		dialogTitle,
		MB_YESNO|MB_ICONWARNING|MB_DEFBUTTON2,
	)
	if answer != IDYES {
		return false
	}
	if !terminateProcessByHwnd(hwnd) {
		messageBox(
			"実行中の Claude モニターを終了できませんでした。\n\n"+
				"タスク マネージャーで ClaudeMonitor.exe を終了してから、もう一度起動してください。",
			dialogTitle,
			MB_OK|MB_ICONWARNING,
		)
		return false
	}
	// 旧プロセスの WebView2 ファイルロック解放を待つ（ログアウト再起動時と同じ猶予）
	time.Sleep(600 * time.Millisecond)
	return true
}

// ensureSingleInstance は多重起動を防ぐ。起動を続行してよい場合に true を返す。
// 既存インスタンスが応答した場合はそれを前面に出して false を返し、
// 応答しない場合はユーザーに状況を伝える（黙って終了しない）。
func ensureSingleInstance(windowTitle string) bool {
	const errorAlreadyExists = 183
	mutexName, _ := syscall.UTF16PtrFromString("Global\\claude-monitor-single-instance-mutex")
	// ここで取得したハンドルはプロセス終了まで保持され、多重起動の判定に使われる。
	// 応答しない旧プロセスを終了した後も、このハンドルによりミューテックスは維持される。
	_, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	errno, ok := err.(syscall.Errno)
	if !ok || errno != errorAlreadyExists {
		return true
	}
	if notifyExistingInstance(windowTitle) {
		return false
	}
	return handleUnresponsiveInstance(windowTitle)
}
