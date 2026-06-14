package constants

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

//go:embed channels.json
var channelsJSON []byte

//go:embed accelerators.json
var acceleratorsJSON []byte

// ChannelConfig JSON 配置结构
type ChannelConfig struct {
	Servers []ServerConfig `json:"servers"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	IPPrefix string          `json:"ipPrefix"`
	Channels []ChannelDetail `json:"channels"`
}

// ChannelDetail 频道详情
type ChannelDetail struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port uint16 `json:"port"`
}

// ChannelInfo 频道信息（兼容旧接口）
type ChannelInfo struct {
	Channel int    `json:"channel"`
	IP      string `json:"ip"`
	Port    uint16 `json:"port"`
}

// AcceleratorInfo 加速器信息
type AcceleratorInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port uint16 `json:"port"`
}

// AcceleratorConfig 加速器配置结构
type AcceleratorConfig struct {
	Accelerators []AcceleratorInfo `json:"accelerators"`
}

// 全局配置
var (
	config                  ChannelConfig
	acceleratorConfig       AcceleratorConfig
	ChannelMap              map[int]ChannelInfo
	PCAP_GAMESERVER_FILTER  string
	PCAP_ACCELERATOR_FILTER = "tcp" // 加速器兼容模式：抓取所有TCP流量，通过连接跟踪过滤
	SERVER_START_AT         = time.Now().Unix()
	AcceleratorMode         = false // 是否启用加速器兼容模式
	SelectedAccelerator     = "uu"  // 当前选择的加速器ID
)

func init() {
	// 解析频道 JSON 配置
	if err := json.Unmarshal(channelsJSON, &config); err != nil {
		panic("解析 channels.json 失败: " + err.Error())
	}

	// 解析加速器 JSON 配置
	if err := json.Unmarshal(acceleratorsJSON, &acceleratorConfig); err != nil {
		panic("解析 accelerators.json 失败: " + err.Error())
	}

	// 构建频道映射表
	ChannelMap = make(map[int]ChannelInfo)
	for _, server := range config.Servers {
		for _, ch := range server.Channels {
			ChannelMap[ch.ID] = ChannelInfo{
				Channel: ch.ID,
				IP:      ch.IP,
				Port:    ch.Port,
			}
		}
	}

	// 构建 BPF 过滤器
	buildPcapFilter()
}

// buildPcapFilter 从配置构建 pcap 过滤器
func buildPcapFilter() {
	hostSet := make(map[string]bool)
	portSet := make(map[uint16]bool)

	for _, server := range config.Servers {
		for _, ch := range server.Channels {
			hostSet[ch.IP] = true
			portSet[ch.Port] = true
		}
	}

	// 构建 IP 过滤部分
	hosts := make([]string, 0, len(hostSet))
	for h := range hostSet {
		hosts = append(hosts, h)
	}

	hostFilter := ""
	if len(hosts) == 1 {
		hostFilter = "src host " + hosts[0]
	} else {
		hostFilter = "src host ( " + strings.Join(hosts, " or ") + " )"
	}

	// 构建端口过滤部分
	ports := make([]string, 0, len(portSet))
	for p := range portSet {
		ports = append(ports, fmt.Sprintf("%d", p))
	}

	portFilter := ""
	if len(ports) == 1 {
		portFilter = "src port " + ports[0]
	} else {
		portFilter = "src port ( " + strings.Join(ports, " or ") + " )"
	}

	PCAP_GAMESERVER_FILTER = fmt.Sprint("tcp and ", hostFilter, " and ", portFilter)
}

// GetChannelConfig 获取完整频道配置（供前端使用）
func GetChannelConfig() ChannelConfig {
	return config
}

// GetAllChannels 获取所有频道列表
func GetAllChannels() []ChannelInfo {
	channels := make([]ChannelInfo, 0, len(ChannelMap))
	for i := 1; i <= len(ChannelMap); i++ {
		if ch, ok := ChannelMap[i]; ok {
			channels = append(channels, ch)
		}
	}
	return channels
}

// GetChannelFilter 获取指定频道的 BPF 过滤器
func GetChannelFilter(channel int) string {
	if info, ok := ChannelMap[channel]; ok {
		return fmt.Sprintf("tcp and src host %s and src port %d", info.IP, info.Port)
	}
	return PCAP_GAMESERVER_FILTER
}

// GetChannelName 根据服务器IP和端口获取频道名称
func GetChannelName(ip string, port uint16) string {
	for _, server := range config.Servers {
		for _, ch := range server.Channels {
			if ch.IP == ip && ch.Port == port {
				return fmt.Sprintf("[%s %s]", server.Name, ch.Name)
			}
		}
	}
	return ""
}

// GetChannelNumber 根据服务器IP和端口获取频道号
func GetChannelNumber(ip string, port uint16) int {
	for _, server := range config.Servers {
		for _, ch := range server.Channels {
			if ch.IP == ip && ch.Port == port {
				return ch.ID
			}
		}
	}
	return 0
}

// GetServerByIP 根据 IP 获取服务器信息
func GetServerByIP(ip string) *ServerConfig {
	for i := range config.Servers {
		if strings.HasPrefix(ip, config.Servers[i].IPPrefix) {
			return &config.Servers[i]
		}
	}
	return nil
}

// GetDisplayChannelNumber 获取显示用的频道号（1-10）
func GetDisplayChannelNumber(channelID int) int {
	if info, ok := ChannelMap[channelID]; ok {
		for _, server := range config.Servers {
			for _, ch := range server.Channels {
				if ch.ID == channelID {
					// 从频道名称中提取数字，如 "频道1" -> 1
					var num int
					fmt.Sscanf(ch.Name, "频道%d", &num)
					if num > 0 {
						return num
					}
					// 如果解析失败，使用 ID 计算
					if info.Port == 11020 {
						return (channelID-1)%5 + 1
					}
					return (channelID-1)%5 + 6
				}
			}
		}
	}
	return channelID
}

// GetCurrentFilter 获取当前模式的 BPF 过滤器
func GetCurrentFilter() string {
	if AcceleratorMode {
		// 根据选择的加速器生成具体的过滤器
		for _, acc := range acceleratorConfig.Accelerators {
			if acc.ID == SelectedAccelerator {
				// 端口为0表示不限制端口（用于动态端口的加速器，如UU）
				if acc.Port == 0 {
					// UU加速器使用动态端口响应，只过滤源IP
					return fmt.Sprintf("tcp and src host %s", acc.IP)
				}
				// 使用 src host 和 src port，只捕获从加速器（服务器端）发送的数据包
				// 这与非加速器模式一致，避免捕获客户端发送的加密数据包
				return fmt.Sprintf("tcp and src host %s and src port %d", acc.IP, acc.Port)
			}
		}
		// 如果没有找到匹配的加速器，使用通用过滤器
		return PCAP_ACCELERATOR_FILTER
	}
	return PCAP_GAMESERVER_FILTER
}

// SetAcceleratorMode 设置加速器兼容模式
func SetAcceleratorMode(enabled bool) {
	AcceleratorMode = enabled
}

// GetAccelerators 获取所有加速器列表
func GetAccelerators() []AcceleratorInfo {
	return acceleratorConfig.Accelerators
}

// GetSelectedAccelerator 获取当前选择的加速器
func GetSelectedAccelerator() string {
	return SelectedAccelerator
}

// SetSelectedAccelerator 设置当前选择的加速器
func SetSelectedAccelerator(id string) bool {
	for _, acc := range acceleratorConfig.Accelerators {
		if acc.ID == id {
			SelectedAccelerator = id
			return true
		}
	}
	return false
}

// GetAcceleratorInfo 根据ID获取加速器信息
func GetAcceleratorInfo(id string) *AcceleratorInfo {
	for i := range acceleratorConfig.Accelerators {
		if acceleratorConfig.Accelerators[i].ID == id {
			return &acceleratorConfig.Accelerators[i]
		}
	}
	return nil
}
