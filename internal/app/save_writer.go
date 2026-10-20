package app

import (
	"path/filepath"
	"sync"
	"time"
)

// 战斗记录保存的后台执行者。
//
// 背景：一次保存 = 构建导出数据 + json.Marshal + gzip + 原子写盘，开启上传时还要
// 再来一次 JSON+gzip。过去这些都在 a.mu 写锁内同步做，实测 2.39MB 的存档会把整个
// 进程占住 1.27 秒（packet_max_ms 618~1266ms 与存档时刻一一对应），期间数据包处理
// 和统计读写全部停摆。
//
// 现在调用方（持写锁）只做两件事：构建快照、入队；序列化/写盘/上传准备由这里
// 串行执行，锁的持有时间不再包含这些慢操作。

// battleUploadPlan 是上传所需的、必须持锁读取的状态快照。
type battleUploadPlan struct {
	playerID   string
	playerName string
	saveName   string
}

type saveTask struct {
	saveData SaveFileData
	// filePath 是排定路径；真实落盘路径由 writeSaveFile 在后台做唯一化后确定，
	// 以 [Save] 日志里的路径为准。
	filePath string
	saveName string
	// sinceSeq 标记本次保存的起点，用于诊断失败范围。
	sinceSeq      int64
	upload        *battleUploadPlan
	retries       int
	retryOfFailed bool
}

const saveTaskMaxRetries = 2

type saveWriter struct {
	app *App

	mu      sync.Mutex
	queue   []saveTask
	running bool
	written uint64
	// failedSeq 记录最早一次彻底写盘失败的起点序号（<0 表示没有）。
	failedSeq   int64
	failedTasks []saveTask
}

// enqueue 追加任务；空闲时才拉起执行 goroutine，执行完队列为空即退出。
func (w *saveWriter) enqueue(t saveTask) {
	w.mu.Lock()
	w.enqueueFailedLocked()
	w.queue = append(w.queue, t)
	start := !w.running
	if start {
		w.running = true
	}
	w.mu.Unlock()
	if start {
		go w.loop()
	}
}

func (w *saveWriter) loop() {
	for {
		w.mu.Lock()
		if len(w.queue) == 0 {
			w.running = false
			w.mu.Unlock()
			return
		}
		task := w.queue[0]
		w.queue = w.queue[1:]
		w.mu.Unlock()

		err := w.app.executeSaveTask(task)
		w.mu.Lock()
		switch {
		case err == nil:
			// 只有失败快照自身成功后才能清除失败标记。
			if task.retryOfFailed {
				w.recomputeFailedSeqLocked()
			}
			w.written++
		case task.retries < saveTaskMaxRetries:
			// 瞬时失败（文件被占用等）重排一次，避免这段战斗记录无声丢失。
			task.retries++
			logger.Printf("[Save] 保存失败，重试 %d/%d: %v\n", task.retries, saveTaskMaxRetries, err)
			w.queue = append(w.queue, task)
		default:
			logger.Printf("[Save] 放弃保存（已重试 %d 次），下次保存会重新包含这段战斗记录: %v\n", saveTaskMaxRetries, err)
			if w.failedSeq < 0 || (task.sinceSeq > 0 && task.sinceSeq < w.failedSeq) {
				w.failedSeq = task.sinceSeq
			}
			w.failedTasks = append(w.failedTasks, task)
		}
		w.mu.Unlock()
	}
}

func (w *saveWriter) enqueueFailedLocked() {
	for _, failed := range w.failedTasks {
		failed.retryOfFailed = true
		failed.retries = 0
		w.queue = append(w.queue, failed)
	}
	w.failedTasks = nil
}

func (w *saveWriter) recomputeFailedSeqLocked() {
	w.failedSeq = -1
	update := func(task saveTask) {
		if w.failedSeq < 0 || (task.sinceSeq > 0 && task.sinceSeq < w.failedSeq) {
			w.failedSeq = task.sinceSeq
		}
	}
	for _, task := range w.failedTasks {
		update(task)
	}
	for _, task := range w.queue {
		if task.retryOfFailed {
			update(task)
		}
	}
}

func (w *saveWriter) retryFailed() {
	w.mu.Lock()
	if len(w.failedTasks) == 0 {
		w.mu.Unlock()
		return
	}
	w.enqueueFailedLocked()
	start := !w.running
	if start {
		w.running = true
	}
	w.mu.Unlock()
	if start {
		go w.loop()
	}
}

// flush 等待队列排空（程序退出前调用）。必须在锁外调用：
// 后台任务会读 App 状态，持写锁等待可能与它们互相阻塞。
func (w *saveWriter) flush(timeout time.Duration) bool {
	w.retryFailed()
	deadline := time.Now().Add(timeout)
	for {
		w.mu.Lock()
		idle := !w.running && len(w.queue) == 0
		w.mu.Unlock()
		if idle {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// enqueueSaveTask 把保存任务交给后台 writer（持锁调用）。
func (a *App) enqueueSaveTask(t saveTask) {
	a.saveWriterMu.Lock()
	if a.saveWriter == nil {
		a.saveWriter = &saveWriter{app: a, failedSeq: -1}
	}
	writer := a.saveWriter
	a.saveWriterMu.Unlock()
	writer.enqueue(t)
}

// pendingSaveSince 返回需要重新包含的最早起点序号（<0 表示没有写盘失败）。
func (a *App) pendingSaveSince() int64 {
	a.saveWriterMu.Lock()
	writer := a.saveWriter
	a.saveWriterMu.Unlock()
	if writer == nil {
		return -1
	}
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.failedSeq
}

// flushPendingSaves 等待所有已排定的保存写完；timeout 内没写完返回 false。
func (a *App) flushPendingSaves(timeout time.Duration) bool {
	a.saveWriterMu.Lock()
	writer := a.saveWriter
	a.saveWriterMu.Unlock()
	if writer == nil {
		return true
	}
	return writer.flush(timeout)
}

// executeSaveTask 后台串行执行一次保存：写盘 -> 记录/通知 -> 上传。
func (a *App) executeSaveTask(t saveTask) error {
	finalPath, err := writeSaveFile(t.filePath, t.saveData)
	if err != nil {
		logger.Printf("[Save] 写入失败 file=%q: %v\n", t.filePath, err)
		return err
	}
	recordLastBattleSave(t.saveName, filepath.Base(finalPath))
	a.emitHistorySaved(finalPath)
	logger.Printf("[Save] 已保存 %d 个目标 (%d 个 bossHP) -> %s\n",
		len(t.saveData.Targets), countBossHPTargets(t.saveData.Targets), finalPath)
	if t.upload != nil {
		a.runBattleUpload(*t.upload, t.saveData, finalPath)
	}
	return nil
}
