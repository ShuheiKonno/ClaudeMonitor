package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

var (
	shell32             = syscall.NewLazyDLL("shell32.dll")
	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")

	gdi32            = syscall.NewLazyDLL("gdi32.dll")
	procCreateBitmap = gdi32.NewProc("CreateBitmap")
	procDeleteObject = gdi32.NewProc("DeleteObject")

	procCreateIconIndirect = user32.NewProc("CreateIconIndirect")
	procDestroyIcon        = user32.NewProc("DestroyIcon")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procPostMessageW     = user32.NewProc("PostMessageW")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

const (
	NIM_ADD    = 0x0
	NIM_MODIFY = 0x1
	NIM_DELETE = 0x2

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	WM_APP       = 0x8000
	WM_TRAY      = WM_APP + 1
	WM_LBUTTONUP = 0x0202
	WM_RBUTTONUP = 0x0205
	WM_COMMAND   = 0x0111

	MF_STRING    = 0x00000000
	MF_SEPARATOR = 0x00000800

	TPM_RIGHTBUTTON = 0x0002

	SW_HIDE_VAL = 0

	IDM_SHOW = 1
	IDM_EXIT = 2

	trayIconID    = 1
	trayIconSize  = 16
	trayClassName = "ClaudeMonitorTray"
)

// HWND_MESSAGE = (HWND)-3
var hwndMessage = ^uintptr(2)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type msgStruct struct {
	HWnd     uintptr
	Message  uint32
	_        uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type notifyIconData struct {
	CbSize            uint32
	_                 uint32
	HWnd              uintptr
	UID               uint32
	UFlags            uint32
	UCallbackMessage  uint32
	_                 uint32
	HIcon             uintptr
	SzTip             [128]uint16
	DwState           uint32
	DwStateMask       uint32
	SzInfo            [256]uint16
	UTimeoutOrVersion uint32
	SzInfoTitle       [64]uint16
	DwInfoFlags       uint32
	GuidItem          guid
	HBalloonIcon      uintptr
}

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

var (
	trayMu          sync.Mutex
	currentIcon     uintptr
	trayAdded       bool
	trayHwnd        uintptr
	wndProcCallback = syscall.NewCallback(trayWndProc)
)

func startTray() {
	go func() {
		runtime.LockOSThread()
		// windowHandle がセットされるまで待機（最大 10 秒）
		for i := 0; i < 200; i++ {
			if windowHandle != 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if windowHandle == 0 {
			return
		}
		if !createTrayWindow() {
			return
		}
		addTrayIcon()
		updateTrayFromSnapshot()
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				updateTrayFromSnapshot()
			}
		}()
		// メッセージループ（このスレッドをブロック）
		var msg msgStruct
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}

func trayWndProc(hwnd, msg uintptr, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAY:
		switch uint32(lParam) {
		case WM_LBUTTONUP:
			showMainWindow()
		case WM_RBUTTONUP:
			showTrayMenu(hwnd)
		}
		return 0
	case WM_COMMAND:
		switch wParam & 0xFFFF {
		case IDM_SHOW:
			showMainWindow()
		case IDM_EXIT:
			removeTrayIcon()
			os.Exit(0)
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func createTrayWindow() bool {
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString(trayClassName)
	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		WndProc:   wndProcCallback,
		Instance:  hInstance,
		ClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	empty, _ := syscall.UTF16PtrFromString("")
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(empty)),
		0,
		0, 0, 0, 0,
		hwndMessage,
		0,
		hInstance,
		0,
	)
	if hwnd == 0 {
		return false
	}
	trayHwnd = hwnd
	return true
}

func showMainWindow() {
	if windowHandle == 0 {
		return
	}
	procShowWindow.Call(windowHandle, SW_SHOW)
	procSetForegroundWindow.Call(windowHandle)
}

func hideMainWindow() {
	if windowHandle == 0 {
		return
	}
	procShowWindow.Call(windowHandle, SW_HIDE_VAL)
}

func showTrayMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	showPtr, _ := syscall.UTF16PtrFromString("表示")
	exitPtr, _ := syscall.UTF16PtrFromString("終了")
	procAppendMenuW.Call(menu, MF_STRING, IDM_SHOW, uintptr(unsafe.Pointer(showPtr)))
	procAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	procAppendMenuW.Call(menu, MF_STRING, IDM_EXIT, uintptr(unsafe.Pointer(exitPtr)))

	var pt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// TrackPopupMenu 前に SetForegroundWindow を呼ばないと、メニュー外クリックで閉じない
	procSetForegroundWindow.Call(hwnd)
	procTrackPopupMenu.Call(menu, TPM_RIGHTBUTTON, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
	procPostMessageW.Call(hwnd, 0, 0, 0)
	procDestroyMenu.Call(menu)
}

func pct(used, limit int64) int {
	if limit <= 0 {
		return 0
	}
	p := int(used * 100 / limit)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return p
}

func updateTrayFromSnapshot() {
	snap := getUsageSnapshot()
	pct5h := pct(snap.FiveHour.Tokens, snap.FiveHour.LimitTokens)
	pct7d := pct(snap.SevenDay.Tokens, snap.SevenDay.LimitTokens)
	hIcon := generateTrayIcon(pct5h, pct7d)
	if hIcon == 0 {
		return
	}
	setTrayIcon(hIcon, pct5h, pct7d)
}

func generateTrayIcon(pct5h, pct7d int) uintptr {
	img := image.NewRGBA(image.Rect(0, 0, trayIconSize, trayIconSize))

	var bg, fg color.RGBA
	switch {
	case pct7d <= 50:
		bg = color.RGBA{144, 238, 144, 255} // light green
		fg = color.RGBA{0, 0, 0, 255}
	case pct7d <= 80:
		bg = color.RGBA{255, 179, 71, 255} // light orange
		fg = color.RGBA{0, 0, 0, 255}
	default:
		bg = color.RGBA{230, 70, 70, 255} // red
		fg = color.RGBA{255, 255, 255, 255}
	}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	val := pct5h
	if val > 99 {
		val = 99
	}
	text := strconv.Itoa(val)
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(fg),
		Face: basicfont.Face7x13,
	}
	w := drawer.MeasureString(text).Round()
	x := (trayIconSize - w) / 2
	if x < 0 {
		x = 0
	}
	drawer.Dot = fixed.P(x, 12)
	drawer.DrawString(text)

	bgra := make([]byte, len(img.Pix))
	for i := 0; i < len(img.Pix); i += 4 {
		bgra[i+0] = img.Pix[i+2]
		bgra[i+1] = img.Pix[i+1]
		bgra[i+2] = img.Pix[i+0]
		bgra[i+3] = img.Pix[i+3]
	}

	hbmColor, _, _ := procCreateBitmap.Call(trayIconSize, trayIconSize, 1, 32, uintptr(unsafe.Pointer(&bgra[0])))
	if hbmColor == 0 {
		return 0
	}
	mask := make([]byte, 2*trayIconSize)
	hbmMask, _, _ := procCreateBitmap.Call(trayIconSize, trayIconSize, 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	if hbmMask == 0 {
		procDeleteObject.Call(hbmColor)
		return 0
	}

	info := iconInfo{FIcon: 1, HbmMask: hbmMask, HbmColor: hbmColor}
	hIcon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))

	procDeleteObject.Call(hbmColor)
	procDeleteObject.Call(hbmMask)
	return hIcon
}

