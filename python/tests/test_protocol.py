"""Tests for wgrok.protocol - driven by shared test cases."""

import json
from pathlib import Path

import pytest

from wgrok.protocol import (
    ECHO_PREFIX,
    format_echo,
    format_response,
    is_echo,
    parse_echo,
    parse_response,
)

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "protocol_cases.json").read_text())


class TestEchoPrefix:
    def test_constant(self):
        assert CASES["echo_prefix"] == ECHO_PREFIX


class TestFormatEcho:
    @pytest.mark.parametrize("tc", CASES["format_echo"], ids=lambda tc: tc["expected"])
    def test_cases(self, tc):
        assert format_echo(tc["slug"], tc["payload"]) == tc["expected"]


class TestParseEcho:
    @pytest.mark.parametrize("tc", CASES["parse_echo"]["valid"], ids=lambda tc: tc["input"])
    def test_valid(self, tc):
        slug, payload = parse_echo(tc["input"])
        assert slug == tc["slug"]
        assert payload == tc["payload"]

    @pytest.mark.parametrize("tc", CASES["parse_echo"]["errors"], ids=lambda tc: tc["input"])
    def test_errors(self, tc):
        with pytest.raises(ValueError, match="(?i)" + tc["error_contains"]):
            parse_echo(tc["input"])


class TestIsEcho:
    @pytest.mark.parametrize("tc", CASES["is_echo"], ids=lambda tc: tc["input"] or "(empty)")
    def test_cases(self, tc):
        assert is_echo(tc["input"]) is tc["expected"]


class TestFormatResponse:
    @pytest.mark.parametrize("tc", CASES["format_response"], ids=lambda tc: tc["expected"])
    def test_cases(self, tc):
        assert format_response(tc["slug"], tc["payload"]) == tc["expected"]


class TestParseResponse:
    @pytest.mark.parametrize("tc", CASES["parse_response"]["valid"], ids=lambda tc: tc["input"])
    def test_valid(self, tc):
        slug, payload = parse_response(tc["input"])
        assert slug == tc["slug"]
        assert payload == tc["payload"]

    @pytest.mark.parametrize("tc", CASES["parse_response"]["errors"], ids=lambda tc: tc["input"])
    def test_errors(self, tc):
        with pytest.raises(ValueError, match="(?i)" + tc["error_contains"]):
            parse_response(tc["input"])


class TestRoundtrips:
    @pytest.mark.parametrize("tc", CASES["roundtrips"]["echo"])
    def test_echo(self, tc):
        text = format_echo(tc["slug"], tc["payload"])
        slug, payload = parse_echo(text)
        assert slug == tc["slug"]
        assert payload == tc["payload"]

    @pytest.mark.parametrize("tc", CASES["roundtrips"]["response"])
    def test_response(self, tc):
        text = format_response(tc["slug"], tc["payload"])
        slug, payload = parse_response(text)
        assert slug == tc["slug"]
        assert payload == tc["payload"]
