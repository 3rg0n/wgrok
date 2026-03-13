"""Tests for wgrok.protocol - message format, parse, and validate."""

import pytest

from wgrok.protocol import (
    ECHO_PREFIX,
    format_echo,
    format_response,
    is_echo,
    parse_echo,
    parse_response,
)


class TestFormatEcho:
    def test_basic(self):
        assert format_echo("myslug", "hello") == "./echo:myslug:hello"

    def test_empty_payload(self):
        assert format_echo("myslug", "") == "./echo:myslug:"

    def test_payload_with_colons(self):
        assert format_echo("slug", "a:b:c") == "./echo:slug:a:b:c"


class TestParseEcho:
    def test_basic(self):
        slug, payload = parse_echo("./echo:myslug:hello")
        assert slug == "myslug"
        assert payload == "hello"

    def test_empty_payload(self):
        slug, payload = parse_echo("./echo:myslug:")
        assert slug == "myslug"
        assert payload == ""

    def test_colons_in_payload(self):
        slug, payload = parse_echo("./echo:slug:a:b:c")
        assert slug == "slug"
        assert payload == "a:b:c"

    def test_not_echo_raises(self):
        with pytest.raises(ValueError, match="Not an echo message"):
            parse_echo("plain text")

    def test_empty_slug_raises(self):
        with pytest.raises(ValueError, match="Empty slug"):
            parse_echo("./echo::payload")

    def test_roundtrip(self):
        original_slug, original_payload = "agent1", "some data here"
        text = format_echo(original_slug, original_payload)
        slug, payload = parse_echo(text)
        assert slug == original_slug
        assert payload == original_payload


class TestIsEcho:
    def test_echo_message(self):
        assert is_echo("./echo:slug:payload") is True

    def test_not_echo(self):
        assert is_echo("slug:payload") is False

    def test_empty_string(self):
        assert is_echo("") is False

    def test_prefix_constant(self):
        assert ECHO_PREFIX == "./echo:"


class TestFormatResponse:
    def test_basic(self):
        assert format_response("myslug", "hello") == "myslug:hello"

    def test_empty_payload(self):
        assert format_response("myslug", "") == "myslug:"


class TestParseResponse:
    def test_basic(self):
        slug, payload = parse_response("myslug:hello")
        assert slug == "myslug"
        assert payload == "hello"

    def test_colons_in_payload(self):
        slug, payload = parse_response("slug:a:b:c")
        assert slug == "slug"
        assert payload == "a:b:c"

    def test_no_colon(self):
        slug, payload = parse_response("justslug")
        assert slug == "justslug"
        assert payload == ""

    def test_empty_slug_raises(self):
        with pytest.raises(ValueError, match="Empty slug"):
            parse_response(":payload")

    def test_roundtrip(self):
        text = format_response("agent1", "data")
        slug, payload = parse_response(text)
        assert slug == "agent1"
        assert payload == "data"
