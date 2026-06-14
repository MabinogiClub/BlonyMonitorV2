//go:build windows

package app

import (
	"context"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	procGetWindowLongW             = user32.NewProc("GetWindowLongW")
	procSetWindowLongW             = user32.NewProc("SetWindowLongW")
	procFindWindowW                = user32.NewProc("FindWindowW")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
)

const (
	WS_EX_LAYERED     uintptr = 0x00080000
	WS_EX_TRANSPARENT uintptr = 0x00000020
	LWA_ALPHA         uintptr = 0x00000002
	titlebarHeight    int32   = 28 // 标题栏高度
)

// GWL_EXSTYLE = -20, 需要转换为 uintptr
var gwlExStyle = ^uintptr(19) // -20 的补码表示

// RECT Windows 矩形结构
type RECT struct {
	Left, Top, Right, Bottom int32
}

// POINT Windows 点结构
type POINT struct {
	X, Y int32
}

// 缓存窗口句柄
var cachedHwnd uintptr

// getWindowHandle 获取当前窗口句柄
func getWindowHandle() uintptr {
	if cachedHwnd != 0 {
		return cachedHwnd
	}
	// 通过窗口标题查找（尝试多个可能的标题）
	titles := []string{"BlonyMonitor", "伤害统计"}
	for _, titleStr := range titles {
		title, _ := syscall.UTF16PtrFromString(titleStr)
		hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
		if hwnd != 0 {
			cachedHwnd = hwnd
			return hwnd
		}
	}
	return 0
}

// invalidateWindowHandle 使窗口句柄缓存失效
func invalidateWindowHandle() {
	cachedHwnd = 0
}

// setClickThroughRaw 直接设置穿透状态（不检查）
func setClickThroughRaw(hwnd uintptr, enabled bool) {
	if hwnd == 0 {
		return
	}

	// 获取当前扩展样式
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)

	if enabled {
		// 添加穿透样式
		newStyle := exStyle | WS_EX_LAYERED | WS_EX_TRANSPARENT
		procSetWindowLongW.Call(hwnd, gwlExStyle, newStyle)
	} else {
		// 移除穿透样式
		newStyle := exStyle &^ WS_EX_TRANSPARENT
		procSetWindowLongW.Call(hwnd, gwlExStyle, newStyle)
	}
}

// setClickThroughEnabled 设置鼠标穿透
func setClickThroughEnabled(enabled bool) {
	hwnd := getWindowHandle()
	setClickThroughRaw(hwnd, enabled)
}

// setWindowOpacity 设置窗口透明度 (0-255, 255为完全不透明)
func setWindowOpacity(opacity uint8) {
	hwnd := getWindowHandle()
	if hwnd == 0 {
		// 如果找不到窗口句柄，尝试清除缓存并重试一次
		invalidateWindowHandle()
		hwnd = getWindowHandle()
		if hwnd == 0 {
			// 仍然找不到，记录错误但不崩溃
			return
		}
	}

	// 确保窗口有 WS_EX_LAYERED 样式
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)
	if exStyle&WS_EX_LAYERED == 0 {
		newStyle := exStyle | WS_EX_LAYERED
		procSetWindowLongW.Call(hwnd, gwlExStyle, newStyle)
	}

	// 设置透明度
	procSetLayeredWindowAttributes.Call(hwnd, 0, uintptr(opacity), LWA_ALPHA)
}

// isCursorInTitlebar 检查鼠标是否在标题栏区域
func isCursorInTitlebar(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}

	var rect RECT
	var cursor POINT

	// 获取窗口位置
	ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		return false
	}

	// 获取鼠标位置
	ret, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	if ret == 0 {
		return false
	}

	// 检查鼠标是否在标题栏区域
	return cursor.X >= rect.Left && cursor.X <= rect.Right &&
		cursor.Y >= rect.Top && cursor.Y <= rect.Top+titlebarHeight
}

// startClickThroughMonitor 启动穿透监控协程
func startClickThroughMonitor(ctx context.Context, app *App) {
	ticker := time.NewTicker(20 * time.Millisecond) // 20ms 检测一次，提高响应速度
	defer ticker.Stop()

	hwnd := uintptr(0)
	lastInTitlebar := false
	lastClickThrough := false
	retryCount := 0
	const maxRetries = 100 // 最多重试100次（2秒）

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 检查是否开启了穿透模式
			app.mu.RLock()
			clickThrough := app.clickThrough
			app.mu.RUnlock()

			// 获取窗口句柄（缓存）
			if hwnd == 0 {
				hwnd = getWindowHandle()
				if hwnd == 0 {
					// 窗口句柄未找到，继续重试
					retryCount++
					if retryCount >= maxRetries {
						// 超过最大重试次数，重置计数器继续尝试
						retryCount = 0
					}
					continue
				}
				retryCount = 0 // 找到句柄后重置计数器
			}

			// 检测到穿透模式状态变化
			if clickThrough != lastClickThrough {
				lastClickThrough = clickThrough
				if !clickThrough {
					// 穿透模式被关闭，确保窗口不是穿透状态
					setClickThroughRaw(hwnd, false)
					lastInTitlebar = false
					continue
				}
				// 穿透模式被开启，检查鼠标位置决定初始状态
				inTitlebar := isCursorInTitlebar(hwnd)
				if inTitlebar {
					// 鼠标在标题栏，不启用穿透
					setClickThroughRaw(hwnd, false)
					lastInTitlebar = true
				} else {
					// 鼠标不在标题栏，启用穿透
					setClickThroughRaw(hwnd, true)
					lastInTitlebar = false
				}
				continue
			}

			// 如果穿透模式未开启，跳过检测
			if !clickThrough {
				continue
			}

			// 检查鼠标是否在标题栏
			inTitlebar := isCursorInTitlebar(hwnd)

			if inTitlebar != lastInTitlebar {
				if inTitlebar {
					// 鼠标进入标题栏，临时关闭穿透
					setClickThroughRaw(hwnd, false)
				} else {
					// 鼠标离开标题栏，恢复穿透
					setClickThroughRaw(hwnd, true)
				}
				lastInTitlebar = inTitlebar
			}
		}
	}
}
