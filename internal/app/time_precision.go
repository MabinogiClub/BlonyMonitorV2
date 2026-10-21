package app

import (
	"sync/atomic"
	"time"

	"blonymonitorv2/internal/packet"
)

const timePrecisionScale int64 = 100

// 战斗统计里的时间戳有两种来源：
//   - 伤害/命中记录用 pkt.At（数据包自带的抓包时刻），见 packetCentiseconds；
//   - 血量历史、死亡时间过去用本机时钟 time.Now()（处理时刻）。
//
// 实时抓包且处理及时时两者只差毫秒，但一旦数据包积压（战斗高峰期、保存存档时持锁
// 序列化等），处理时刻会比抓包时刻晚几秒到几分钟，于是同一条目标记录里 firstHit 落在
// 早的时间轴上、deathTime 落在晚的时间轴上，duration = deathTime - firstHit 被凭空撑大，
// DPS 随之偏低。
//
// 因此战斗相关的"现在"统一走 eventNow：以最近一个数据包的抓包时刻为基准，再加上自那
// 之后经过的本机时间。实时无积压时它等于本机时钟；有积压时它跟着处理进度推进，始终与
// 伤害记录处于同一条时间轴上。
var (
	lastPacketCentis     atomic.Int64 // 最近一个数据包的抓包时刻（厘秒）
	lastPacketWallCentis atomic.Int64 // 处理该数据包时的本机时刻（厘秒）
)

// notePacketTime 用数据包自带的抓包时刻推进事件时钟（只前进，不回退）。
func notePacketTime(pkt *packet.GamePacket) {
	if pkt == nil || pkt.At.IsZero() {
		return
	}
	cs := pkt.At.UnixMilli() / 10
	wall := nowCentiseconds()
	for {
		cur := lastPacketCentis.Load()
		if cs <= cur {
			return
		}
		if lastPacketCentis.CompareAndSwap(cur, cs) {
			lastPacketWallCentis.Store(wall)
			return
		}
	}
}

// eventNow 返回战斗统计使用的"现在"，与伤害记录（pkt.At）同源。
func eventNow() int64 {
	pktCentis := lastPacketCentis.Load()
	wallAtPacket := lastPacketWallCentis.Load()
	if pktCentis <= 0 || wallAtPacket <= 0 {
		return nowCentiseconds()
	}
	elapsed := nowCentiseconds() - wallAtPacket
	if elapsed < 0 {
		elapsed = 0
	}
	return pktCentis + elapsed
}

func nowCentiseconds() int64 {
	return time.Now().UnixMilli() / 10
}

func centisecondsToSeconds(t int64) float64 {
	return float64(t) / float64(timePrecisionScale)
}

func durationSeconds(start, end int64) float64 {
	duration := float64(end-start) / float64(timePrecisionScale)
	if duration < 0.01 {
		return 0.01
	}
	return duration
}
