package app

import "time"

const timePrecisionScale int64 = 100

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
