"""NDJSON logging for wgrok, compatible with webex-message-handler's Logger protocol."""

from __future__ import annotations

import json
import sys
from datetime import datetime, timezone


class NdjsonLogger:
    """Emits NDJSON log lines to stderr."""

    def __init__(self, module: str = "wgrok") -> None:
        self._module = module

    def _write(self, level: str, msg: str) -> None:
        line = json.dumps({
            "ts": datetime.now(timezone.utc).isoformat(),
            "level": level,
            "msg": msg,
            "module": self._module,
        })
        print(line, file=sys.stderr, flush=True)

    def debug(self, msg: str) -> None:
        self._write("DEBUG", msg)

    def info(self, msg: str) -> None:
        self._write("INFO", msg)

    def warning(self, msg: str) -> None:
        self._write("WARNING", msg)

    def error(self, msg: str) -> None:
        self._write("ERROR", msg)


class NoopLogger:
    """Silent logger that discards all messages."""

    def debug(self, msg: str) -> None:
        pass

    def info(self, msg: str) -> None:
        pass

    def warning(self, msg: str) -> None:
        pass

    def error(self, msg: str) -> None:
        pass


def get_logger(debug: bool, module: str = "wgrok") -> NdjsonLogger | NoopLogger:
    """Return an NdjsonLogger if debug is True, otherwise a NoopLogger."""
    if debug:
        return NdjsonLogger(module)
    return NoopLogger()
