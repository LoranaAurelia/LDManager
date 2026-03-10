package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractFirstHTTPURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "direct", raw: "https://example.com/install.sh", want: "https://example.com/install.sh"},
		{name: "command", raw: "curl -o napcat.sh https://raw.githubusercontent.com/NapNeko/napcat-linux-installer/refs/heads/main/install.sh && sudo bash napcat.sh", want: "https://raw.githubusercontent.com/NapNeko/napcat-linux-installer/refs/heads/main/install.sh"},
		{name: "missing", raw: "bash install.sh", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractFirstHTTPURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.want {
				t.Fatalf("want %q got %q", tt.want, got)
			}
		})
	}
}

func TestUpdateReadNapcatWebUIPort(t *testing.T) {
	tmp := t.TempDir()
	installDir := filepath.Join(tmp, "napcat-instance")
	cfgDir := filepath.Join(installDir, "napcat", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, "webui.json")
	if err := os.WriteFile(cfgPath, []byte(`{"host":"::","port":6099,"token":"abc"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := UpdateNapcatWebUIPort(installDir, 6100); err != nil {
		t.Fatalf("update: %v", err)
	}
	port, err := ReadNapcatWebUIPort(installDir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if port != 6100 {
		t.Fatalf("want 6100 got %d", port)
	}
}

func TestEnsureNapcatRunner(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, napcatLauncherSOName), []byte("x"), 0o644); err != nil {
		t.Fatalf("write so: %v", err)
	}
	runner, err := EnsureNapcatRunner(tmp, nil)
	if err != nil {
		t.Fatalf("ensure runner: %v", err)
	}
	raw, err := os.ReadFile(runner)
	if err != nil {
		t.Fatalf("read runner: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, "LD_PRELOAD") || !strings.Contains(s, napcatQQBinary) {
		t.Fatalf("runner missing expected content: %s", s)
	}
}
