package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	once   sync.Once
	global *zap.Logger
)

// Config holds logger configuration.
type Config struct {
	Level      string // debug, info, warn, error
	Encoding   string // json, console
	OutputPath string // stdout or file path
}

// Init initializes the global logger with the given config.
func Init(cfg Config) {
	once.Do(func() {
		global = mustBuild(cfg)
	})
}

func mustBuild(cfg Config) *zap.Logger {
	level := zap.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zap.InfoLevel
	}

	encoding := cfg.Encoding
	if encoding == "" {
		encoding = "json"
	}

	outputPath := cfg.OutputPath
	if outputPath == "" {
		outputPath = "stdout"
	}

	encCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	zapCfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Sampling:         &zap.SamplingConfig{Initial: 100, Thereafter: 100},
		Encoding:         encoding,
		EncoderConfig:    encCfg,
		OutputPaths:      []string{outputPath},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := zapCfg.Build(zap.AddCallerSkip(0))
	if err != nil {
		// Fallback to a basic logger
		logger, _ = zap.NewProduction()
	}
	return logger
}

// Default returns a default production logger if Init has not been called.
func Default() *zap.Logger {
	if global == nil {
		env := os.Getenv("LOG_LEVEL")
		if env == "" {
			env = "info"
		}
		global = mustBuild(Config{Level: env, Encoding: "json"})
	}
	return global
}

// L is a convenience shortcut for Default().
func L() *zap.Logger {
	return Default()
}

// S returns a sugared logger.
func S() *zap.SugaredLogger {
	return Default().Sugar()
}

// With returns a child logger with the given fields.
func With(fields ...zap.Field) *zap.Logger {
	return Default().With(fields...)
}

// Named returns a named logger.
func Named(name string) *zap.Logger {
	return Default().Named(name)
}
