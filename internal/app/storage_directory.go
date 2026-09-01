package app

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func ensureDirectoryForOpen(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}
	return nil
}

// OpenSaveDir opens the persistent battle-history directory in the system file manager.
func (a *App) OpenSaveDir() error {
	dir := a.saveDir()
	if err := ensureDirectoryForOpen(dir); err != nil {
		return err
	}

	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("explorer.exe", dir)
	case "darwin":
		command = exec.Command("open", dir)
	case "linux":
		command = exec.Command("xdg-open", dir)
	default:
		return fmt.Errorf("opening storage directory is unsupported on %s", runtime.GOOS)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("open storage directory: %w", err)
	}
	return nil
}
