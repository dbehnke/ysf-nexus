package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps zap.Logger with additional functionality
type Logger struct {
	*zap.Logger
	config Config
}

// Config holds logger configuration
type Config struct {
	Level       string
	Format      string
	File        string
	MaxSize     int
	MaxBackups  int
	MaxAge      int
	Development bool
}

// New creates a new logger with the given configuration
func New(config Config) (*Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Create encoder
	var encoder zapcore.Encoder
	encoderConfig := getEncoderConfig(config.Development)

	if config.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create writer
	writer := getWriter(config)

	// Create core
	core := zapcore.NewCore(encoder, writer, level)

	// Create logger
	var logger *zap.Logger
	if config.Development {
		logger = zap.New(core, zap.Development(), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		logger = zap.New(core, zap.AddCaller())
	}

	return &Logger{
		Logger: logger,
		config: config,
	}, nil
}

// getEncoderConfig returns encoder configuration
func getEncoderConfig(development bool) zapcore.EncoderConfig {
	if development {
		return zap.NewDevelopmentEncoderConfig()
	}

	config := zap.NewProductionEncoderConfig()
	config.TimeKey = "timestamp"
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncodeLevel = zapcore.CapitalLevelEncoder
	return config
}

// getWriter creates the appropriate writer based on configuration
func getWriter(config Config) zapcore.WriteSyncer {
	if config.File == "" {
		// Console only
		return zapcore.AddSync(os.Stdout)
	}

	// Ensure directory exists
	dir := filepath.Dir(config.File)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// Fallback to console if directory creation fails
		return zapcore.AddSync(os.Stdout)
	}

	// File with rotation
	fileWriter := &lumberjack.Logger{
		Filename:   config.File,
		MaxSize:    config.MaxSize, // MB
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge, // days
		Compress:   true,
	}

	// Write to both console and file
	return zapcore.AddSync(io.MultiWriter(os.Stdout, fileWriter))
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() {
	_ = l.Logger.Sync()
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	var zapFields []zap.Field
	for key, value := range fields {
		zapFields = append(zapFields, zap.Any(key, value))
	}

	return &Logger{
		Logger: l.Logger.With(zapFields...),
		config: l.config,
	}
}

// WithComponent returns a logger with a component field
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.String("component", component)),
		config: l.config,
	}
}

// WithError returns a logger with an error field
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger: l.Logger.With(zap.Error(err)),
		config: l.config,
	}
}

// Default creates a default logger for development
func Default() *Logger {
	config := Config{
		Level:       "info",
		Format:      "console",
		Development: true,
	}

	logger, err := New(config)
	if err != nil {
		// Fallback to basic zap logger
		zapLogger, _ := zap.NewDevelopment()
		return &Logger{Logger: zapLogger, config: config}
	}

	return logger
}

// FromConfig creates a logger from a configuration struct
func FromConfig(config Config) (*Logger, error) {
	return New(config)
}

// Convenience methods for common field types
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

func Int64(key string, value int64) zap.Field {
	return zap.Int64(key, value)
}

func Uint64(key string, value uint64) zap.Field {
	return zap.Uint64(key, value)
}

func Uint32(key string, value uint32) zap.Field {
	return zap.Uint32(key, value)
}

func Duration(key string, value time.Duration) zap.Field {
	return zap.Duration(key, value)
}

func Error(err error) zap.Field {
	return zap.Error(err)
}