func fillNotifyIconData(nid *notifyIconData, tip string) {
	nid.CbSize = uint32(unsafe.Sizeof(*nid))
	nid.HWnd = trayHwnd
	nid.UID = trayIconID
	nid.UCallbackMessage = WM_TRAY
	if tip != "" {
		tipUtf16, err := syscall.UTF16FromString(tip)
		if err == nil {
			n := len(tipUtf16)
			if n > len(nid.SzTip) {
				n = len(nid.SzTip)
			}
			copy(nid.SzTip[:], tipUtf16[:n])
		}
	}
}

func addTrayIcon() {
	trayMu.Lock()
	defer trayMu.Unlock()
	if trayAdded {
		return
	}
	hIcon := generateTrayIcon(0, 0)
	var nid notifyIconData
	fillNotifyIconData(&nid, "Claude モニター")
	nid.UFlags = NIF_ICON | NIF_TIP | NIF_MESSAGE
	nid.HIcon = hIcon
	procShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	currentIcon = hIcon
	trayAdded = true
}

func setTrayIcon(hIcon uintptr, pct5h, pct7d int) {
	trayMu.Lock()
	defer trayMu.Unlock()
	if !trayAdded {
		return
	}
	var nid notifyIconData
	fillNotifyIconData(&nid, fmt.Sprintf("Claude モニター — 5h: %d%% / 7d: %d%%", pct5h, pct7d))
	nid.UFlags = NIF_ICON | NIF_TIP | NIF_MESSAGE
	nid.HIcon = hIcon
	procShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&nid)))

	old := currentIcon
	currentIcon = hIcon
	if old != 0 && old != hIcon {
		procDestroyIcon.Call(old)
	}
}

func removeTrayIcon() {
	trayMu.Lock()
	defer trayMu.Unlock()
	if !trayAdded {
		return
	}
	var nid notifyIconData
	fillNotifyIconData(&nid, "")
	procShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	if currentIcon != 0 {
		procDestroyIcon.Call(currentIcon)
		currentIcon = 0
	}
	trayAdded = false
}
