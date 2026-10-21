package app

import (
	"fmt"
	"net"
	"net/http"
	httpprof "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	runtimepprof "runtime/pprof"
	"strconv"
	"strings"
	"time"
)

const (
	pprofTriggerFileName = "pprof.trigger"
	pprofDefaultSeconds  = 30
	pprofMaxSeconds      = 300
	pprofPollInterval    = time.Second
	pprofPreferredPort   = 6060
)

// startPprofEndpoint 在回环地址上启动 pprof HTTP 服务。
// 项目没有自有的 HTTP 服务，这里单独开一个仅监听 127.0.0.1 的口子，
// 方便用 `go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30` 抓取。
// 首选 6060 端口，被占用时自动退到随机端口并打印实际地址。
func startPprofEndpoint() {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", httpprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", httpprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", httpprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", httpprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", httpprof.Trace)

	for _, port := range []int{pprofPreferredPort, 0} {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			logger.Printf("[Pprof] 绑定 %s 失败: %v", addr, err)
			continue
		}
		logger.Printf("[Pprof] HTTP 服务已启动: http://%s/debug/pprof/", ln.Addr())
		go func() {
			_ = http.Serve(ln, mux)
		}()
		return
	}
}

// startPprofTriggerWatcher 轮询用户数据目录下的 pprof.trigger 文件。
// 运行中的某个节点想抓一段 profile 时，写入该文件（内容为采样秒数，可选）即可，
// 例如 `echo 60 > pprof.trigger`；采样完成后文件会被删除并输出 profile 文件路径。
func startPprofTriggerWatcher(dir string) {
	triggerPath := filepath.Join(dir, pprofTriggerFileName)
	go func() {
		ticker := time.NewTicker(pprofPollInterval)
		defer ticker.Stop()
		for range ticker.C {
			data, err := os.ReadFile(triggerPath)
			if err != nil {
				continue
			}
			_ = os.Remove(triggerPath)
			seconds := pprofDefaultSeconds
			if n, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && n > 0 {
				if n > pprofMaxSeconds {
					n = pprofMaxSeconds
				}
				seconds = n
			}
			runPprofCapture(dir, seconds)
		}
	}()
}

// runPprofCapture 采样 seconds 秒 CPU profile，并同时导出 heap 与 goroutine 快照。
func runPprofCapture(dir string, seconds int) {
	stamp := time.Now().Format("20060102_150405")
	cpuPath := filepath.Join(dir, fmt.Sprintf("pprof_cpu_%s.pprof", stamp))

	f, err := os.Create(cpuPath)
	if err != nil {
		logger.Printf("[Pprof] 创建 CPU profile 文件失败: %v", err)
		return
	}
	if err := runtimepprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		logger.Printf("[Pprof] 启动 CPU 采样失败: %v", err)
		return
	}
	logger.Printf("[Pprof] 开始 CPU 采样 %d 秒 -> %s", seconds, cpuPath)
	time.Sleep(time.Duration(seconds) * time.Second)
	runtimepprof.StopCPUProfile()
	_ = f.Close()

	heapPath := filepath.Join(dir, fmt.Sprintf("pprof_heap_%s.pprof", stamp))
	if hf, err := os.Create(heapPath); err == nil {
		runtime.GC()
		_ = runtimepprof.WriteHeapProfile(hf)
		_ = hf.Close()
	}

	goroutinePath := filepath.Join(dir, fmt.Sprintf("pprof_goroutine_%s.txt", stamp))
	if gf, err := os.Create(goroutinePath); err == nil {
		_ = runtimepprof.Lookup("goroutine").WriteTo(gf, 1)
		_ = gf.Close()
	}

	logger.Printf("[Pprof] 采样完成: cpu=%s heap=%s goroutine=%s", cpuPath, heapPath, goroutinePath)
}
