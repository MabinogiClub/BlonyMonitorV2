package pcaputil

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/gopacket/gopacket/pcap"
	"blonymonitorv2/internal/constants"
	"blonymonitorv2/internal/packet"
)

var logger = log.New(os.Stdout, "pcaputil ", log.LstdFlags|log.Lshortfile)

func FindNic() (string, error) {
	// 查找接收游戏服务器数据包的网络接口。
	packetWaitTime := time.Second * 1 // 减少等待时间，加快查找速度

	nics, err := pcap.FindAllDevs()
	if err != nil {
		logger.Println(err)
		return "", err
	}

	for _, nic := range nics {
		ctx, cancel := context.WithCancel(context.Background())

		r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
			Ctx:        ctx,
			NicName:    nic.Name,
			DisableLog: true, // 查找网卡时不需要日志
		})
		if err != nil {
			logger.Println("findNic failed", err, nic.Name)
			cancel()
			continue
		}

		select {
		case <-time.After(packetWaitTime):
			logger.Println("findNic timeout", nic.Name)
			cancel()
			r.Close()
			continue // 继续检查下一个网卡

		case <-r.PacketCh():
			logger.Println("findNic success:", nic.Name)
			cancel()
			r.Close()
			return nic.Name, nil // 找到后立即返回
		}
	}

	err = errors.New("findNic failed: not found")
	logger.Println(err)
	return "", err
}

// FindNicForChannel 查找指定频道的网络接口
func FindNicForChannel(channel int) (string, error) {
	packetWaitTime := time.Second * 1

	nics, err := pcap.FindAllDevs()
	if err != nil {
		logger.Println(err)
		return "", err
	}

	filter := constants.GetChannelFilter(channel)
	logger.Printf("查找频道 %d 的网卡，过滤器: %s\n", channel, filter)

	for _, nic := range nics {
		ctx, cancel := context.WithCancel(context.Background())

		r, err := packet.NewGameServerPacketReaderWithFilter(&packet.GameServerPacketReaderOpt{
			Ctx:        ctx,
			NicName:    nic.Name,
			DisableLog: true, // 查找网卡时不需要日志
		}, filter)
		if err != nil {
			logger.Println("findNicForChannel failed", err, nic.Name)
			cancel()
			continue
		}

		select {
		case <-time.After(packetWaitTime):
			logger.Println("findNicForChannel timeout", nic.Name)
			cancel()
			r.Close()
			continue

		case <-r.PacketCh():
			logger.Printf("findNicForChannel success: %s (channel %d)\n", nic.Name, channel)
			cancel()
			r.Close()
			return nic.Name, nil
		}
	}

	err = errors.New("findNicForChannel failed: not found")
	logger.Println(err)
	return "", err
}
