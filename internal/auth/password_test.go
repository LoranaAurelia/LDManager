package auth

import (
	"path/filepath"
	"testing"
)

func TestPasswordStoreSetAndVerify(t *testing.T) {
	store := NewPasswordStore(filepath.Join(t.TempDir(), "password.hash"))
	if err := store.Set("12345678"); err != nil {
		t.Fatalf("set password failed: %v", err)
	}

	ok, err := store.Verify("12345678")
	if err != nil {
		t.Fatalf("verify should not fail: %v", err)
	}
	if !ok {
		t.Fatalf("verify should return true for correct password")
	}

	bad, err := store.Verify("wrong-password")
	if err != nil {
		t.Fatalf("verify wrong password should not fail: %v", err)
	}
	if bad {
		t.Fatalf("verify should return false for wrong password")
	}
}

func TestPasswordStoreRejectsShortPassword(t *testing.T) {
	store := NewPasswordStore(filepath.Join(t.TempDir(), "password.hash"))
	if err := store.Set("short"); err == nil {
		t.Fatalf("expected short password to be rejected")
	}
}

func TestParseArgon2Params(t *testing.T) {
	memory, timeCost, parallel, err := parseArgon2Params("m=65536,t=3,p=2")
	if err != nil {
		t.Fatalf("parseArgon2Params failed: %v", err)
	}
	if memory != 65536 || timeCost != 3 || parallel != 2 {
		t.Fatalf("unexpected params: m=%d t=%d p=%d", memory, timeCost, parallel)
	}

	if _, _, _, err := parseArgon2Params("m=65536,x=3,p=2"); err == nil {
		t.Fatalf("expected parse error for invalid key")
	}
}
