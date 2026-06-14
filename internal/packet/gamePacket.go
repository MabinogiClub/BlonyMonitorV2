package packet

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"blonymonitorv2/internal/config"
)

var logger *log.Logger

func init() {
	if !config.EnableFileLog {
		// 禁用日志输出
		logger = log.New(io.Discard, "", 0)
		return
	}

	// 获取日志输出目录
	var logDir string
	if workDir := os.Getenv("MABI_WORK_DIR"); workDir != "" {
		logDir = workDir
	} else {
		exePath, err := os.Executable()
		if err != nil {
			logger = log.New(os.Stdout, "packet ", log.LstdFlags|log.Lshortfile)
			return
		}
		logDir = filepath.Dir(exePath)
	}
	logPath := filepath.Join(logDir, "overlay.log")

	// 打开日志文件（追加模式）
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logger = log.New(os.Stdout, "packet ", log.LstdFlags|log.Lshortfile)
		return
	}

	logger = log.New(logFile, "packet ", log.LstdFlags|log.Lshortfile)
}

type GamePacket struct {
	At     time.Time
	Sign   uint8
	Length uint32
	Flag   uint8

	// raw packet
	IsShortPacket bool
	ShortBody     []byte

	// normal packet
	Op  uint32
	Id  uint64
	Msg Message

	// checksum uint32

	RawPacket []byte
}
