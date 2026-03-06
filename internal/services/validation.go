package services

import (
	"errors"
	"regexp"
	"strings"
)

var registryNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ValidateRegistryName 实现该函数对应的业务逻辑。
func ValidateRegistryName(name string) error {
	value := strings.TrimSpace(name)
	if value == "" {
		return errors.New("registry_name is required")
	}
	if !registryNamePattern.MatchString(value) {
		return errors.New("registry_name can only contain letters, numbers, underscore and hyphen")
	}
	return nil
}

// SanitizeName 清洗输入并保证安全性。
func SanitizeName(name string) string {
	value := strings.TrimSpace(name)
	if value == "" {
		return ""
	}
	out := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			out = append(out, r)
		} else {
			out = append(out, '-')
		}
	}
	return strings.Trim(string(out), "-")
}
