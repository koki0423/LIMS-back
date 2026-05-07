//go:build windows

package printLabels

import (
	"os/exec"
	"syscall"
)

func setPlatformSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
