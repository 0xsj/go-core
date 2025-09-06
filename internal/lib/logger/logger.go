// internal/lib/logger/logger.go
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[37m"
	ColorWhite  = "\033[97m"
	ColorGreen  = "\033[32m"
	ColorCyan   = "\033[36m"
)

type contextKey string

const (
	ContextKeyRequestID     contextKey = "request_id"
	ContextKeyTraceID       contextKey = "trace_id"
	ContextKeyCorrelationID contextKey = "correlation_id"
	ContextKeyUserID        contextKey = "user_id"
	ContextKeySessionID     contextKey = "session_id"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

type LogFormat int

const (
	FormatPretty LogFormat = iota
	FormatJSON
)

type LogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     LogLevel       `json:"level"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
	Error     error          `json:"error,omitempty"`
	Caller    string         `json:"caller,omitempty"`
}

type Field struct {
	Key   string
	Value any
}

func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}

func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

type LoggerConfig struct {
	Level      LogLevel
	Output     io.Writer
	Format     LogFormat
	ShowCaller bool
	ShowColor  bool
}

func DefaultConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:      LevelInfo,
		Output:     os.Stdout,
		Format:     FormatPretty,
		ShowCaller: true,
		ShowColor:  true,
	}
}

type Logger interface {
	// Core logging methods
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	// Contextual logging
	WithContext(ctx context.Context) Logger
	WithFields(fields ...Field) Logger
	WithError(err error) Logger

	// Correlation support
	WithCorrelationID(id string) Logger
	WithTraceID(id string) Logger
	WithRequestID(id string) Logger
	WithUserID(id string) Logger

	// Level control
	SetLevel(level LogLevel)
	GetLevel() LogLevel
	IsLevelEnabled(level LogLevel) bool

	// Output control
	SetOutput(w io.Writer)

	// Structured field support
	With(key string, value any) Logger
}

// logger is the concrete implementation of Logger
type logger struct {
	config *LoggerConfig
	fields map[string]any
	ctx    context.Context
	mu     sync.RWMutex
}

func NewLogger(config *LoggerConfig) Logger {
	if config == nil {
		config = DefaultConfig()
	}

	if config.Output == nil {
		config.Output = os.Stdout
	}

	return &logger{
		config: config,
		fields: make(map[string]any),
		ctx:    nil,
	}
}

func (l *logger) Debug(msg string, fields ...Field) {
	if !l.IsLevelEnabled(LevelDebug) {
		return
	}
	l.log(LevelDebug, msg, fields...)
}

func (l *logger) Info(msg string, fields ...Field) {
	if !l.IsLevelEnabled(LevelInfo) {
		return
	}
	l.log(LevelInfo, msg, fields...)
}

func (l *logger) Warn(msg string, fields ...Field) {
	if !l.IsLevelEnabled(LevelWarn) {
		return
	}
	l.log(LevelWarn, msg, fields...)
}

func (l *logger) Error(msg string, fields ...Field) {
	if !l.IsLevelEnabled(LevelError) {
		return
	}
	l.log(LevelError, msg, fields...)
}

func (l *logger) Fatal(msg string, fields ...Field) {
	l.log(LevelFatal, msg, fields...)
	os.Exit(1)
}

func (l *logger) WithContext(ctx context.Context) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return &logger{
		config: l.config,
		fields: l.copyFields(),
		ctx:    ctx,
	}
}

func (l *logger) WithFields(fields ...Field) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newFields := l.copyFields()
	for _, field := range fields {
		newFields[field.Key] = field.Value
	}

	return &logger{
		config: l.config,
		fields: newFields,
		ctx:    l.ctx,
	}
}

func (l *logger) WithError(err error) Logger {
	return l.WithFields(Err(err))
}

func (l *logger) With(key string, value any) Logger {
	return l.WithFields(Field{Key: key, Value: value})
}

func (l *logger) WithCorrelationID(id string) Logger {
	return l.WithFields(String("correlation_id", id))
}

func (l *logger) WithTraceID(id string) Logger {
	return l.WithFields(String("trace_id", id))
}

func (l *logger) WithRequestID(id string) Logger {
	return l.WithFields(String("request_id", id))
}

func (l *logger) WithUserID(id string) Logger {
	return l.WithFields(String("user_id", id))
}

func (l *logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Level = level
}

func (l *logger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config.Level
}

func (l *logger) IsLevelEnabled(level LogLevel) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level >= l.config.Level
}

func (l *logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Output = w
}

func (l *logger) log(level LogLevel, msg string, fields ...Field) {
	entry := &LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
		Fields:    l.buildFields(fields...),
	}

	if l.config.ShowCaller {
		entry.Caller = l.getCaller()
	}

	l.addCorrelationFromContext(entry)

	output := l.formatEntry(entry)
	if _, err := l.config.Output.Write([]byte(output)); err != nil {
		fmt.Fprintf(os.Stderr, "Logger write error: %v\n", err)
	}
}

func (l *logger) buildFields(fields ...Field) map[string]any {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := l.copyFields()
	for _, field := range fields {
		result[field.Key] = field.Value
	}
	return result
}

func (l *logger) copyFields() map[string]any {
	result := make(map[string]any, len(l.fields))
	for k, v := range l.fields {
		result[k] = v
	}
	return result
}

func (l *logger) getCaller() string {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown"
	}

	parts := strings.Split(file, "/")
	if len(parts) > 0 {
		file = parts[len(parts)-1]
	}

	return fmt.Sprintf("%s:%d", file, line)
}

func (l *logger) addCorrelationFromContext(entry *LogEntry) {
	if l.ctx == nil {
		return
	}

	correlationKeys := []contextKey{
		ContextKeyRequestID,
		ContextKeyTraceID,
		ContextKeyCorrelationID,
		ContextKeyUserID,
		ContextKeySessionID,
	}

	for _, key := range correlationKeys {
		if value := l.ctx.Value(key); value != nil {
			if entry.Fields == nil {
				entry.Fields = make(map[string]any)
			}
			entry.Fields[string(key)] = value
		}
	}
}

func (l *logger) formatEntry(entry *LogEntry) string {
	switch l.config.Format {
	case FormatJSON:
		return l.formatJSON(entry)
	case FormatPretty:
		return l.formatPretty(entry)
	default:
		return l.formatPretty(entry)
	}
}

func (l *logger) formatJSON(entry *LogEntry) string {
	timestamp := entry.Timestamp.Format(time.RFC3339)

	var parts []string
	parts = append(parts, fmt.Sprintf(`"timestamp":"%s"`, timestamp))
	parts = append(parts, fmt.Sprintf(`"level":"%s"`, entry.Level.String()))
	parts = append(parts, fmt.Sprintf(`"message":"%s"`, entry.Message))

	if entry.Caller != "" {
		parts = append(parts, fmt.Sprintf(`"caller":"%s"`, entry.Caller))
	}

	for key, value := range entry.Fields {
		parts = append(parts, fmt.Sprintf(`"%s":"%v"`, key, value))
	}

	return fmt.Sprintf("{%s}\n", strings.Join(parts, ","))
}

func (l *logger) formatPretty(entry *LogEntry) string {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")

	levelColor, resetColor := l.getLevelColors(entry.Level)

	var builder strings.Builder

	builder.WriteString(ColorGray)
	builder.WriteString(timestamp)
	builder.WriteString(resetColor)
	builder.WriteString(" ")

	builder.WriteString(levelColor)
	builder.WriteString(fmt.Sprintf("[%-5s]", entry.Level.String()))
	builder.WriteString(resetColor)
	builder.WriteString(" ")

	if entry.Caller != "" {
		builder.WriteString(ColorCyan)
		builder.WriteString(fmt.Sprintf("%-20s", entry.Caller))
		builder.WriteString(resetColor)
		builder.WriteString(" ")
	}

	builder.WriteString(entry.Message)

	if len(entry.Fields) > 0 {
		builder.WriteString(" ")
		builder.WriteString(ColorGray)
		builder.WriteString("[")

		first := true
		for key, value := range entry.Fields {
			if !first {
				builder.WriteString(" ")
			}
			builder.WriteString(fmt.Sprintf("%s=%v", key, value))
			first = false
		}

		builder.WriteString("]")
		builder.WriteString(resetColor)
	}

	builder.WriteString("\n")
	return builder.String()
}

func (l *logger) getLevelColors(level LogLevel) (levelColor, resetColor string) {
	if !l.config.ShowColor {
		return "", ""
	}

	switch level {
	case LevelDebug:
		return ColorGray, ColorReset
	case LevelInfo:
		return ColorGreen, ColorReset
	case LevelWarn:
		return ColorYellow, ColorReset
	case LevelError:
		return ColorRed, ColorReset
	case LevelFatal:
		return ColorRed, ColorReset
	default:
		return ColorWhite, ColorReset
	}
}
