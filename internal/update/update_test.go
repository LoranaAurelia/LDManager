package update

import (
	"encoding/json"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0.0.179-Internal-Dev+sha.69aae21", "0.0.179"},
		{"v1.2.3", "1.2.3"},
		{"V2.10.4-beta", "2.10.4"},
		{"invalid", ""},
	}
	for _, c := range cases {
		if got := normalizeVersion(c.in); got != c.want {
			t.Fatalf("normalizeVersion(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestCompareNormalizedVersion(t *testing.T) {
	if compareNormalizedVersion("0.0.180", "0.0.179") <= 0 {
		t.Fatal("expected remote newer")
	}
	if compareNormalizedVersion("1.2.0", "1.2") != 0 {
		t.Fatal("expected 1.2.0 == 1.2")
	}
	if compareNormalizedVersion("0.9.9", "1.0.0") >= 0 {
		t.Fatal("expected 0.9.9 older")
	}
}

func TestLoadSourcesConfig(t *testing.T) {
	cfg, err := LoadSourcesConfig()
	if err != nil {
		t.Fatalf("LoadSourcesConfig failed: %v", err)
	}
	if len(cfg.ManifestSources) == 0 {
		t.Fatal("manifest sources should not be empty")
	}
	if _, err := cfg.DefaultManifest(); err != nil {
		t.Fatalf("default manifest invalid: %v", err)
	}
}

func TestNestedLatestManifestShape(t *testing.T) {
	raw := []byte(`{
		"latest": {
			"version": "0.0.180",
			"files": {
				"amd64": "http://manifest.xuetao.host/LoranasDiceManager/files/internal-dev/%200.0.180/ldm-linux-amd64"
			}
		}
	}`)
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	latestMap, ok := generic["latest"].(map[string]any)
	if !ok {
		t.Fatal("latest should be object")
	}
	version := pickString(latestMap, "version")
	if version != "0.0.180" {
		t.Fatalf("version mismatch: %s", version)
	}
	filesMap, ok := latestMap["files"].(map[string]any)
	if !ok {
		t.Fatal("latest.files should be object")
	}
	dl := pickString(filesMap, "amd64")
	if dl == "" {
		t.Fatal("amd64 download url should not be empty")
	}
}
