"""Tests for wgrok.protocol - driven by shared test cases."""

import json
from pathlib import Path

import pytest

from wgrok.protocol import (
    ECHO_PREFIX,
    format_echo,
    format_flags,
    format_response,
    is_echo,
    is_pause,
    is_resume,
    parse_echo,
    parse_flags,
    parse_response,
)

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "protocol_cases.json").read_text())


class TestEchoPrefix:
    def test_constant(self):
        assert CASES["echo_prefix"] == ECHO_PREFIX


class TestFormatEcho:
    @pytest.mark.parametrize("tc", CASES["format_echo"], ids=lambda tc: tc["expected"])
    def test_cases(self, tc):
        assert format_echo(tc["to"], tc["from"], tc["flags"], tc["payload"]) == tc["expected"]


class TestParseEcho:
    @pytest.mark.parametrize("tc", CASES["parse_echo"]["valid"], ids=lambda tc: tc["input"])
    def test_valid(self, tc):
        to, from_slug, flags, payload = parse_echo(tc["input"])
        assert to == tc["to"]
        assert from_slug == tc["from"]
        assert flags == tc["flags"]
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
        assert format_response(tc["to"], tc["from"], tc["flags"], tc["payload"]) == tc["expected"]


class TestParseResponse:
    @pytest.mark.parametrize("tc", CASES["parse_response"]["valid"], ids=lambda tc: tc["input"])
    def test_valid(self, tc):
        to, from_slug, flags, payload = parse_response(tc["input"])
        assert to == tc["to"]
        assert from_slug == tc["from"]
        assert flags == tc["flags"]
        assert payload == tc["payload"]

    @pytest.mark.parametrize("tc", CASES["parse_response"]["errors"], ids=lambda tc: tc["input"])
    def test_errors(self, tc):
        with pytest.raises(ValueError, match="(?i)" + tc["error_contains"]):
            parse_response(tc["input"])


class TestParseFlags:
    @pytest.mark.parametrize("tc", CASES["parse_flags"], ids=lambda tc: tc["input"])
    def test_cases(self, tc):
        compressed, encrypted, chunk_seq, chunk_total = parse_flags(tc["input"])
        assert compressed is tc["compressed"]
        assert encrypted is tc["encrypted"]
        assert chunk_seq == tc["chunk_seq"]
        assert chunk_total == tc["chunk_total"]


class TestFormatFlags:
    @pytest.mark.parametrize("tc", CASES["format_flags"], ids=lambda tc: tc["expected"])
    def test_cases(self, tc):
        result = format_flags(tc["compressed"], tc["encrypted"], tc["chunk_seq"], tc["chunk_total"])
        assert result == tc["expected"]


class TestIsPause:
    @pytest.mark.parametrize("tc", CASES["is_pause"], ids=lambda tc: tc["input"] or "(empty)")
    def test_cases(self, tc):
        assert is_pause(tc["input"]) is tc["expected"]


class TestIsResume:
    @pytest.mark.parametrize("tc", CASES["is_resume"], ids=lambda tc: tc["input"] or "(empty)")
    def test_cases(self, tc):
        assert is_resume(tc["input"]) is tc["expected"]


class TestRoundtrips:
    @pytest.mark.parametrize("tc", CASES["roundtrips"]["echo"])
    def test_echo(self, tc):
        text = format_echo(tc["to"], tc["from"], tc["flags"], tc["payload"])
        to, from_slug, flags, payload = parse_echo(text)
        assert to == tc["to"]
        assert from_slug == tc["from"]
        assert flags == tc["flags"]
        assert payload == tc["payload"]

    @pytest.mark.parametrize("tc", CASES["roundtrips"]["response"])
    def test_response(self, tc):
        text = format_response(tc["to"], tc["from"], tc["flags"], tc["payload"])
        to, from_slug, flags, payload = parse_response(text)
        assert to == tc["to"]
        assert from_slug == tc["from"]
        assert flags == tc["flags"]
        assert payload == tc["payload"]
