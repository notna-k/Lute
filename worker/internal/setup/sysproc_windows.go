//go:build windows

package setup

import "os/exec"

// setDetachedProcessAttr is a no-op on Windows
// Windows processes run in background by default when started this way
func setDetachedProcessAttr(cmd *exec.Cmd) {
	// No special attributes needed on Windows
}

