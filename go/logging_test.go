package wgrok

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestNdjsonLoggerOutput(t *testing.T) {
	levels := []struct {
		method string
		level  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARNING"},
		{"error", "ERROR"},
	}

	for _, lv := range levels {
		t.Run(lv.method, func(t *testing.T) {
			// Capture stderr
			r, w, _ := os.Pipe()
			origStderr := os.Stderr
			os.Stderr = w

			logger := &NdjsonLogger{Module: "wgrok.test"}
			switch lv.method {
			case "debug":
				logger.Debug("test msg")
			case "info":
				logger.Info("test msg")
			case "warn":
				logger.Warn("test msg")
			case "error":
				logger.Error("test msg")
			}

			w.Close()
			os.Stderr = origStderr

			var buf bytes.Buffer
			buf.ReadFrom(r)
			r.Close()

			var line map[string]string
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("invalid JSON: %v (output: %s)", err, buf.String())
			}
			if line["level"] != lv.level {
				t.Errorf("level = %q, want %q", line["level"], lv.level)
			}
			if line["msg"] != "test msg" {
				t.Errorf("msg = %q, want %q", line["msg"], "test msg")
			}
			if line["module"] != "wgrok.test" {
				t.Errorf("module = %q, want %q", line["module"], "wgrok.test")
			}
			if line["ts"] == "" {
				t.Error("missing ts field")
			}
		})
	}
}

func TestNoopLoggerSilent(t *testing.T) {
	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w

	logger := noopWgrokLogger{}
	logger.Debug("x")
	logger.Info("x")
	logger.Warn("x")
	logger.Error("x")

	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	r.Close()

	if buf.Len() != 0 {
		t.Errorf("noop logger produced output: %s", buf.String())
	}
}

func TestGetLoggerDebugTrue(t *testing.T) {
	logger := GetLogger(true, "test")
	if _, ok := logger.(*NdjsonLogger); !ok {
		t.Errorf("expected *NdjsonLogger, got %T", logger)
	}
}

func TestGetLoggerDebugFalse(t *testing.T) {
	logger := GetLogger(false, "test")
	if _, ok := logger.(noopWgrokLogger); !ok {
		t.Errorf("expected noopWgrokLogger, got %T", logger)
	}
}
