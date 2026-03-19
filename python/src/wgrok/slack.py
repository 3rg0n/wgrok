"""Thin Slack REST client for sending messages and cards (Block Kit)."""

from __future__ import annotations

import asyncio

import aiohttp

SLACK_API_BASE = "https://slack.com/api"
MAX_RETRIES = 3
SLACK_POST_MESSAGE_URL = f"{SLACK_API_BASE}/chat.postMessage"


def _headers(token: str) -> dict:
    return {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}


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
    channel: str,
    text: str,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Send a text-only Slack message to a channel or DM."""
    payload = {"channel": channel, "text": text}

    async def _do(s):
        return await _request_with_retry(s, "POST", SLACK_POST_MESSAGE_URL, _headers(token), json=payload)

    return await _manage_session(session, _do)


async def send_card(
    token: str,
    channel: str,
    text: str,
    card: dict,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Send a Slack message with Block Kit blocks.

    Args:
        text: Fallback text for notifications.
        card: Block Kit blocks list (passed as the 'blocks' field).
    """
    blocks = card if isinstance(card, list) else [card]
    payload = {"channel": channel, "text": text, "blocks": blocks}

    async def _do(s):
        return await _request_with_retry(s, "POST", SLACK_POST_MESSAGE_URL, _headers(token), json=payload)

    return await _manage_session(session, _do)
