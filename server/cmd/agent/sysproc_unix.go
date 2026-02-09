//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setDetachedProcessAttr sets process attributes to detach from terminal on Unix systems
func setDetachedProcessAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // detach from terminal (Unix/Linux/macOS)
	}
}

