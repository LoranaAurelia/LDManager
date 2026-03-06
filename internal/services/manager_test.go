package services

import "testing"

func TestNormalizeSealdiceArgs(t *testing.T) {
	in := []string{"--address:0.0.0.0:3211", "--foo=bar"}
	out := normalizeSealdiceArgs(in)

	if out[0] != "--address=0.0.0.0:3211" {
		t.Fatalf("unexpected normalized arg: %s", out[0])
	}
	if out[1] != "--foo=bar" {
		t.Fatalf("unexpected second arg: %s", out[1])
	}

	// 验证 normalizeSealdiceArgs 不会原地修改输入切片，避免调用方状态被污染。
	if in[0] != "--address:0.0.0.0:3211" {
		t.Fatalf("input args should remain unchanged")
	}
}

func TestEqualArgs(t *testing.T) {
	if !equalArgs([]string{"a", "b"}, []string{"a", "b"}) {
		t.Fatalf("expected args to be equal")
	}
	if equalArgs([]string{"a", "b"}, []string{"a", "c"}) {
		t.Fatalf("expected args to be different")
	}
}
