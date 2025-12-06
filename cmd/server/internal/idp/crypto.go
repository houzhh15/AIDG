package idp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
)

const (
	// EncryptionKeyEnvVar 加密密钥环境变量名
	EncryptionKeyEnvVar = "IDP_ENCRYPTION_KEY"

	// 加密前缀，用于识别已加密的字符串
	encryptedPrefix = "enc:"
)

// Crypto 提供敏感字段的加解密功能
type Crypto struct {
	key []byte
}

// NewCrypto 创建加密器实例
// 如果环境变量未设置，返回 nil（不加密模式）
func NewCrypto() *Crypto {
	key := os.Getenv(EncryptionKeyEnvVar)
	if key == "" {
		return nil
	}
	// AES-256 需要 32 字节密钥
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		// 补齐到 32 字节
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}
	return &Crypto{key: keyBytes}
}

// Encrypt 使用 AES-256-GCM 加密字符串
// 返回 base64 编码的密文，带 "enc:" 前缀
func (c *Crypto) Encrypt(plaintext string) (string, error) {
	if c == nil || plaintext == "" {
		return plaintext, nil
	}

	// 如果已经是加密的，直接返回
	if len(plaintext) > len(encryptedPrefix) && plaintext[:len(encryptedPrefix)] == encryptedPrefix {
		return plaintext, nil
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return encryptedPrefix + encoded, nil
}

// Decrypt 解密 AES-256-GCM 加密的字符串
// 输入必须是带 "enc:" 前缀的 base64 编码密文
func (c *Crypto) Decrypt(ciphertext string) (string, error) {
	if c == nil || ciphertext == "" {
		return ciphertext, nil
	}

	// 如果不是加密的，直接返回
	if len(ciphertext) <= len(encryptedPrefix) || ciphertext[:len(encryptedPrefix)] != encryptedPrefix {
		return ciphertext, nil
	}

	encoded := ciphertext[len(encryptedPrefix):]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrDecryptionFailed
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// IsEncrypted 检查字符串是否已加密
func IsEncrypted(s string) bool {
	return len(s) > len(encryptedPrefix) && s[:len(encryptedPrefix)] == encryptedPrefix
}

// MaskSecret 将敏感信息脱敏（显示为星号）
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "********"
}
