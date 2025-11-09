package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// CalculateMD5 计算文件的MD5哈希值
// 返回十六进制字符串和可能的错误
func CalculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate MD5: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CopyDirectory 递归复制源目录到目标目录
// 保持目录结构和文件权限
func CopyDirectory(src, dst string) error {
	// 检查源目录是否存在
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source directory does not exist: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	// 创建目标目录
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 遍历源目录
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk error: %w", err)
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// 目标路径
		dstPath := filepath.Join(dst, relPath)

		// 如果是目录，创建目录
		if info.IsDir() {
			if err := os.MkdirAll(dstPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			return nil
		}

		// 如果是文件，复制文件
		return copyFile(path, dstPath, info.Mode())
	})
}

// copyFile 复制单个文件
func copyFile(src, dst string, perm os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// ValidateTagName 验证tag名称是否合法
// 允许字母、数字、下划线、横线，长度1-50字符
func ValidateTagName(tagName string) bool {
	if tagName == "" {
		return false
	}

	// 正则表达式：^[a-zA-Z0-9_-]{1,50}$
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_-]{1,50}$`, tagName)
	if err != nil {
		return false
	}

	return matched
}
