package app

import "github.com/wailsapp/wails/v2/pkg/runtime"

// SetClickThrough 设置鼠标穿透状态
func (a *App) SetClickThrough(enabled bool) {
	a.mu.Lock()
	a.clickThrough = enabled
	a.mu.Unlock()
	setClickThroughEnabled(enabled)
}

// GetClickThrough 获取鼠标穿透状态
func (a *App) GetClickThrough() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.clickThrough
}

// SetOpacity 设置窗口透明度 (0-100, 100为完全不透明)
func (a *App) SetOpacity(opacity int) {
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 100 {
		opacity = 100
	}
	a.mu.Lock()
	a.opacity = opacity
	a.mu.Unlock()
	// 转换为 0-255 范围
	alpha := uint8(opacity * 255 / 100)
	setWindowOpacity(alpha)
}

// GetOpacity 获取窗口透明度
func (a *App) GetOpacity() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.opacity
}

// SetAlwaysOnTop 设置窗口固定在前
func (a *App) SetAlwaysOnTop(enabled bool) {
	a.mu.Lock()
	a.alwaysOnTop = enabled
	a.mu.Unlock()
	runtime.WindowSetAlwaysOnTop(a.ctx, enabled)
}

// GetAlwaysOnTop 获取窗口固定在前状态
func (a *App) GetAlwaysOnTop() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.alwaysOnTop
}

// WindowSize 窗口大小结构
type WindowSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// SetWindowSize 设置窗口大小
func (a *App) SetWindowSize(width, height int) {
	runtime.WindowSetSize(a.ctx, width, height)
}

// GetWindowSize 获取窗口大小
func (a *App) GetWindowSize() WindowSize {
	w, h := runtime.WindowGetSize(a.ctx)
	return WindowSize{Width: w, Height: h}
}

// SetWindowMinSize 设置窗口最小尺寸
func (a *App) SetWindowMinSize(width, height int) {
	runtime.WindowSetMinSize(a.ctx, width, height)
}

// SetWindowMaxSize 设置窗口最大尺寸
func (a *App) SetWindowMaxSize(width, height int) {
	runtime.WindowSetMaxSize(a.ctx, width, height)
}

// SetWindowResizable 设置窗口是否可调整大小
func (a *App) SetWindowResizable(resizable bool) {
	if resizable {
		// 恢复可调整大小：设置合理的最大尺寸
		runtime.WindowSetMaxSize(a.ctx, 0, 0) // 0 表示无限制
	} else {
		// 禁用调整大小：将最大和最小尺寸设置为当前尺寸
		w, h := runtime.WindowGetSize(a.ctx)
		runtime.WindowSetMinSize(a.ctx, w, h)
		runtime.WindowSetMaxSize(a.ctx, w, h)
	}
}