package metrics

import (
	"runtime"
	"strconv"
)

// Collect gathers lightweight system metrics
func Collect() map[string]string {
	m := map[string]string{
		"num_goroutine": strconv.Itoa(runtime.NumGoroutine()),
		"num_cpu":       strconv.Itoa(runtime.NumCPU()),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	m["mem_alloc_mb"] = strconv.FormatUint(mem.Alloc/1024/1024, 10)
	m["mem_sys_mb"] = strconv.FormatUint(mem.Sys/1024/1024, 10)

	return m
}

