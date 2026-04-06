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


class MinLevelLogger:
    """Logger that only outputs WARN and ERROR, suppressing DEBUG and INFO."""

    def __init__(self, module: str = "wgrok") -> None:
        self._module = module
        self._ndjson = NdjsonLogger(module)

    def debug(self, msg: str) -> None:
        pass

    def info(self, msg: str) -> None:
        pass

    def warning(self, msg: str) -> None:
        self._ndjson.warning(msg)

    def error(self, msg: str) -> None:
        self._ndjson.error(msg)


def get_logger(debug: bool, module: str = "wgrok") -> NdjsonLogger | MinLevelLogger:
    """Return an NdjsonLogger if debug is True, otherwise a MinLevelLogger that only outputs WARN/ERROR."""
    if debug:
        return NdjsonLogger(module)
    return MinLevelLogger(module)
