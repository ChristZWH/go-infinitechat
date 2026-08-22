package common

import (
	"github.com/mattn/go-colorable"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

func init() {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		// 时间格式
		EncodeTime: zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"),
		// 彩色日志级别
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder, // 使用相对路径，而不是绝对路径
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(colorable.NewColorableStdout()), // 以秒的形式呈现
		zapcore.DebugLevel,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	Logger = logger.Sugar()
}

func Info(args ...any) {
	Logger.Info(args...)
}

func Infof(format string, args ...any) {
	Logger.Infof(format, args...)
}

func Error(args ...any) {
	Logger.Error(args...)
}

func Errorf(format string, args ...any) {
	Logger.Errorf(format, args...)
}

func Warn(args ...any) {
	Logger.Warn(args...)
}

func Warnf(format string, args ...any) {
	Logger.Warnf(format, args...)
}

func Debug(args ...any) {
	Logger.Debug(args...)
}

func Debugf(format string, args ...any) {
	Logger.Debugf(format, args...)
}

func Fatal(args ...any) {
	Logger.Fatal(args...)
}

func Fatalf(format string, args ...any) {
	Logger.Fatalf(format, args...)
}
