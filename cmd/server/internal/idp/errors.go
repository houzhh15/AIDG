package idp

import "errors"

// 错误定义
var (
	// ErrTypeMismatch 类型不匹配
	ErrTypeMismatch = errors.New("identity provider type mismatch")

	// ErrNotFound 身份源不存在
	ErrNotFound = errors.New("identity provider not found")

	// ErrAlreadyExists 身份源已存在
	ErrAlreadyExists = errors.New("identity provider already exists")

	// ErrDisabled 身份源已禁用
	ErrDisabled = errors.New("identity provider is disabled")

	// ErrConnectionFailed 连接失败
	ErrConnectionFailed = errors.New("connection to identity provider failed")

	// ErrAuthFailed 认证失败
	ErrAuthFailed = errors.New("authentication failed")

	// ErrStateMismatch OIDC state 验证失败
	ErrStateMismatch = errors.New("OIDC state mismatch")

	// ErrTokenInvalid Token 无效
	ErrTokenInvalid = errors.New("token is invalid")

	// ErrSyncInProgress 同步任务正在进行中
	ErrSyncInProgress = errors.New("sync is already in progress")

	// ErrConfigInvalid 配置无效
	ErrConfigInvalid = errors.New("invalid configuration")

	// ErrUserDisabled 用户已禁用
	ErrUserDisabled = errors.New("user is disabled")

	// ErrEncryptionKeyMissing 加密密钥缺失
	ErrEncryptionKeyMissing = errors.New("encryption key not configured")

	// ErrDecryptionFailed 解密失败
	ErrDecryptionFailed = errors.New("decryption failed")
)
