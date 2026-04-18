"""NDJSON logging for wgrok, compatible with webex-message-handler's Logger protocol."""

from __future__ import annotations

import json
import sys
from datetime import datetime, timezone


class NdjsonLogger:
    """Emits NDJSON log lines to stderr."""

    def __init__(self, module: str = "wgrok") -> None:
        self._module = module

    def _write(self, level: str, msg: str, **kwargs: str) -> None:
        line: dict[str, str] = {
            "ts": datetime.now(timezone.utc).isoformat(),
            "level": level,
            "msg": msg,
            "module": self._module,
        }
        if kwargs:
            line.update(kwargs)
        print(json.dumps(line), file=sys.stderr, flush=True)

    def debug(self, msg: str, **kwargs: str) -> None:
        self._write("DEBUG", msg, **kwargs)

    def info(self, msg: str, **kwargs: str) -> None:
        self._write("INFO", msg, **kwargs)

    def warning(self, msg: str, **kwargs: str) -> None:
        self._write("WARNING", msg, **kwargs)

    def error(self, msg: str, **kwargs: str) -> None:
        self._write("ERROR", msg, **kwargs)


class MinLevelLogger:
    """Logger that only outputs WARN and ERROR, suppressing DEBUG and INFO."""

    def __init__(self, module: str = "wgrok") -> None:
        self._module = module
        self._ndjson = NdjsonLogger(module)

    def debug(self, msg: str, **kwargs: str) -> None:
        pass

    def info(self, msg: str, **kwargs: str) -> None:
        pass

    def warning(self, msg: str, **kwargs: str) -> None:
        self._ndjson.warning(msg, **kwargs)

    def error(self, msg: str, **kwargs: str) -> None:
        self._ndjson.error(msg, **kwargs)


def get_logger(debug: bool, module: str = "wgrok") -> NdjsonLogger | MinLevelLogger:
    """Return an NdjsonLogger if debug is True, otherwise a MinLevelLogger that only outputs WARN/ERROR."""
    if debug:
        return NdjsonLogger(module)
    return MinLevelLogger(module)
