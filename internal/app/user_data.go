package app

import (
	"io"
	"os"
	"path/filepath"
)

// userDataDirectory is the persistent, user-owned data directory. MABI_WORK_DIR
// remains an explicit override for tests and portable development builds.
func userDataDirectory() string {
	if dir := os.Getenv("MABI_WORK_DIR"); dir != "" {
		return filepath.Clean(dir)
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "BlonyMonitorV2")
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "BlonyMonitorV2-data")
	}
	return "."
}

func ensureUserDataDirectory() string {
	dir := userDataDirectory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		if logger != nil {
			logger.Printf("[Storage] 创建用户数据目录失败 %q: %v", dir, err)
		}
	}
	return dir
}

func legacyExecutableDirectory() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return ""
}

func copyFileIfAbsent(source, destination string) error {
	if source == "" || destination == "" || filepath.Clean(source) == filepath.Clean(destination) {
		return nil
	}
	if _, err := os.Stat(destination); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	removeOnError := true
	defer func() {
		_ = out.Close()
		if removeOnError {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	removeOnError = false
	return nil
}

// migrateLegacyUserData preserves files written beside older executables.
// Existing AppData files always win, and legacy files are intentionally kept.
func migrateLegacyUserData() {
	if os.Getenv("MABI_WORK_DIR") != "" {
		return
	}
	migrateLegacyUserDataFrom(legacyExecutableDirectory(), ensureUserDataDirectory())
}

func migrateLegacyUserDataFrom(sourceDir, targetDir string) {
	if sourceDir == "" || filepath.Clean(sourceDir) == filepath.Clean(targetDir) {
		return
	}

	files := []string{
		"blonymonitor-analysis.log",
		"blonymonitor-analysis-settings.json",
		"blonymonitor-refresh-settings.json",
		"blonymonitor-farm-settings.json",
		"blonymonitor-farm-recommendation.json",
		"blonymonitor-farm-recommendation-capture.jsonl",
		"blonymonitor-sound-settings.json",
		"buff_order.json",
		"overlay.log",
	}
	for _, name := range files {
		if err := copyFileIfAbsent(filepath.Join(sourceDir, name), filepath.Join(targetDir, name)); err != nil && !os.IsNotExist(err) {
			logger.Printf("[Storage] 迁移文件失败 %q: %v", name, err)
		}
	}

	legacySaveDir := filepath.Join(sourceDir, "save")
	_ = filepath.WalkDir(legacySaveDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(legacySaveDir, path)
		if err != nil {
			return err
		}
		if err := copyFileIfAbsent(path, filepath.Join(targetDir, "save", rel)); err != nil {
			logger.Printf("[Storage] 迁移存档失败 %q: %v", rel, err)
		}
		return nil
	})
}
