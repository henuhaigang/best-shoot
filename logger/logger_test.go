package logger

import (
	"bytes"
	"strings"
	"testing"
)

func capture(component string, level Level) (*Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	l := New(component, &buf)
	l.SetLevel(level)
	return l, &buf
}

func TestLogLevelFiltering(t *testing.T) {
	l, buf := capture("test", LevelWarn)

	l.Debug("nope")
	l.Info("nope")
	l.Warn("yes")
	l.Error("yes")

	out := buf.String()
	if strings.Contains(out, "nope") {
		t.Error("debug/info should be filtered at warn level")
	}
	if !strings.Contains(out, "[WARN]") || !strings.Contains(out, "[ERROR]") {
		t.Error("warn/error should pass")
	}
}

func TestLogFormat(t *testing.T) {
	l, buf := capture("server", LevelInfo)
	l.Info("client connected", F("clientID", "node-1"))

	out := buf.String()
	for _, want := range []string{"[INFO]", "server:", "client connected", "clientID=node-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestLogWithFields(t *testing.T) {
	l, buf := capture("tunnel", LevelInfo)
	l2 := l.With(F("tunnelID", 5))
	l2.Info("serving")

	out := buf.String()
	if !strings.Contains(out, "tunnelID=5") {
		t.Errorf("output missing field: %s", out)
	}
}

func TestLogWithMultipleFields(t *testing.T) {
	l, buf := capture("client", LevelDebug)
	l.Debug("data", F("tunnelID", 1), F("connID", 3), F("bytes", 1024))

	out := buf.String()
	for _, want := range []string{"tunnelID=1", "connID=3", "bytes=1024"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
		err   bool
	}{
		{"debug", LevelDebug, false},
		{"INFO", LevelInfo, false},
		{"Warn", LevelWarn, false},
		{"ERROR", LevelError, false},
		{"bogus", LevelInfo, true},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("ParseLevel(%q) error = %v, wantErr %v", tt.input, err, tt.err)
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDefaultLogger(t *testing.T) {
	l := Default("test")
	if l == nil {
		t.Fatal("Default returned nil")
	}
}

func TestTimestampFormat(t *testing.T) {
	l, buf := capture("test", LevelInfo)
	l.Info("ts")

	out := buf.String()
	// Should start with ISO-like timestamp
	if len(out) < 23 || out[10] != 'T' {
		t.Errorf("unexpected timestamp format: %s", out[:30])
	}
}
