package metrics

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Canonical metric keys (same as API Machine.Metrics and machine_snapshots).
const (
	KeyCpuLoad     = "cpu_load"
	KeyMemUsageMb  = "mem_usage_mb"
	KeyDiskUsedGb  = "disk_used_gb"
	KeyDiskTotalGb = "disk_total_gb"
)

// Collect returns only canonical metrics: cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb (all float64).
func Collect() map[string]interface{} {
	m := make(map[string]interface{})

	// cpu_load: 1-min load average (Linux) or 0
	if load1, _, _ := readLoadAvg(); load1 >= 0 {
		m[KeyCpuLoad] = load1
	} else {
		m[KeyCpuLoad] = 0.0
	}

	// mem_usage_mb: process memory in MB (e.g. Sys)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	m[KeyMemUsageMb] = float64(mem.Sys) / (1024 * 1024)

	// disk_used_gb, disk_total_gb: root filesystem in GB
	usedGb, totalGb := readRootDiskGB()
	m[KeyDiskUsedGb] = usedGb
	m[KeyDiskTotalGb] = totalGb

	return m
}

// readLoadAvg reads /proc/loadavg on Linux and returns (load1, load5, load15).
// Returns (-1, -1, -1) on non-Linux or on read/parse error.
func readLoadAvg() (float64, float64, float64) {
	if runtime.GOOS != "linux" {
		return -1, -1, -1
	}
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return -1, -1, -1
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return -1, -1, -1
	}
	load1, ok1 := parseFloat(fields[0])
	load5, ok5 := parseFloat(fields[1])
	load15, ok15 := parseFloat(fields[2])
	if !ok1 || !ok5 || !ok15 {
		return -1, -1, -1
	}
	return load1, load5, load15
}

func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}
