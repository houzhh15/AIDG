package documents

import (
	"os"
)

// writeFileAtomic 原子性写入文件
func writeFileAtomic(path string, data []byte) error {
	tempPath := path + ".tmp"

	// 写入临时文件
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// 原子替换
	return os.Rename(tempPath, path)
}

// deleteFileIfExists 删除文件（如果存在）
func deleteFileIfExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // 文件不存在，认为成功
	}
	return os.Remove(path)
}
