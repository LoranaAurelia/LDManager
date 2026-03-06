package version

import "strings"

// 这些变量由构建脚本通过 -ldflags 注入。
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	Channel   = "dev"
	Dirty     = "false"
)

func Display() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "dev"
	}
	return v
}
