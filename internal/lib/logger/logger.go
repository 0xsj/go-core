package logger

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