//go:build linux || darwin

package metrics

import "syscall"

func rootDiskGB() (used, total float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, 0
	}
	const gb = 1 << 30
	bs := uint64(stat.Bsize)
	return float64((stat.Blocks-stat.Bfree)*bs) / gb, float64(stat.Blocks*bs) / gb
}
