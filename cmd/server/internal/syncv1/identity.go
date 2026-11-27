package syncv1

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const identityFileName = "server-id"

// LoadOrCreateServerID returns the stable identity used to distinguish one AIDG
// installation from another. The identity is stored outside project content so
// moving or renaming projects cannot change server identity.
func LoadOrCreateServerID(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("sync identity data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", fmt.Errorf("create sync identity directory: %w", err)
	}
	path := filepath.Join(dataDir, identityFileName)
	if data, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(data))
		if _, parseErr := uuid.Parse(id); parseErr != nil {
			return "", fmt.Errorf("invalid persisted sync server identity: %w", parseErr)
		}
		return id, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read sync server identity: %w", err)
	}

	id := uuid.NewString()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return LoadOrCreateServerID(dataDir)
		}
		return "", fmt.Errorf("create sync server identity: %w", err)
	}
	if _, err := file.WriteString(id + "\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write sync server identity: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("sync server identity: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close sync server identity: %w", err)
	}
	return id, nil
}
