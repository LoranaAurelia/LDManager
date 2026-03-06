//go:build windows

package services

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup 实现该函数对应的业务逻辑。
func setProcessGroup(cmd *exec.Cmd) {}

// signalProcessGroup 实现该函数对应的业务逻辑。
func signalProcessGroup(proc *os.Process, sig syscall.Signal) error {
	if proc == nil {
		return nil
	}
	return proc.Signal(sig)
}
