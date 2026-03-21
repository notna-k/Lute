//go:build linux || darwin

package metrics

import "syscall"

// readRootDiskGB returns (used GB, total GB) for root filesystem, or (0, 0) on error.
func readRootDiskGB() (float64, float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, 0
	}
	blockSize := uint64(stat.Bsize)
	totalBytes := stat.Blocks * blockSize
	freeBytes := stat.Bfree * blockSize
	usedBytes := totalBytes - freeBytes
	const gb = 1024 * 1024 * 1024
	return float64(usedBytes) / gb, float64(totalBytes) / gb
}
