package logger

import "time"

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
	FormatJson
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

func Int() {}

func Bool() {}

func Duration() {}

func Any() {}

func Err() {}

type LoggerConfig struct {}

func DefaultConfig() {}

type Logger interface {}

type logger struct {}

func NewLogger() {}

func Debug() {}

func Info() {}

func Warn() {}

func Error() {}

func Fatal() {}

func WithContext() {}

func WithFields() {}

func WithError() {}

func With() {}

func WithCorrelationID() {}

func WithTraceID(){}

func WithRequestID() {}

func WithUserID() {}

func SetLevel() {}

func GetLevel() {}

func IsLevelEnabled() {}

func SetOutput() {}

func log() {}

func buildFields() {}

func copyFields() {}

func getCaller() {}

func addCorrelationFromContext() {}

func formatEntry() {}

func formatJSON() {}

func formatPretty() {}

func getLevelColors() {}