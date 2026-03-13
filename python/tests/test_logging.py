"""Tests for wgrok.logging - NDJSON logger and noop logger."""

import json

from wgrok.logging import NdjsonLogger, NoopLogger, get_logger


class TestNdjsonLogger:
    def test_info_output(self, capsys):
        logger = NdjsonLogger("wgrok.test")
        logger.info("hello world")
        captured = capsys.readouterr()
        line = json.loads(captured.err.strip())
        assert line["level"] == "INFO"
        assert line["msg"] == "hello world"
        assert line["module"] == "wgrok.test"
        assert "ts" in line

    def test_debug_output(self, capsys):
        logger = NdjsonLogger()
        logger.debug("debug msg")
        line = json.loads(capsys.readouterr().err.strip())
        assert line["level"] == "DEBUG"

    def test_warning_output(self, capsys):
        logger = NdjsonLogger()
        logger.warning("warn msg")
        line = json.loads(capsys.readouterr().err.strip())
        assert line["level"] == "WARNING"

    def test_error_output(self, capsys):
        logger = NdjsonLogger()
        logger.error("error msg")
        line = json.loads(capsys.readouterr().err.strip())
        assert line["level"] == "ERROR"

    def test_default_module(self, capsys):
        logger = NdjsonLogger()
        logger.info("test")
        line = json.loads(capsys.readouterr().err.strip())
        assert line["module"] == "wgrok"


class TestNoopLogger:
    def test_silent(self, capsys):
        logger = NoopLogger()
        logger.debug("x")
        logger.info("x")
        logger.warning("x")
        logger.error("x")
        captured = capsys.readouterr()
        assert captured.out == ""
        assert captured.err == ""


class TestGetLogger:
    def test_debug_true_returns_ndjson(self):
        logger = get_logger(True)
        assert isinstance(logger, NdjsonLogger)

    def test_debug_false_returns_noop(self):
        logger = get_logger(False)
        assert isinstance(logger, NoopLogger)

    def test_custom_module(self):
        logger = get_logger(True, "custom.mod")
        assert isinstance(logger, NdjsonLogger)
        assert logger._module == "custom.mod"
