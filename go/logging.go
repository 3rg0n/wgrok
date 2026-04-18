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

func (l *NdjsonLogger) write(level, msg string, args ...any) {
	line := map[string]string{
		"ts":     time.Now().UTC().Format(time.RFC3339Nano),
		"level":  level,
		"msg":    msg,
		"module": l.Module,
	}
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok {
			line[k] = fmt.Sprint(args[i+1])
		}
	}
	data, _ := json.Marshal(line)
	fmt.Fprintln(os.Stderr, string(data))
}

func (l *NdjsonLogger) Debug(msg string, args ...any) { l.write("DEBUG", msg, args...) }
func (l *NdjsonLogger) Info(msg string, args ...any)  { l.write("INFO", msg, args...) }
func (l *NdjsonLogger) Warn(msg string, args ...any)  { l.write("WARNING", msg, args...) }
func (l *NdjsonLogger) Error(msg string, args ...any) { l.write("ERROR", msg, args...) }

// minLevelWgrokLogger logs only WARN and ERROR, suppressing DEBUG and INFO.
type minLevelWgrokLogger struct {
	ndjson *NdjsonLogger
}

func (m *minLevelWgrokLogger) Debug(string, ...any) {}
func (m *minLevelWgrokLogger) Info(string, ...any)  {}
func (m *minLevelWgrokLogger) Warn(msg string, args ...any) {
	m.ndjson.write("WARNING", msg, args...)
}
func (m *minLevelWgrokLogger) Error(msg string, args ...any) {
	m.ndjson.write("ERROR", msg, args...)
}

// GetLogger returns an NdjsonLogger if debug is true, otherwise a minLevelWgrokLogger that only outputs WARN/ERROR.
func GetLogger(debug bool, module string) wmh.Logger {
	if debug {
		return &NdjsonLogger{Module: module}
	}
	return &minLevelWgrokLogger{ndjson: &NdjsonLogger{Module: module}}
}
