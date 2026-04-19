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
	logger             *zap.Logger
	sugar              *zap.SugaredLogger
	once               sync.Once
	logLevel           zapcore.Level = zapcore.InfoLevel
	fileLogLevel       zapcore.Level = zapcore.DebugLevel
	consoleAtomicLevel               = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	fileAtomicLevel                  = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logFile            *os.File
)

// InitLogger 初始化全局日志配置
func InitLogger(logDir string) error {
	return initLoggerWithFilename(logDir, "app")
}

func initLoggerWithFilename(logDir string, filePrefix string) error {
	var initErr error
	once.Do(func() {
		// 确保日志目录存在
		if err := os.MkdirAll(logDir, 0755); err != nil {
			initErr = err
			return
		}

		if strings.TrimSpace(filePrefix) == "" {
			filePrefix = "app"
		}
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		logFilePath := filepath.Join(logDir, filePrefix+"_"+timestamp+".log")

		// 配置日志文件
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			initErr = err
			return
		}
		logFile = file

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

		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(file),
			fileAtomicLevel,
		)
		consoleCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			nopSyncer{WriteSyncer: zapcore.AddSync(os.Stdout)},
			consoleAtomicLevel,
		)
		core := zapcore.NewTee(fileCore, consoleCore)

		// 创建日志记录器
		logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
		sugar = logger.Sugar()
	})

	return initErr
}

// InitLoggerWithPackage 使用被测应用包名作为日志前缀：
// 包名_年-月-日_时-分-秒.log
func InitLoggerWithPackage(logDir string, packageName string) error {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return InitLogger(logDir)
	}
	safeName := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(packageName)
	return initLoggerWithFilename(logDir, safeName)
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

// SetLevel 设置控制台日志级别。
func SetLevel(level string) error {
	zapLevel, err := parseLevel(level)
	if err != nil {
		return err
	}
	logLevel = zapLevel
	consoleAtomicLevel.SetLevel(zapLevel)
	return nil
}

// SetFileLevel 设置文件日志级别，支持在 logger 初始化后动态生效。
func SetFileLevel(level string) error {
	zapLevel, err := parseLevel(level)
	if err != nil {
		return err
	}
	fileLogLevel = zapLevel
	fileAtomicLevel.SetLevel(zapLevel)
	return nil
}

func parseLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}

type nopSyncer struct {
	zapcore.WriteSyncer
}

func (n nopSyncer) Sync() error {
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
