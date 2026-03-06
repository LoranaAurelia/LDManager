package bootstrap

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
)

var llbotAPTDependencies = []string{
	"ffmpeg",
	"xvfb",
	"x11-utils",
	"libgtk-3-0",
	"libxcb-xinerama0",
	"libgl1-mesa-dri",
	"libnotify4",
	"libnss3",
	"xdg-utils",
	"libsecret-1-0",
	"libappindicator3-1",
	"libgbm1",
	"fonts-noto-cjk",
	"libxss1",
	"wget",
}

// EnsureLLBotDeps 实现该函数对应的业务逻辑。
func EnsureLLBotDeps() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return nil
	}

	osID, versionID, err := readOSRelease()
	if err != nil {
		return err
	}
	if osID != "debian" && osID != "ubuntu" {
		return fmt.Errorf("unsupported linux distribution for LLBot dependencies: %s %s", osID, versionID)
	}

	missing := findMissingLLBotDeps()
	if len(missing) == 0 {
		log.Printf("llbot: dependencies already installed")
		return nil
	}

	log.Printf("llbot: missing packages detected: %s", strings.Join(missing, ", "))
	steps := [][]string{
		{"bash", "-lc", "apt-get update -y"},
		{"bash", "-lc", "apt-get install -y " + strings.Join(missing, " ")},
	}
	for _, step := range steps {
		cmd := exec.Command(step[0], step[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("command failed: %s | %w | %s", strings.Join(step, " "), err, strings.TrimSpace(string(out)))
		}
	}

	if _, err := exec.LookPath("xvfb-run"); err != nil {
		return fmt.Errorf("LLBot dependencies install completed but xvfb-run not found: %w", err)
	}
	log.Printf("llbot: dependency install completed")
	return nil
}

// findMissingLLBotDeps 查找并返回匹配目标。
func findMissingLLBotDeps() []string {
	missing := make([]string, 0, len(llbotAPTDependencies))
	for _, pkg := range llbotAPTDependencies {
		cmd := exec.Command("dpkg-query", "-W", "-f=${Status}", pkg)
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "install ok installed") {
			missing = append(missing, pkg)
		}
	}
	return missing
}
