//go:build !windows

package services

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup 实现该函数对应的业务逻辑。
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// signalProcessGroup 实现该函数对应的业务逻辑。
func signalProcessGroup(proc *os.Process, sig syscall.Signal) error {
	if proc == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(proc.Pid)
	if err == nil && pgid > 0 {
		return syscall.Kill(-pgid, sig)
	}
	return proc.Signal(sig)
}
