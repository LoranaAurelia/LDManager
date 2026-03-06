package filemgr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathBlocksSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	linkPath := filepath.Join(root, "escape")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	if _, _, err := ResolvePath(root, "escape"); err == nil {
		t.Fatalf("expected symlink escape to be blocked")
	}
}
