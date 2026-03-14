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
	_ wmh.Logger = (*noopWgrokLogger)(nil)
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

// noopWgrokLogger is a silent logger.
type noopWgrokLogger struct{}

func (noopWgrokLogger) Debug(string, ...any) {}
func (noopWgrokLogger) Info(string, ...any)  {}
func (noopWgrokLogger) Warn(string, ...any)  {}
func (noopWgrokLogger) Error(string, ...any) {}

// GetLogger returns an NdjsonLogger if debug is true, otherwise a silent logger.
func GetLogger(debug bool, module string) wmh.Logger {
	if debug {
		return &NdjsonLogger{Module: module}
	}
	return noopWgrokLogger{}
}
