package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

// ParseLevel parses a level string.
func ParseLevel(s string) (Level, error) {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LevelDebug, nil
	case "INFO":
		return LevelInfo, nil
	case "WARN":
		return LevelWarn, nil
	case "ERROR":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown level: %s", s)
	}
}

// Logger is a structured logger with levels and components.
type Logger struct {
	mu        sync.Mutex
	level     Level
	component string
	logger    *log.Logger
	fields    []Field
}

// Field is a key-value pair for structured logging.
type Field struct {
	Key   string
	Value interface{}
}

// F creates a field.
func F(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// New creates a logger with the given component name and output writer.
func New(component string, w io.Writer) *Logger {
	return &Logger{
		level:     LevelInfo,
		component: component,
		logger:    log.New(w, "", 0),
	}
}

// Default creates a logger writing to stderr.
func Default(component string) *Logger {
	return New(component, os.Stderr)
}

// SetLevel sets the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// With returns a new logger with additional fields.
func (l *Logger) With(fields ...Field) *Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)
	return &Logger{
		level:     l.level,
		component: l.component,
		logger:    l.logger,
		fields:    newFields,
	}
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs at info level.
func (l *Logger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs at error level.
func (l *Logger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

func (l *Logger) log(level Level, msg string, fields ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	ts := time.Now().Format("2006-01-02T15:04:05.000")
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	var sb strings.Builder
	sb.WriteString(ts)
	sb.WriteString(" [")
	sb.WriteString(levelNames[level])
	sb.WriteString("] ")
	sb.WriteString(l.component)
	sb.WriteString(": ")
	sb.WriteString(msg)

	for _, f := range allFields {
		sb.WriteString(" ")
		sb.WriteString(f.Key)
		sb.WriteString("=")
		sb.WriteString(fmt.Sprintf("%v", f.Value))
	}

	l.logger.Output(2, sb.String())
}
