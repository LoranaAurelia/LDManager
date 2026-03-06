package services

import "testing"

func TestValidateRegistryName(t *testing.T) {
	if err := ValidateRegistryName("test_01-abc"); err != nil {
		t.Fatalf("expected valid registry name: %v", err)
	}
	if err := ValidateRegistryName("测试"); err == nil {
		t.Fatalf("expected invalid registry name to be rejected")
	}
	if err := ValidateRegistryName(" "); err == nil {
		t.Fatalf("expected empty registry name to be rejected")
	}
}

func TestSanitizeName(t *testing.T) {
	got := SanitizeName("  a b/c?d  ")
	if got != "a-b-c-d" {
		t.Fatalf("unexpected sanitize result: %s", got)
	}
}
