package tray

import (
	"context"
	_ "embed"

	"github.com/energye/systray"
)

//go:embed icon.ico
var iconData []byte

// TrayApp 系统托盘应用
type TrayApp struct {
	ctx        context.Context
	onDblClick func() // 双击回调（显示窗口）
	onQuit     func()
	mShow      *systray.MenuItem
	mQuit      *systray.MenuItem
	isHidden   bool
}

// NewTrayApp 创建托盘应用
func NewTrayApp(ctx context.Context) *TrayApp {
	return &TrayApp{
		ctx:      ctx,
		isHidden: false,
	}
}

// OnDblClick 设置双击回调（显示窗口）
func (t *TrayApp) OnDblClick(callback func()) {
	t.onDblClick = callback
}

// OnQuit 设置退出回调
func (t *TrayApp) OnQuit(callback func()) {
	t.onQuit = callback
}

// SetHidden 设置隐藏状态
func (t *TrayApp) SetHidden(hidden bool) {
	t.isHidden = hidden
}

// Run 运行托盘（阻塞）
func (t *TrayApp) Run() {
	systray.Run(t.onReady, t.onExit)
}

// RunAsync 异步运行托盘
func (t *TrayApp) RunAsync() {
	go systray.Run(t.onReady, t.onExit)
}

// Quit 退出托盘
func (t *TrayApp) Quit() {
	systray.Quit()
}

func (t *TrayApp) onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("伤害统计")
	systray.SetTooltip("伤害统计悬浮窗 - 双击显示窗口")

	// 设置左键双击事件（显示窗口）
	systray.SetOnDClick(func(menu systray.IMenu) {
		if t.onDblClick != nil {
			t.onDblClick()
		}
	})

	// 右键菜单：显示窗口
	t.mShow = systray.AddMenuItem("显示窗口", "显示主窗口")
	t.mShow.Click(func() {
		if t.onDblClick != nil {
			t.onDblClick()
		}
	})

	systray.AddSeparator()

	// 右键菜单：退出
	t.mQuit = systray.AddMenuItem("退出", "退出程序")
	t.mQuit.Click(func() {
		if t.onQuit != nil {
			t.onQuit()
		}
	})
}

func (t *TrayApp) onExit() {
	// 清理
}
