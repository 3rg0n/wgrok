"""Thin Webex REST client for sending messages."""

from __future__ import annotations

import aiohttp

WEBEX_API_URL = "https://webexapis.com/v1/messages"


async def send_message(
    token: str,
    to_email: str,
    text: str,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Send a Webex message to a person by email. Returns the API response JSON."""
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    payload = {"toPersonEmail": to_email, "text": text}

    owns_session = session is None
    if owns_session:
        session = aiohttp.ClientSession()
    try:
        async with session.post(WEBEX_API_URL, json=payload, headers=headers) as resp:
            resp.raise_for_status()
            return await resp.json()
    finally:
        if owns_session:
            await session.close()
