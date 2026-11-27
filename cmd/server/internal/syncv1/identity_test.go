package syncv1

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateServerIDIsStable(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateServerID(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateServerID(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("server identity is not stable: first=%q second=%q", first, second)
	}
	info, err := os.Stat(filepath.Join(dir, identityFileName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("server identity permissions are too broad: %o", info.Mode().Perm())
	}
}

func TestLoadOrCreateServerIDRejectsInvalidPersistedValue(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, identityFileName), []byte("not-a-uuid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateServerID(dir); err == nil {
		t.Fatal("expected invalid persisted identity to fail closed")
	}
}
