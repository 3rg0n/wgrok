"""Tests for wgrok.logging - NDJSON logger and min-level logger."""

import json

from wgrok.logging import MinLevelLogger, NdjsonLogger, get_logger


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


class TestMinLevelLogger:
    def test_suppresses_debug_and_info(self, capsys):
        logger = MinLevelLogger()
        logger.debug("x")
        logger.info("x")
        captured = capsys.readouterr()
        assert captured.out == ""
        assert captured.err == ""

    def test_emits_warning(self, capsys):
        logger = MinLevelLogger("test")
        logger.warning("warn msg")
        line = json.loads(capsys.readouterr().err.strip())
        assert line["level"] == "WARNING"
        assert line["msg"] == "warn msg"

    def test_emits_error(self, capsys):
        logger = MinLevelLogger("test")
        logger.error("error msg")
        line = json.loads(capsys.readouterr().err.strip())
        assert line["level"] == "ERROR"
        assert line["msg"] == "error msg"


class TestGetLogger:
    def test_debug_true_returns_ndjson(self):
        logger = get_logger(True)
        assert isinstance(logger, NdjsonLogger)

    def test_debug_false_returns_min_level(self):
        logger = get_logger(False)
        assert isinstance(logger, MinLevelLogger)

    def test_custom_module(self):
        logger = get_logger(True, "custom.mod")
        assert isinstance(logger, NdjsonLogger)
        assert logger._module == "custom.mod"
