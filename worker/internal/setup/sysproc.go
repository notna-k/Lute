//go:build !windows

package setup

import (
	"os/exec"
	"syscall"
)

// detach puts the agent in its own session so it outlives the terminal that ran setup.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
