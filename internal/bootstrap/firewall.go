package bootstrap

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// TryAllowUFWPort 实现该函数对应的业务逻辑。
func TryAllowUFWPort(port int, logf func(string, ...any)) {
	if runtime.GOOS != "linux" {
		return
	}
	if port < 1 || port > 65535 {
		return
	}
	if _, err := exec.LookPath("ufw"); err != nil {
		if logf != nil {
			logf("firewall: ufw not found, skip allow tcp/%d", port)
		}
		return
	}
	cmd := exec.Command("ufw", "allow", fmt.Sprintf("%d/tcp", port))
	out, err := cmd.CombinedOutput()
	msg := strings.TrimSpace(string(out))
	if err != nil {
		if logf != nil {
			if msg == "" {
				msg = err.Error()
			}
			logf("firewall: ufw allow tcp/%d failed: %s", port, msg)
		}
		return
	}
	if logf != nil {
		if msg == "" {
			msg = "ok"
		}
		logf("firewall: ufw allow tcp/%d -> %s", port, msg)
	}
}

// TryDeleteUFWPort 实现该函数对应的业务逻辑。
func TryDeleteUFWPort(port int, logf func(string, ...any)) {
	if runtime.GOOS != "linux" {
		return
	}
	if port < 1 || port > 65535 {
		return
	}
	if _, err := exec.LookPath("ufw"); err != nil {
		if logf != nil {
			logf("firewall: ufw not found, skip delete allow tcp/%d", port)
		}
		return
	}
	cmd := exec.Command("ufw", "delete", "allow", fmt.Sprintf("%d/tcp", port))
	out, err := cmd.CombinedOutput()
	msg := strings.TrimSpace(string(out))
	if err != nil {
		if logf != nil {
			if msg == "" {
				msg = err.Error()
			}
			logf("firewall: ufw delete allow tcp/%d failed: %s", port, msg)
		}
		return
	}
	if logf != nil {
		if msg == "" {
			msg = "ok"
		}
		logf("firewall: ufw delete allow tcp/%d -> %s", port, msg)
	}
}
