"""Tests for wgrok.allowlist - driven by shared test cases."""

import json
from pathlib import Path

import pytest

from wgrok.allowlist import Allowlist

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "allowlist_cases.json").read_text())


class TestAllowlist:
    @pytest.mark.parametrize("tc", CASES["cases"], ids=lambda tc: tc["name"])
    def test_cases(self, tc):
        al = Allowlist(tc["patterns"])
        assert al.is_allowed(tc["email"]) is tc["expected"]
