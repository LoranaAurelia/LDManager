package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// PasswordStore 负责密码哈希文件的读写与校验。
type PasswordStore struct {
	path string
}

// NewPasswordStore 创建并初始化对应对象。
func NewPasswordStore(path string) *PasswordStore {
	return &PasswordStore{path: path}
}

// Exists 实现该函数对应的业务逻辑。
func (s *PasswordStore) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Set 实现该函数对应的业务逻辑。
func (s *PasswordStore) Set(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	hash, err := hashPassword(password)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, []byte(hash), 0o600)
}

// Verify 实现该函数对应的业务逻辑。
func (s *PasswordStore) Verify(password string) (bool, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return false, err
	}

	ok, err := verifyPassword(strings.TrimSpace(string(raw)), password)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// hashPassword 使用 Argon2id 对明文密码进行单向哈希，返回可持久化存储的编码字符串。
// 输出格式为：argon2id$v=19$m=...,t=...,p=...$salt$hash。
func hashPassword(password string) (string, error) {
	var (
		timeCost    uint32 = 3
		memoryCost  uint32 = 64 * 1024
		parallelism uint8  = 2
		keyLen      uint32 = 32
	)

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, parallelism, keyLen)
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memoryCost, timeCost, parallelism, saltB64, hashB64), nil
}

// verifyPassword 实现该函数对应的业务逻辑。
func verifyPassword(encoded string, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 {
		return false, errors.New("invalid password hash format")
	}
	if parts[0] != "argon2id" || parts[1] != "v=19" {
		return false, errors.New("unsupported hash version")
	}

	memory, timeCost, parallel, err := parseArgon2Params(parts[2])
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		uint32(timeCost),
		uint32(memory),
		uint8(parallel),
		uint32(len(expectedHash)),
	)

	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
}

// parseArgon2Params 解析输入并转换为结构化结果。
func parseArgon2Params(raw string) (memory uint32, timeCost uint32, parallelism uint8, err error) {
	chunks := strings.Split(raw, ",")
	if len(chunks) != 3 {
		return 0, 0, 0, errors.New("invalid hash params")
	}

	if !strings.HasPrefix(chunks[0], "m=") || !strings.HasPrefix(chunks[1], "t=") || !strings.HasPrefix(chunks[2], "p=") {
		return 0, 0, 0, errors.New("invalid hash params")
	}

	memoryParsed, err := strconv.ParseUint(strings.TrimPrefix(chunks[0], "m="), 10, 32)
	if err != nil {
		return 0, 0, 0, err
	}
	timeParsed, err := strconv.ParseUint(strings.TrimPrefix(chunks[1], "t="), 10, 32)
	if err != nil {
		return 0, 0, 0, err
	}
	parallelParsed, err := strconv.ParseUint(strings.TrimPrefix(chunks[2], "p="), 10, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	return uint32(memoryParsed), uint32(timeParsed), uint8(parallelParsed), nil
}
