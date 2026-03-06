package bootstrap

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// EnsureDotnet9 实现该函数对应的业务逻辑。
func EnsureDotnet9() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if hasDotnet9() {
		log.Printf("dotnet: detected .NET 9 runtime")
		return nil
	}
	log.Printf("dotnet: .NET 9 not found, installing from Microsoft packages")

	osID, versionID, err := readOSRelease()
	if err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		return errors.New("dotnet install requires root privileges")
	}

	switch osID {
	case "ubuntu":
		if err := installDotnetFromMicrosoftRepo("ubuntu", versionID); err != nil {
			return err
		}
	case "debian":
		major := strings.SplitN(versionID, ".", 2)[0]
		if major == "" {
			major = versionID
		}
		if err := installDotnetFromMicrosoftRepo("debian", major); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported linux distribution for auto .NET install: %s %s", osID, versionID)
	}

	if !hasDotnet9() {
		return errors.New(".NET 9 install completed but runtime not found")
	}
	log.Printf("dotnet: .NET 9 install completed")
	return nil
}

// hasDotnet9 判断并返回条件结果。
func hasDotnet9() bool {
	paths := []string{"dotnet", "/usr/local/dotnet/dotnet"}
	for _, bin := range paths {
		cmd := exec.Command(bin, "--list-runtimes")
		out, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}
		if strings.Contains(string(out), "Microsoft.NETCore.App 9.") {
			return true
		}
	}
	return false
}

// installDotnetFromMicrosoftRepo 实现该函数对应的业务逻辑。
func installDotnetFromMicrosoftRepo(distro string, version string) error {
	debURL := fmt.Sprintf("https://packages.microsoft.com/config/%s/%s/packages-microsoft-prod.deb", distro, version)
	debPath := "/tmp/packages-microsoft-prod.deb"

	steps := [][]string{
		{"bash", "-lc", fmt.Sprintf("curl -fsSL -o %s %s", debPath, debURL)},
		{"bash", "-lc", fmt.Sprintf("dpkg -i %s", debPath)},
		{"bash", "-lc", "apt-get update -y"},
		{"bash", "-lc", "apt-get install -y dotnet-sdk-9.0"},
	}
	for _, step := range steps {
		cmd := exec.Command(step[0], step[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("command failed: %s | %w | %s", strings.Join(step, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// readOSRelease 读取并返回相关数据。
func readOSRelease() (string, string, error) {
	raw, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", "", err
	}
	var osID string
	var versionID string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			osID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}
	if osID == "" || versionID == "" {
		return "", "", errors.New("cannot parse /etc/os-release")
	}
	return osID, versionID, nil
}
