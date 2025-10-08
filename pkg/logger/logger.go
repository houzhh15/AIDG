package logger

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Config 定义日志初始化配置
// Level 支持 debug/info/warn/error，Environment 支持 prod/dev 等
// WithSource 控制是否记录源码位置
// Default 对于未提供 level/环境时采用 info 与文本格式
type Config struct {
	Level       string
	Environment string
	WithSource  bool
}

var (
	global *slog.Logger
	once   sync.Once
)

func levelFromString(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, errors.New("invalid log level: " + level)
	}
}

// New 根据配置创建新的 slog.Logger，不设置全局实例
func New(cfg Config) (*slog.Logger, error) {
	lvl, err := levelFromString(cfg.Level)
	if err != nil {
		return nil, err
	}

	handlerOpts := &slog.HandlerOptions{Level: lvl, AddSource: cfg.WithSource}
	var handler slog.Handler
	if strings.ToLower(cfg.Environment) == "prod" {
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	}

	return slog.New(handler), nil
}

// Init 初始化全局日志实例，重复调用将返回首次创建的 logger
func Init(cfg Config) (*slog.Logger, error) {
	var initErr error
	once.Do(func() {
		global, initErr = New(cfg)
	})
	return global, initErr
}

// L 返回已初始化的全局 logger，未初始化时 panic
func L() *slog.Logger {
	if global == nil {
		panic("logger.Init must be called before logger.L")
	}
	return global
}
