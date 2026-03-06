package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const DefaultQQDebURL = "https://dldir1v6.qq.com/qqfile/qq/QQNT/7516007c/linuxqq_3.2.25-45758_amd64.deb"

// HasLinuxQQ 判断并返回条件结果。
func HasLinuxQQ() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	info, err := os.Stat("/opt/QQ/qq")
	return err == nil && !info.IsDir()
}

// InstallLinuxQQ 实现该函数对应的业务逻辑。
func InstallLinuxQQ(url string, logf func(string, ...any)) error {
	if runtime.GOOS != "linux" {
		return errors.New("qq install is only supported on linux")
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return errors.New("qq install requires apt-get")
	}
	if os.Geteuid() != 0 {
		return errors.New("qq install requires root privileges")
	}

	debURL := strings.TrimSpace(url)
	if debURL == "" {
		debURL = DefaultQQDebURL
	}
	debPath := "/tmp/linuxqq-current.deb"
	if logf != nil {
		logf("qq: target package url %s", debURL)
	}

	steps := [][]string{
		{"bash", "-lc", "if [ -e /opt/QQ/qq ] || dpkg -s linuxqq >/dev/null 2>&1; then apt-get remove -y linuxqq || dpkg -r linuxqq || true; fi"},
		{"bash", "-lc", "rm -rf /opt/QQ"},
		{"bash", "-lc", fmt.Sprintf("curl -fL --retry 3 --connect-timeout 20 -o %s %q", debPath, debURL)},
		{"bash", "-lc", fmt.Sprintf("dpkg -i %s || (apt-get install -f -y && dpkg -i %s)", debPath, debPath)},
	}
	stepNames := []string{
		"remove old linuxqq package if present",
		"remove old /opt/QQ directory",
		"download qq deb package",
		"install qq deb package",
	}
	for i, step := range steps {
		if logf != nil {
			logf("qq: step %d/%d %s", i+1, len(steps), stepNames[i])
		}
		cmd := exec.Command(step[0], step[1:]...)
		out, err := cmd.CombinedOutput()
		if logf != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = "(no output)"
			}
			logf("qq: output %s", msg)
		}
		if err != nil {
			return fmt.Errorf("command failed: %s | %w | %s", strings.Join(step, " "), err, strings.TrimSpace(string(out)))
		}
	}

	if !HasLinuxQQ() {
		return errors.New("qq install completed but /opt/QQ/qq not found")
	}
	if logf != nil {
		logf("qq: install completed and /opt/QQ/qq detected")
	}
	return nil
}
