"""Tests for wgrok.platform - platform dispatch."""

import json
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

from wgrok.platform import platform_send_card, platform_send_message

CASES = json.loads((Path(__file__).resolve().parents[2] / "tests" / "platform_dispatch_cases.json").read_text())


class TestPlatformDispatch:
    @pytest.mark.parametrize(
        "tc",
        [c for c in CASES["cases"] if not c.get("expected_error")],
        ids=lambda tc: tc["name"],
    )
    async def test_routes_to_correct_module(self, tc):
        mock = AsyncMock(return_value={"id": "msg-1"})
        module = tc["expected_module"]
        with patch(f"wgrok.platform.{module}.send_message", mock):
            await platform_send_message(tc["platform"], "token", "target", "text")
        mock.assert_called_once()

    @pytest.mark.parametrize(
        "tc",
        [c for c in CASES["cases"] if not c.get("expected_error")],
        ids=lambda tc: tc["name"],
    )
    async def test_routes_card_to_correct_module(self, tc):
        mock = AsyncMock(return_value={"id": "card-1"})
        module = tc["expected_module"]
        with patch(f"wgrok.platform.{module}.send_card", mock):
            await platform_send_card(tc["platform"], "token", "target", "text", {"body": []})
        mock.assert_called_once()

    @pytest.mark.parametrize(
        "tc",
        [c for c in CASES["cases"] if c.get("expected_error")],
        ids=lambda tc: tc["name"],
    )
    async def test_invalid_platform_raises(self, tc):
        with pytest.raises(ValueError, match="Unsupported platform"):
            await platform_send_message(tc["platform"], "token", "target", "text")

    @pytest.mark.parametrize(
        "tc",
        [c for c in CASES["cases"] if c.get("expected_error")],
        ids=lambda tc: tc["name"],
    )
    async def test_invalid_platform_card_raises(self, tc):
        with pytest.raises(ValueError, match="Unsupported platform"):
            await platform_send_card(tc["platform"], "token", "target", "text", {})
