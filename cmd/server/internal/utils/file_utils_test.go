package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCalculateMD5 测试MD5计算函数
func TestCalculateMD5(t *testing.T) {
	// 创建临时测试文件
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	
	// 正常情况：计算已知内容的MD5
	content := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	hash, err := CalculateMD5(testFile)
	if err != nil {
		t.Errorf("CalculateMD5 failed: %v", err)
	}
	
	// "Hello, World!" 的MD5是 65a8e27d8879283831b664bd8b7f0ad4
	expected := "65a8e27d8879283831b664bd8b7f0ad4"
	if hash != expected {
		t.Errorf("Expected MD5 %s, got %s", expected, hash)
	}
	
	// 异常情况：文件不存在
	_, err = CalculateMD5("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

// TestCopyDirectory 测试目录复制函数
func TestCopyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")
	
	// 创建源目录结构
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	
	// 创建测试文件
	file1 := filepath.Join(srcDir, "file1.txt")
	file2 := filepath.Join(srcDir, "subdir", "file2.txt")
	
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}
	
	// 正常情况：复制目录
	if err := CopyDirectory(srcDir, dstDir); err != nil {
		t.Errorf("CopyDirectory failed: %v", err)
	}
	
	// 验证目标文件存在
	dstFile1 := filepath.Join(dstDir, "file1.txt")
	dstFile2 := filepath.Join(dstDir, "subdir", "file2.txt")
	
	if _, err := os.Stat(dstFile1); os.IsNotExist(err) {
		t.Error("Destination file1 does not exist")
	}
	if _, err := os.Stat(dstFile2); os.IsNotExist(err) {
		t.Error("Destination file2 does not exist")
	}
	
	// 验证文件内容
	content1, _ := os.ReadFile(dstFile1)
	if string(content1) != "content1" {
		t.Errorf("Expected content1, got %s", string(content1))
	}
	
	content2, _ := os.ReadFile(dstFile2)
	if string(content2) != "content2" {
		t.Errorf("Expected content2, got %s", string(content2))
	}
	
	// 异常情况：源目录不存在
	err := CopyDirectory("/nonexistent/dir", dstDir)
	if err == nil {
		t.Error("Expected error for nonexistent source directory, got nil")
	}
	
	// 异常情况：源不是目录
	err = CopyDirectory(file1, dstDir)
	if err == nil {
		t.Error("Expected error when source is not a directory, got nil")
	}
}

// TestValidateTagName 测试tag名称验证函数
func TestValidateTagName(t *testing.T) {
	tests := []struct {
		name     string
		tagName  string
		expected bool
	}{
		{"valid alphanumeric", "v1-0", true},
		{"valid with underscore", "tag_name", true},
		{"valid with dash", "my-tag", true},
		{"valid mixed", "Tag_123-v1", true},
		{"valid single char", "a", true},
		{"valid 50 chars", "12345678901234567890123456789012345678901234567890", true},
		{"invalid empty", "", false},
		{"invalid too long", "123456789012345678901234567890123456789012345678901", false},
		{"invalid special char", "tag@name", false},
		{"invalid space", "tag name", false},
		{"invalid dot", "v1.0", false},
		{"invalid slash", "tag/name", false},
		{"invalid backslash", "tag\\name", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateTagName(tt.tagName)
			if result != tt.expected {
				t.Errorf("ValidateTagName(%q) = %v, expected %v", tt.tagName, result, tt.expected)
			}
		})
	}
}
