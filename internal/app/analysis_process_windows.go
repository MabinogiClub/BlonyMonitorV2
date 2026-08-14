//go:build windows

package app

import (
	"time"

	"golang.org/x/sys/windows"
)

func processCPUTime() (time.Duration, error) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err != nil {
		return 0, err
	}
	return time.Duration(kernel.Nanoseconds() + user.Nanoseconds()), nil
}
