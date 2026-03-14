"""Tests for wgrok.irc - IRC connection string parsing and client."""

import json
from pathlib import Path

import pytest

from wgrok.irc import parse_connection_string

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "irc_cases.json").read_text())


class TestParseConnectionString:
    @pytest.mark.parametrize(
        "tc",
        [c for c in CASES["parse_connection_string"] if not c.get("expected_error")],
        ids=lambda tc: tc["name"],
    )
    def test_valid_cases(self, tc):
        result = parse_connection_string(tc["input"])
        assert result == tc["expected"]

    @pytest.mark.parametrize(
        "tc",
        [c for c in CASES["parse_connection_string"] if c.get("expected_error")],
        ids=lambda tc: tc["name"],
    )
    def test_error_cases(self, tc):
        with pytest.raises(ValueError):
            parse_connection_string(tc["input"])
