"""Tests for wgrok.allowlist - domain/email pattern matching."""

from wgrok.allowlist import Allowlist


class TestAllowlist:
    def test_domain_match(self):
        al = Allowlist(["example.com"])
        assert al.is_allowed("user@example.com") is True

    def test_domain_no_match(self):
        al = Allowlist(["example.com"])
        assert al.is_allowed("user@other.com") is False

    def test_wildcard_match(self):
        al = Allowlist(["*@example.com"])
        assert al.is_allowed("anyone@example.com") is True

    def test_exact_match(self):
        al = Allowlist(["admin@example.com"])
        assert al.is_allowed("admin@example.com") is True
        assert al.is_allowed("other@example.com") is False

    def test_case_insensitive(self):
        al = Allowlist(["Example.COM"])
        assert al.is_allowed("User@EXAMPLE.com") is True

    def test_multiple_patterns(self):
        al = Allowlist(["example.com", "user@trusted.org"])
        assert al.is_allowed("anyone@example.com") is True
        assert al.is_allowed("user@trusted.org") is True
        assert al.is_allowed("other@trusted.org") is False

    def test_empty_patterns(self):
        al = Allowlist([])
        assert al.is_allowed("user@example.com") is False

    def test_whitespace_stripped(self):
        al = Allowlist(["  example.com  ", " user@test.com "])
        assert al.is_allowed("a@example.com") is True
        assert al.is_allowed("user@test.com") is True

    def test_empty_string_pattern_ignored(self):
        al = Allowlist(["", "example.com", "  "])
        assert al.is_allowed("a@example.com") is True
