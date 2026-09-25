//go:build windows

package metrics

func rootDiskGB() (used, total float64) { return 0, 0 }
