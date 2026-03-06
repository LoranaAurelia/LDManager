package bootstrap

import (
	"os"
	"path/filepath"

	"web-sealdice/internal/config"
)

// EnsureLayout 实现该函数对应的业务逻辑。
func EnsureLayout(cfg config.Config) error {
	dirs := []string{
		cfg.DataDir,
		filepath.Join(cfg.DataDir, "Sealdice"),
		filepath.Join(cfg.DataDir, "Napcat"),
		filepath.Join(cfg.DataDir, "Lagrange"),
		filepath.Join(cfg.DataDir, "LLBot"),
		filepath.Join(cfg.DataDir, "services"),
		filepath.Join(cfg.DataDir, "logs"),
		filepath.Join(cfg.DataDir, "logs", "deploy"),
		cfg.AuthDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
