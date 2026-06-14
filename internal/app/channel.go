package app

import (
	"github.com/gopacket/gopacket/pcap"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"blonymonitorv2/internal/constants"
)

// GetChannelName 获取当前频道名称 (供前端调用)
func (a *App) GetChannelName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.channelName
}

// GetAllChannels 获取所有频道列表 (供前端调用)
func (a *App) GetAllChannels() []constants.ChannelInfo {
	return constants.GetAllChannels()
}

// GetChannelConfig 获取完整频道配置 (供前端调用)
func (a *App) GetChannelConfig() constants.ChannelConfig {
	return constants.GetChannelConfig()
}

// GetAutoDetect 获取是否自动检测频道 (供前端调用)
func (a *App) GetAutoDetect() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.autoDetect
}

// GetSelectedChannel 获取当前选择的频道号 (供前端调用)
func (a *App) GetSelectedChannel() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.selectedChannel
}

// SetAutoDetect 设置是否自动检测频道 (供前端调用)
func (a *App) SetAutoDetect(auto bool) {
	a.mu.Lock()
	a.autoDetect = auto
	if auto {
		a.selectedChannel = 0
	}
	a.mu.Unlock()

	// 重启抓包
	a.restartCapture()
}

// SetChannel 手动设置频道 (供前端调用)
func (a *App) SetChannel(channel int) {
	if channel < 1 || channel > 20 {
		return
	}

	a.mu.Lock()
	a.autoDetect = false
	a.selectedChannel = channel
	a.channelName = constants.GetChannelName(constants.ChannelMap[channel].IP, constants.ChannelMap[channel].Port)
	a.mu.Unlock()

	// 通知前端频道变更
	runtime.EventsEmit(a.ctx, "channel", a.channelName)
	runtime.EventsEmit(a.ctx, "autoDetectChanged", false)

	// 重启抓包
	a.restartCapture()
}

// GetAcceleratorMode 获取是否启用加速器兼容模式 (供前端调用)
func (a *App) GetAcceleratorMode() bool {
	return constants.AcceleratorMode
}

// SetAcceleratorMode 设置加速器兼容模式 (供前端调用)
func (a *App) SetAcceleratorMode(enabled bool) {
	logger.Printf("切换加速器模式: %v\n", enabled)

	// 设置加速器模式
	constants.SetAcceleratorMode(enabled)

	// 通知前端模式变更
	runtime.EventsEmit(a.ctx, "acceleratorModeChanged", enabled)

	// 重启抓包以应用新的过滤器
	a.restartCapture()
}

// GetAccelerators 获取所有加速器列表 (供前端调用)
func (a *App) GetAccelerators() []constants.AcceleratorInfo {
	return constants.GetAccelerators()
}

// GetSelectedAccelerator 获取当前选择的加速器 (供前端调用)
func (a *App) GetSelectedAccelerator() string {
	return constants.GetSelectedAccelerator()
}

// SetAccelerator 设置当前选择的加速器 (供前端调用)
func (a *App) SetAccelerator(id string) bool {
	logger.Printf("切换加速器: %s\n", id)

	if constants.SetSelectedAccelerator(id) {
		// 通知前端加速器变更
		runtime.EventsEmit(a.ctx, "acceleratorChanged", id)

		// 重启抓包以应用新的配置
		a.restartCapture()
		return true
	}
	return false
}

// NicInfo 网卡信息
type NicInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GetAllNics 获取所有网卡列表 (供前端调用)
func (a *App) GetAllNics() []NicInfo {
	nics, err := pcap.FindAllDevs()
	if err != nil {
		logger.Printf("获取网卡列表失败: %v\n", err)
		return []NicInfo{}
	}

	result := make([]NicInfo, 0, len(nics))
	for _, nic := range nics {
		result = append(result, NicInfo{
			Name:        nic.Name,
			Description: nic.Description,
		})
	}
	return result
}

// GetManualNic 获取手动选择的网卡 (供前端调用)
func (a *App) GetManualNic() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.manualNic
}

// SetManualNic 设置手动选择的网卡 (供前端调用)
func (a *App) SetManualNic(nicName string) {
	logger.Printf("设置手动网卡: %s\n", nicName)

	a.mu.Lock()
	a.manualNic = nicName
	// 手动选择网卡时，禁用自动检测
	a.autoDetect = false
	a.mu.Unlock()

	// 通知前端网卡变更
	runtime.EventsEmit(a.ctx, "manualNicChanged", nicName)

	// 重启抓包以应用新的配置
	a.restartCapture()
}
