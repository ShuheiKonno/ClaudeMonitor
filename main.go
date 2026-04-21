package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	procFindWindow                 = user32.NewProc("FindWindowW")
	procGetWindowLong              = user32.NewProc("GetWindowLongW")
	procSetWindowLong              = user32.NewProc("SetWindowLongW")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procGetDpiForWindow            = user32.NewProc("GetDpiForWindow")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procMonitorFromWindow          = user32.NewProc("MonitorFromWindow")
	procMonitorFromPoint           = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW            = user32.NewProc("GetMonitorInfoW")

	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW = kernel32.NewProc("CreateMutexW")
)

const (
	WS_CAPTION       = 0x00C00000
	WS_THICKFRAME    = 0x00040000
	WS_SYSMENU       = 0x00080000
	SWP_FRAMECHANGED = 0x0020
	SWP_NOMOVE       = 0x0002
	SWP_NOSIZE       = 0x0001
	SWP_NOZORDER     = 0x0004

	GWL_EXSTYLE   = -20
	GWL_STYLE     = -16
	WS_EX_LAYERED = 0x00080000
	LWA_ALPHA     = 0x00000002

	ALPHA_OPAQUE      = 255
	ALPHA_TRANSPARENT = 180

	SW_SHOW = 5

	MONITOR_DEFAULTTONEAREST = 0x00000002

	windowWidth  = 240
	windowHeight = 170
)

var windowHandle uintptr

type POINT struct {
	X, Y int32
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

var (
	dragMu        sync.Mutex
	dragStartCurX int32
	dragStartCurY int32
	dragStartWinX int32
	dragStartWinY int32
)

func setTransparent(enabled bool) {
	if windowHandle == 0 {
		return
	}
	gwlExStyle := int32(GWL_EXSTYLE)
	exStyle, _, _ := procGetWindowLong.Call(windowHandle, uintptr(gwlExStyle))
	newStyle := exStyle | WS_EX_LAYERED
	if newStyle != exStyle {
		procSetWindowLong.Call(windowHandle, uintptr(gwlExStyle), newStyle)
	}
	var alpha uintptr = ALPHA_OPAQUE
	if enabled {
		alpha = ALPHA_TRANSPARENT
	}
	procSetLayeredWindowAttributes.Call(windowHandle, 0, alpha, LWA_ALPHA)
}

func setTopmost(topmost bool) {
	if windowHandle == 0 {
		return
	}
	var insertAfter uintptr
	if topmost {
		insertAfter = ^uintptr(0) // HWND_TOPMOST (-1)
	} else {
		insertAfter = ^uintptr(1) // HWND_NOTOPMOST (-2)
	}
	procSetWindowPos.Call(windowHandle, insertAfter, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE)
}

func getWindowDpiScale() float64 {
	if windowHandle == 0 {
		return 1.0
	}
	dpi, _, _ := procGetDpiForWindow.Call(windowHandle)
	if dpi == 0 {
		return 1.0
	}
	return float64(dpi) / 96.0
}

func findMainWindow(title string) uintptr {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	return hwnd
}

func setupMainWindow(title string) bool {
	hwnd := findMainWindow(title)
	if hwnd == 0 {
		return false
	}
	windowHandle = hwnd

	gwlStyle := int32(GWL_STYLE)
	style, _, _ := procGetWindowLong.Call(hwnd, uintptr(gwlStyle))
	newStyle := style &^ (WS_CAPTION | WS_THICKFRAME | WS_SYSMENU)
	procSetWindowLong.Call(hwnd, uintptr(gwlStyle), newStyle)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, SWP_FRAMECHANGED|SWP_NOMOVE|SWP_NOZORDER|SWP_NOSIZE)

	// Alt+Tab / タイトルバー用にアプリアイコン（Claude オレンジの "C"）を適用。
	applyWindowIcon(hwnd)
	return true
}

func restoreWindowGeometry() bool {
	if windowHandle == 0 {
		return false
	}
	c := snapshotConfig()
	if !c.Window.Saved || c.Window.Width <= 0 || c.Window.Height <= 0 {
		return false
	}
	scale := getWindowDpiScale()
	if scale <= 0 {
		scale = 1.0
	}
	physX := int32(math.Round(float64(c.Window.X) * scale))
	physY := int32(math.Round(float64(c.Window.Y) * scale))
	// フレームレスでリサイズ不可のため W/H はコード定数を正とする（古い保存値を無視）
	physW := int32(math.Round(float64(windowWidth) * scale))
	physH := int32(math.Round(float64(windowHeight) * scale))

	// 保存された矩形中心のモニターを基準にクランプ（マルチモニター対応）。
	// MonitorFromPoint は POINT を値渡し（struct as argument）する必要があるため
	// uintptr にパックして渡す。
	center := POINT{X: physX + physW/2, Y: physY + physH/2}
	pointArg := uintptr(uint32(center.X)) | (uintptr(uint32(center.Y)) << 32)
	hmon, _, _ := procMonitorFromPoint.Call(pointArg, MONITOR_DEFAULTTONEAREST)
	if hmon != 0 {
		var mi MONITORINFO
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		if ok, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); ok != 0 {
			if physX < mi.RcWork.Left {
				physX = mi.RcWork.Left
			}
			if physX > mi.RcWork.Right-physW {
				physX = mi.RcWork.Right - physW
			}
			if physY < mi.RcWork.Top {
				physY = mi.RcWork.Top
			}
			if physY > mi.RcWork.Bottom-physH {
				physY = mi.RcWork.Bottom - physH
			}
		}
	}
	procSetWindowPos.Call(windowHandle, 0, uintptr(physX), uintptr(physY), uintptr(physW), uintptr(physH), SWP_NOZORDER)
	return true
}

