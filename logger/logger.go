package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger   *zap.Logger
	sugar    *zap.SugaredLogger
	once     sync.Once
	logLevel zapcore.Level = zapcore.InfoLevel
)

// InitLogger 初始化全局日志配置
func InitLogger(logDir string) error {
	var initErr error
	once.Do(func() {
		// 确保日志目录存在
		if err := os.MkdirAll(logDir, 0755); err != nil {
			initErr = err
			return
		}

		// 获取当前日期用于日志文件名
		currentDate := time.Now().Format("2006-01-02")
		logFile := filepath.Join(logDir, "app_"+currentDate+".log")

		// 配置日志文件
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			initErr = err
			return
		}

		// 配置编码器
		encoderConfig := zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		// 创建核心配置
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(file), zapcore.AddSync(os.Stdout)),
			logLevel,
		)

		// 创建日志记录器
		logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
		sugar = logger.Sugar()
	})

	return initErr
}

// GetLogger 获取标准日志记录器
func GetLogger() *zap.Logger {
	if logger == nil {
		InitLogger("log")
	}
	return logger
}

// GetSugar 获取Sugared日志记录器
func GetSugar() *zap.SugaredLogger {
	if sugar == nil {
		InitLogger("log")
	}
	return sugar
}

// Debug 记录调试日志
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Info 记录信息日志
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Warn 记录警告日志
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Error 记录错误日志
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Fatal 记录致命错误日志
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// Debugf 记录格式化的调试日志
func Debugf(template string, args ...interface{}) {
	GetSugar().Debugf(template, args...)
}

// Infof 记录格式化的信息日志
func Infof(template string, args ...interface{}) {
	GetSugar().Infof(template, args...)
}

// Warnf 记录格式化的警告日志
func Warnf(template string, args ...interface{}) {
	GetSugar().Warnf(template, args...)
}

// Errorf 记录格式化的错误日志
func Errorf(template string, args ...interface{}) {
	GetSugar().Errorf(template, args...)
}

// Fatalf 记录格式化的致命错误日志
func Fatalf(template string, args ...interface{}) {
	GetSugar().Fatalf(template, args...)
}

// Sync 同步日志缓冲区
func Sync() error {
	if logger != nil {
		return logger.Sync()
	}
	return nil
}

// SetLevel 设置日志级别
func SetLevel(level string) error {
	var zapLevel zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	case "fatal":
		zapLevel = zapcore.FatalLevel
	default:
		return fmt.Errorf("invalid log level: %s", level)
	}
	logLevel = zapLevel
	return nil
}

// 以下是常用的字段构造辅助函数
func String(key string, val string) zap.Field {
	return zap.String(key, val)
}

func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func Float64(key string, val float64) zap.Field {
	return zap.Float64(key, val)
}

func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

func Any(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}

func ErrorField(err error) zap.Field {
	return zap.Error(err)
}
