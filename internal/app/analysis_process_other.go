//go:build !windows

package app

import (
	"errors"
	"time"
)

func processCPUTime() (time.Duration, error) {
	return 0, errors.New("process CPU sampling is only available on Windows")
}
