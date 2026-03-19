"""Thin Discord REST client for sending messages and embeds."""

from __future__ import annotations

import asyncio

import aiohttp

DISCORD_API_BASE = "https://discord.com/api/v10"
MAX_RETRIES = 3


def _headers(token: str) -> dict:
    return {"Authorization": f"Bot {token}", "Content-Type": "application/json"}


def _messages_url(channel_id: str) -> str:
    return f"{DISCORD_API_BASE}/channels/{channel_id}/messages"


async def _manage_session(session, func):
    """Run func with session, closing it afterward if we created it."""
    owns_session = session is None
    if owns_session:
        session = aiohttp.ClientSession()
    try:
        return await func(session)
    finally:
        if owns_session:
            await session.close()


async def _request_with_retry(session, method, url, headers, **kwargs):
    """Execute an HTTP request with Retry-After handling for 429 responses."""
    for attempt in range(MAX_RETRIES + 1):
        async with session.request(method, url, headers=headers, **kwargs) as resp:
            if resp.status == 429:
                if attempt >= MAX_RETRIES:
                    resp.raise_for_status()
                try:
                    retry_after = min(int(resp.headers.get("Retry-After", "1")), 300)
                except (ValueError, TypeError):
                    retry_after = 1
                await asyncio.sleep(retry_after)
                continue
            resp.raise_for_status()
            return await resp.json()
    msg = f"HTTP request to {url} failed after {MAX_RETRIES + 1} attempts"
    raise aiohttp.ClientResponseError(None, None, message=msg, status=429)


async def send_message(
    token: str,
    channel_id: str,
    text: str,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Send a text-only Discord message to a channel."""
    url = _messages_url(channel_id)
    payload = {"content": text}

    async def _do(s):
        return await _request_with_retry(s, "POST", url, _headers(token), json=payload)

    return await _manage_session(session, _do)


async def send_card(
    token: str,
    channel_id: str,
    text: str,
    card: dict,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Send a Discord message with an embed.

    Args:
        text: Message content text.
        card: Discord embed object (passed as first element of 'embeds' array).
    """
    embeds = card if isinstance(card, list) else [card]
    url = _messages_url(channel_id)
    payload = {"content": text, "embeds": embeds}

    async def _do(s):
        return await _request_with_retry(s, "POST", url, _headers(token), json=payload)

    return await _manage_session(session, _do)
