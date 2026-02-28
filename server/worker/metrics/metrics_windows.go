//go:build windows

package metrics

// readRootDiskGB returns (0, 0) on Windows; Statfs is not available.
func readRootDiskGB() (float64, float64) {
	return 0, 0
}
