// Package metrics samples host metrics for heartbeats.
package metrics

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Collect returns the metric keys the API stores on Machine.Metrics and machine_snapshots.
func Collect() map[string]float64 {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	usedGB, totalGB := rootDiskGB()
	return map[string]float64{
		"cpu_load":      loadAvg1(),
		"mem_usage_mb":  float64(mem.Sys) / (1024 * 1024),
		"disk_used_gb":  usedGB,
		"disk_total_gb": totalGB,
	}
}

// loadAvg1 is the 1-minute load average from /proc/loadavg, or 0 where that is unavailable.
func loadAvg1() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return load
}
