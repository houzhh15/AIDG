package sync

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

// normalizePath canonicalizes path separators to forward slash for transport & comparison
func NormalizePath(p string) string {
	if p == "" {
		return p
	}
	// filepath.Clean will use OS separators. We then replace backslashes to forward for canonical form.
	cleaned := filepath.Clean(p)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	return cleaned
}

// IsAllowedSyncPath checks if a path is in the allowed list (exported for API handlers)
func IsAllowedSyncPath(p string, allowList []string) bool {
	clean := NormalizePath(p)
	for _, allow := range allowList {
		baseAllow := NormalizePath(allow)
		if strings.HasSuffix(baseAllow, "/") { // directory prefix
			if strings.HasPrefix(clean, strings.TrimSuffix(baseAllow, "/")) {
				return true
			}
		} else if clean == baseAllow {
			return true
		}
	}
	return false
}

// isAllowedSyncPath internal wrapper for backward compatibility
func isAllowedSyncPath(p string, allowList []string) bool {
	return IsAllowedSyncPath(p, allowList)
}

// HashContent computes SHA256 hash of content
func HashContent(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("sha256:%x", h[:])
}

// isIgnoredSyncFile determines if a file should be excluded from sync (e.g. large audio)
func isIgnoredSyncFile(p string) bool {
	lp := strings.ToLower(p)
	// 忽略 .wav 音频文件
	return strings.HasSuffix(lp, ".wav")
}
