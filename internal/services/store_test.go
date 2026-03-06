package services

import (
	"path/filepath"
	"testing"
)

func TestStoreCreateGetUpdateDelete(t *testing.T) {
	dataDir := t.TempDir()
	store := NewStore(dataDir)

	created, err := store.Create(CreateServiceRequest{
		ID:         "test_service",
		Name:       "test_service",
		Type:       "Sealdice",
		ExecPath:   "/tmp/sealdice-core",
		InstallDir: "/tmp",
	}, filepath.Join(dataDir, "logs"))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "test_service" {
		t.Fatalf("unexpected service id: %s", got.ID)
	}

	got.DisplayName = "测试显示名"
	if err := store.Update(got); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got2, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if got2.DisplayName != "测试显示名" {
		t.Fatalf("display name was not updated")
	}

	if err := store.Delete(created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := store.Get(created.ID); err == nil {
		t.Fatalf("expected service file to be deleted")
	}
}
