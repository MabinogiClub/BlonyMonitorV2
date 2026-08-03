package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const localEnvFileName = ".env"

func init() {
	applyBuildInjectOverrides()
	loadLocalEnvFile()
	applyUploadEnvOverrides()
}

func loadLocalEnvFile() {
	path, err := resolveLocalEnvPath()
	if err != nil || path == "" {
		return
	}

	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	values := parseEnvFile(file)
	applyUploadEnvMap(values)
}

func parseEnvFile(file *os.File) map[string]string {
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		value = strings.Trim(value, `"'`)
		values[key] = value
	}
	return values
}

func applyUploadEnvMap(values map[string]string) {
	if secret := strings.TrimSpace(values["BLONY_UPLOAD_SECRET"]); secret != "" && secret != UploadSecretPlaceholder {
		UploadSecret = secret
	}
	if endpoint := strings.TrimSpace(values["BLONY_UPLOAD_ENDPOINT"]); endpoint != "" {
		UploadEndpoint = endpoint
	}
	if endpoint := strings.TrimSpace(values["BLONY_RANKING_CONSENT_ENDPOINT"]); endpoint != "" {
		RankingConsentEndpoint = endpoint
	}
	if endpoint := strings.TrimSpace(values["BLONY_ANNOUNCEMENT_ENDPOINT"]); endpoint != "" {
		AnnouncementEndpoint = endpoint
	}
	if enabled := strings.TrimSpace(values["BLONY_UPLOAD_ENABLED"]); enabled != "" {
		UploadEnabled = enabled == "1" || strings.EqualFold(enabled, "true")
	}
	if keyword := strings.TrimSpace(values["BLONY_UPLOAD_DUNGEON_KEYWORD"]); keyword != "" {
		UploadDungeonKeyword = keyword
	}
}

func resolveLocalEnvPath() (string, error) {
	exeDir := ""
	if exePath, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exePath)
	}
	cwd, _ := os.Getwd()
	workDir := strings.TrimSpace(os.Getenv("MABI_WORK_DIR"))
	return resolveLocalEnvPathFrom(exeDir, cwd, workDir)
}

func resolveLocalEnvPathFrom(exeDir, cwd, workDir string) (string, error) {
	seen := make(map[string]struct{})
	candidates := make([]string, 0, 12)

	appendCandidate := func(path string) {
		path = filepath.Clean(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	appendUpwards := func(start string, maxDepth int) {
		dir := start
		for i := 0; i < maxDepth && dir != ""; i++ {
			appendCandidate(filepath.Join(dir, localEnvFileName))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if workDir != "" {
		appendCandidate(filepath.Join(workDir, localEnvFileName))
	}
	if exeDir != "" {
		appendUpwards(exeDir, 6)
	}
	if cwd != "" {
		appendUpwards(cwd, 6)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

// LocalEnvPath 返回实际加载到的 .env 路径（调试用）。
func LocalEnvPath() string {
	path, err := resolveLocalEnvPath()
	if err != nil {
		return ""
	}
	return path
}

func applyUploadEnvOverrides() {
	if secret := strings.TrimSpace(os.Getenv("BLONY_UPLOAD_SECRET")); secret != "" {
		UploadSecret = secret
	}
	if endpoint := strings.TrimSpace(os.Getenv("BLONY_UPLOAD_ENDPOINT")); endpoint != "" {
		UploadEndpoint = endpoint
	}
	if endpoint := strings.TrimSpace(os.Getenv("BLONY_RANKING_CONSENT_ENDPOINT")); endpoint != "" {
		RankingConsentEndpoint = endpoint
	}
	if endpoint := strings.TrimSpace(os.Getenv("BLONY_ANNOUNCEMENT_ENDPOINT")); endpoint != "" {
		AnnouncementEndpoint = endpoint
	}
	if enabled := strings.TrimSpace(os.Getenv("BLONY_UPLOAD_ENABLED")); enabled != "" {
		UploadEnabled = enabled == "1" || strings.EqualFold(enabled, "true")
	}
	if keyword := strings.TrimSpace(os.Getenv("BLONY_UPLOAD_DUNGEON_KEYWORD")); keyword != "" {
		UploadDungeonKeyword = keyword
	}
}
