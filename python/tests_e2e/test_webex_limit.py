"""E2E test: verify Webex message size limit against live API.

Requires WEBEX_TOKEN and WEBEX_TARGET in the root .env file.
These tests make real API calls — skip with `pytest -m "not e2e"`.
"""

import os

import aiohttp
import pytest

# Load token from root .env (first WEBEX_TOKEN line)
ROOT_ENV = os.path.join(os.path.dirname(__file__), "..", "..", ".env")


def _load_first_token() -> str | None:
    """Read first WEBEX_TOKEN from root .env."""
    if not os.path.exists(ROOT_ENV):
        return None
    with open(ROOT_ENV) as f:
        for line in f:
            line = line.strip()
            if line.startswith("WEBEX_TOKEN="):
                return line.split("=", 1)[1]
    return None


WEBEX_TOKEN = _load_first_token()
WEBEX_TARGET = os.environ.get("WEBEX_TARGET", "pongmon@webex.bot")
WEBEX_MESSAGES_URL = "https://webexapis.com/v1/messages"

skip_no_token = pytest.mark.skipif(
    not WEBEX_TOKEN, reason="No WEBEX_TOKEN in root .env"
)


async def _send_text(token: str, target: str, text: str) -> tuple[int, dict]:
    """Send a message and return (status_code, response_json or error body)."""
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    payload = {"toPersonEmail": target, "text": text}
    async with aiohttp.ClientSession() as session:
        async with session.post(WEBEX_MESSAGES_URL, headers=headers, json=payload) as resp:
            body = await resp.json() if resp.content_type == "application/json" else {}
            return resp.status, body


@skip_no_token
class TestWebexMessageLimit:
    async def test_7439_bytes_succeeds(self):
        """Webex accepts a message of exactly 7439 bytes."""
        text = "A" * 7439
        assert len(text.encode("utf-8")) == 7439

        status, body = await _send_text(WEBEX_TOKEN, WEBEX_TARGET, text)
        assert status == 200, f"Expected 200 for 7439 bytes, got {status}: {body}"

    async def test_7440_bytes_fails(self):
        """Webex rejects a message of 7440 bytes."""
        text = "A" * 7440
        assert len(text.encode("utf-8")) == 7440

        status, body = await _send_text(WEBEX_TOKEN, WEBEX_TARGET, text)
        assert status != 200, f"Expected rejection for 7440 bytes, got {status}: {body}"
