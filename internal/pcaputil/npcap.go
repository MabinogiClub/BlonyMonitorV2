package pcaputil

import (
	"strings"

	"github.com/gopacket/gopacket/pcap"
)

// CheckNpcap 检测 Npcap 是否可用。
func CheckNpcap() (bool, string) {
	_, err := pcap.FindAllDevs()
	if err == nil {
		return true, ""
	}
	if !isNpcapRelatedError(err) {
		return true, ""
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "npf driver") {
		return false, "Npcap 驱动未运行，请重新安装 Npcap"
	}
	return false, "未检测到 Npcap，抓包功能需要先安装"
}

func isNpcapRelatedError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	keywords := []string{
		"wpcap.dll",
		"npcap",
		"winpcap",
		"specified module could not be found",
		"npf driver",
		"can't load wpcap",
		"couldn't load wpcap",
	}
	for _, keyword := range keywords {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}
