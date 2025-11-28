package logger

import (
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *zap.Logger

// InitLogger initializes the zap logger with console and file output
func InitLogger(dataDir string) error {
	// Ensure logs directory exists
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	logFile := filepath.Join(logDir, "smarticky.log")

	// Configure lumberjack for log rotation
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    5,     // MB
		MaxBackups: 7,     // Keep 7 old files
		MaxAge:     30,    // Days
		Compress:   true,  // Compress rotated files
		LocalTime:  true,
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // Colored level for console
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create console encoder (with colors)
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Create file encoder config (no colors for file)
	fileEncoderConfig := encoderConfig
	fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // No colors for file

	// Create file encoder
	fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)

	// Create multi-writer: console + file
	consoleWriter := zapcore.Lock(os.Stdout)
	fileWriter := zapcore.AddSync(lumberjackLogger)

	// Set log level
	level := zapcore.InfoLevel
	if os.Getenv("DEBUG") != "" {
		level = zapcore.DebugLevel
	}

	// Create core with multiple outputs
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, consoleWriter, level),
		zapcore.NewCore(fileEncoder, fileWriter, level),
	)

	// Create logger
	Logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	// Replace global logger
	zap.ReplaceGlobals(Logger)

	return nil
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		Logger.Sync()
	}
}

// GetLogWriter returns an io.Writer that writes to the logger at Info level
func GetLogWriter() io.Writer {
	return &logWriter{logger: Logger}
}

// logWriter implements io.Writer interface for logger
type logWriter struct {
	logger *zap.Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.Info(string(p))
	return len(p), nil
}
