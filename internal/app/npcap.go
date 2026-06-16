package app

import (
	"blonymonitorv2/internal/pcaputil"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const npcapDownloadURL = "https://npcap.com/#download"

// NpcapStatus Npcap 检测结果。
type NpcapStatus struct {
	Installed bool   `json:"installed"`
	Message   string `json:"message"`
}

func getNpcapStatus() NpcapStatus {
	installed, message := pcaputil.CheckNpcap()
	return NpcapStatus{
		Installed: installed,
		Message:   message,
	}
}

func (a *App) reportNpcapMissingIfNeeded() bool {
	status := getNpcapStatus()
	if status.Installed {
		return false
	}

	a.setStatus("需要安装 Npcap")
	runtime.EventsEmit(a.ctx, "npcapMissing", status)
	return true
}

// GetNpcapStatus 返回当前 Npcap 状态。
func (a *App) GetNpcapStatus() NpcapStatus {
	return getNpcapStatus()
}

// OpenNpcapDownloadPage 在浏览器中打开 Npcap 下载页。
func (a *App) OpenNpcapDownloadPage() {
	runtime.BrowserOpenURL(a.ctx, npcapDownloadURL)
}

// RecheckNpcap 重新检测 Npcap，安装成功后自动开始抓包。
func (a *App) RecheckNpcap() NpcapStatus {
	status := getNpcapStatus()
	if status.Installed {
		a.setStatus("正在查找网卡...")
		runtime.EventsEmit(a.ctx, "npcapReady", status)
		go a.startCaptureWithMode()
		return status
	}

	runtime.EventsEmit(a.ctx, "npcapMissing", status)
	return status
}