func moveToBottomRight() {
	if windowHandle == 0 {
		return
	}
	hmon, _, _ := procMonitorFromWindow.Call(windowHandle, MONITOR_DEFAULTTONEAREST)
	if hmon == 0 {
		return
	}
	var mi MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return
	}
	var winRect RECT
	procGetWindowRect.Call(windowHandle, uintptr(unsafe.Pointer(&winRect)))
	w := winRect.Right - winRect.Left
	h := winRect.Bottom - winRect.Top
	scale := getWindowDpiScale()
	if scale <= 0 {
		scale = 1.0
	}
	margin := int32(math.Round(8 * scale))
	// widget-go と重ならないよう、右下より少し上に配置
	newX := mi.RcWork.Right - w - margin
	newY := mi.RcWork.Bottom - h - margin*2
	procSetWindowPos.Call(windowHandle, 0, uintptr(newX), uintptr(newY), 0, 0, SWP_NOSIZE|SWP_NOZORDER)
}

func persistCurrentWindow() {
	if windowHandle == 0 {
		return
	}
	var rect RECT
	procGetWindowRect.Call(windowHandle, uintptr(unsafe.Pointer(&rect)))
	scale := getWindowDpiScale()
	if scale <= 0 {
		scale = 1.0
	}
	logicalX := int32(math.Round(float64(rect.Left) / scale))
	logicalY := int32(math.Round(float64(rect.Top) / scale))
	logicalW := int32(math.Round(float64(rect.Right-rect.Left) / scale))
	logicalH := int32(math.Round(float64(rect.Bottom-rect.Top) / scale))
	mutateConfig(func(c *Config) {
		c.Window.X = logicalX
		c.Window.Y = logicalY
		c.Window.Width = logicalW
		c.Window.Height = logicalH
		c.Window.Saved = true
	})
}

func ensureSingleInstance(windowTitle string) bool {
	const errorAlreadyExists = 183
	mutexName, _ := syscall.UTF16PtrFromString("Global\\claude-monitor-single-instance-mutex")
	_, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	if errno, ok := err.(syscall.Errno); ok && errno == errorAlreadyExists {
		titlePtr, _ := syscall.UTF16PtrFromString(windowTitle)
		if hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr))); hwnd != 0 {
			procShowWindow.Call(hwnd, SW_SHOW)
			procSetForegroundWindow.Call(hwnd)
		}
		return false
	}
	return true
}

func main() {
	runtime.LockOSThread()

	windowTitle := "Claude モニター"
	if !ensureSingleInstance(windowTitle) {
		return
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = os.Getenv("APPDATA")
	}
	appRoot := filepath.Join(localAppData, "ClaudeMonitor")
	dataPath := filepath.Join(appRoot, "WebView2")
	_ = os.MkdirAll(dataPath, 0755)

	configPath = filepath.Join(appRoot, "config.json")
	loadConfig()

	startCollector()
	port, err := startServer()
	if err != nil {
		panic(fmt.Sprintf("failed to start server: %v", err))
	}
	startTray()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  dataPath,
		WindowOptions: webview2.WindowOptions{
			Title:  windowTitle,
			Width:  windowWidth,
			Height: windowHeight,
			Center: true,
		},
	})
	if w == nil {
		panic("Failed to create webview")
	}
	defer w.Destroy()

	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			if setupMainWindow(windowTitle) {
				if !restoreWindowGeometry() {
					moveToBottomRight()
				}
				time.AfterFunc(500*time.Millisecond, func() {
					if !restoreWindowGeometry() {
						moveToBottomRight()
					}
				})
				c := snapshotConfig()
				if c.Topmost {
					setTopmost(true)
				}
				if c.Transparent {
					setTransparent(true)
				}
				return
			}
		}
	}()

	w.SetSize(windowWidth, windowHeight, webview2.HintFixed)
	w.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port))
	w.Run()
}
