package wgrok

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	wmh "github.com/3rg0n/webex-message-handler/go"
)

// Ensure our loggers satisfy the webex-message-handler Logger interface.
var (
	_ wmh.Logger = (*NdjsonLogger)(nil)
	_ wmh.Logger = (*minLevelWgrokLogger)(nil)
)

// NdjsonLogger emits NDJSON log lines to stderr.
type NdjsonLogger struct {
	Module string
}

type logLine struct {
	Ts     string `json:"ts"`
	Level  string `json:"level"`
	Msg    string `json:"msg"`
	Module string `json:"module"`
}

func (l *NdjsonLogger) write(level, msg string) {
	line := logLine{
		Ts:     time.Now().UTC().Format(time.RFC3339Nano),
		Level:  level,
		Msg:    msg,
		Module: l.Module,
	}
	data, _ := json.Marshal(line)
	fmt.Fprintln(os.Stderr, string(data))
}

func (l *NdjsonLogger) Debug(msg string, _ ...any) { l.write("DEBUG", msg) }
func (l *NdjsonLogger) Info(msg string, _ ...any)  { l.write("INFO", msg) }
func (l *NdjsonLogger) Warn(msg string, _ ...any)  { l.write("WARNING", msg) }
func (l *NdjsonLogger) Error(msg string, _ ...any) { l.write("ERROR", msg) }

// minLevelWgrokLogger logs only WARN and ERROR, suppressing DEBUG and INFO.
type minLevelWgrokLogger struct {
	ndjson *NdjsonLogger
}

func (m *minLevelWgrokLogger) Debug(string, ...any) {}
func (m *minLevelWgrokLogger) Info(string, ...any)  {}
func (m *minLevelWgrokLogger) Warn(msg string, args ...any) {
	m.ndjson.write("WARNING", msg)
}
func (m *minLevelWgrokLogger) Error(msg string, args ...any) {
	m.ndjson.write("ERROR", msg)
}

// GetLogger returns an NdjsonLogger if debug is true, otherwise a minLevelWgrokLogger that only outputs WARN/ERROR.
func GetLogger(debug bool, module string) wmh.Logger {
	if debug {
		return &NdjsonLogger{Module: module}
	}
	return &minLevelWgrokLogger{ndjson: &NdjsonLogger{Module: module}}
}
