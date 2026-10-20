package app

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupSaveWriterTest 把用户数据目录指到临时目录（saveDir() 依赖它）。
func setupSaveWriterTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MABI_WORK_DIR", dir)
	// 每个用例用独立的 App，避免 writer 状态互相污染。
	return dir
}

func readSavedTargets(t *testing.T, path string) SaveFileData {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip %s: %v", path, err)
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("gunzip %s: %v", path, err)
	}
	var parsed SaveFileData
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return parsed
}

// 保存任务入队后立即返回，写盘在后台完成；flush 之后文件必须已经存在且内容完整。
func TestSaveWriterWritesFileInBackground(t *testing.T) {
	dir := setupSaveWriterTest(t)
	app := &App{}

	filePath := filepath.Join(dir, "save", "2026-01-01_00-00-01_测试.json.gz")
	app.enqueueSaveTask(saveTask{
		saveData: SaveFileData{Targets: []targetExport{
			{TargetID: "1", TargetName: "目标A", TotalDamage: 111},
			{TargetID: "2", TargetName: "目标B", TotalDamage: 222},
		}},
		filePath: filePath,
		saveName: "测试",
	})

	if !app.flushPendingSaves(5 * time.Second) {
		t.Fatal("flush 超时：后台保存没有完成")
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "save", "*.json.gz"))
	if len(matches) != 1 {
		t.Fatalf("落盘文件数 = %d, want 1 (%v)", len(matches), matches)
	}
	parsed := readSavedTargets(t, matches[0])
	if len(parsed.Targets) != 2 || parsed.Targets[0].TargetName != "目标A" || parsed.Targets[1].TotalDamage != 222 {
		t.Fatalf("落盘内容不符合预期: %+v", parsed.Targets)
	}
}

// 连续排定多次保存时，每个任务各写各的文件，不互相覆盖（名字按时间戳唯一化）。
func TestSaveWriterKeepsEachTaskOnItsOwnFile(t *testing.T) {
	dir := setupSaveWriterTest(t)
	app := &App{}

	samePath := filepath.Join(dir, "save", "2026-01-01_00-00-02_同名.json.gz")
	for i := 1; i <= 3; i++ {
		app.enqueueSaveTask(saveTask{
			saveData: SaveFileData{Targets: []targetExport{{TargetID: string(rune('0' + i))}}},
			filePath: samePath,
			saveName: "同名",
		})
	}
	if !app.flushPendingSaves(5 * time.Second) {
		t.Fatal("flush 超时")
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "save", "*.json.gz"))
	if len(matches) != 3 {
		t.Fatalf("落盘文件数 = %d, want 3 (%v)", len(matches), matches)
	}
	seen := map[string]bool{}
	for _, m := range matches {
		parsed := readSavedTargets(t, m)
		if len(parsed.Targets) != 1 {
			t.Fatalf("%s 内容异常: %+v", m, parsed.Targets)
		}
		seen[parsed.Targets[0].TargetID] = true
	}
	if len(seen) != 3 {
		t.Fatalf("三个任务的内容应各不相同，实际 %v", seen)
	}
}

// 写盘彻底失败时要记住起点，下一次保存把起点回退，避免这段战斗记录无声丢失。
func TestSaveWriterRecordsFailureForRetry(t *testing.T) {
	dir := setupSaveWriterTest(t)
	app := &App{}

	// Windows 下文件名含 <> 无法创建，writeSaveFile 必然失败。
	badPath := filepath.Join(dir, "save", "bad<>name.json.gz")
	app.enqueueSaveTask(saveTask{
		saveData: SaveFileData{Targets: []targetExport{{TargetID: "1"}}},
		filePath: badPath,
		saveName: "失败用例",
		sinceSeq: 42,
	})
	if !app.flushPendingSaves(10 * time.Second) {
		t.Fatal("flush 超时（重试应很快结束）")
	}
	if got := app.pendingSaveSince(); got != 42 {
		t.Fatalf("失败起点 = %d, want 42", got)
	}

	// 起点更晚的成功保存并没有覆盖失败那段，标记应保留。
	app.enqueueSaveTask(saveTask{
		saveData: SaveFileData{Targets: []targetExport{{TargetID: "2"}}},
		filePath: filepath.Join(dir, "save", "good-later.json.gz"),
		saveName: "更晚的成功保存",
		sinceSeq: 100,
	})
	if !app.flushPendingSaves(10 * time.Second) {
		t.Fatal("flush 超时")
	}
	if got := app.pendingSaveSince(); got != 42 {
		t.Fatalf("更晚的起点不应清除标记，实际 = %d", got)
	}

	// 新快照不能替代原失败快照，即使它使用相同的起点序号。
	app.enqueueSaveTask(saveTask{
		saveData: SaveFileData{Targets: []targetExport{{TargetID: "3"}}},
		filePath: filepath.Join(dir, "save", "good-retry.json.gz"),
		saveName: "回退成功",
		sinceSeq: 42,
	})
	if !app.flushPendingSaves(10 * time.Second) {
		t.Fatal("flush 超时")
	}
	if got := app.pendingSaveSince(); got != 42 {
		t.Fatalf("原失败快照应保留，实际起点 = %d", got)
	}
}

func TestSaveWriterRetainsFailedSnapshotAfterLiveStateIsCleared(t *testing.T) {
	dir := setupSaveWriterTest(t)
	app := &App{}
	badPath := filepath.Join(dir, "save", "bad<>name.json.gz")
	app.enqueueSaveTask(saveTask{
		saveData: SaveFileData{Targets: []targetExport{{TargetID: "preserved", TotalDamage: 900}}},
		filePath: badPath,
		saveName: "失败后清空",
		sinceSeq: 42,
	})
	if !app.flushPendingSaves(10 * time.Second) {
		t.Fatal("flush 超时")
	}

	app.mu.Lock()
	app.clearDamageStateUnsafe()
	app.mu.Unlock()

	writer := app.saveWriter
	writer.mu.Lock()
	if len(writer.failedTasks) != 1 || writer.failedTasks[0].saveData.Targets[0].TargetID != "preserved" {
		writer.mu.Unlock()
		t.Fatalf("完整失败快照未保留: %+v", writer.failedTasks)
	}
	writer.failedTasks[0].filePath = filepath.Join(dir, "save", "recovered.json.gz")
	writer.mu.Unlock()

	if !app.flushPendingSaves(10 * time.Second) {
		t.Fatal("重试失败快照超时")
	}
	parsed := readSavedTargets(t, filepath.Join(dir, "save", "recovered.json.gz"))
	if len(parsed.Targets) != 1 || parsed.Targets[0].TargetID != "preserved" || parsed.Targets[0].TotalDamage != 900 {
		t.Fatalf("恢复保存的内容错误: %+v", parsed.Targets)
	}
	if got := app.pendingSaveSince(); got >= 0 {
		t.Fatalf("成功重试后仍有失败标记: %d", got)
	}
}

// 没有排定任务时 flush 应立即成功，退出流程不会被无谓拖住。
func TestSaveWriterFlushWithoutTasks(t *testing.T) {
	setupSaveWriterTest(t)
	app := &App{}

	start := time.Now()
	if !app.flushPendingSaves(time.Second) {
		t.Fatal("空队列 flush 应成功")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("空队列 flush 耗时过长: %v", elapsed)
	}
}
