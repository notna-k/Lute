//go:build windows

package setup

import "os/exec"

// detach is a no-op: a started Windows process already outlives its parent.
func detach(*exec.Cmd) {}
