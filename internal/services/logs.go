package services

import (
	"errors"
	"os"
	"strings"
)

// TailFile 实现该函数对应的业务逻辑。
func TailFile(path string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	text := string(raw)
	parts := strings.Split(text, "\n")
	if len(parts) <= lines {
		return text, nil
	}
	return strings.Join(parts[len(parts)-lines:], "\n"), nil
}
