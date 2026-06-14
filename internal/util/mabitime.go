package util

import (
	"time"
)

const kstOffset = 9 * 60 * 60

var kst = time.FixedZone("KST", kstOffset)

func ParseMabiTime(t uint64) time.Time {
	t = t / 1000

	// c# time
	t -= 62135596800

	return time.Unix(int64(t), 0)
}
