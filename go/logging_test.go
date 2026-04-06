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

func TestMinLevelLoggerSuppressesDebugInfo(t *testing.T) {
	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w

	logger := &minLevelWgrokLogger{ndjson: &NdjsonLogger{Module: "test"}}
	logger.Debug("x")
	logger.Info("x")

	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	r.Close()

	if buf.Len() != 0 {
		t.Errorf("minLevelLogger produced output for debug/info: %s", buf.String())
	}
}

func TestMinLevelLoggerEmitsWarnError(t *testing.T) {
	r, w, _ := os.Pipe()
	origStderr := os.Stderr
	os.Stderr = w

	logger := &minLevelWgrokLogger{ndjson: &NdjsonLogger{Module: "test"}}
	logger.Warn("warn msg")
	logger.Error("error msg")

	w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	r.Close()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %s", len(lines), buf.String())
	}

	var line1, line2 map[string]string
	json.Unmarshal(lines[0], &line1)
	json.Unmarshal(lines[1], &line2)
	if line1["level"] != "WARNING" {
		t.Errorf("line1 level = %q, want WARNING", line1["level"])
	}
	if line2["level"] != "ERROR" {
		t.Errorf("line2 level = %q, want ERROR", line2["level"])
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
	if _, ok := logger.(*minLevelWgrokLogger); !ok {
		t.Errorf("expected *minLevelWgrokLogger, got %T", logger)
	}
}
