"""WgrokSender - wraps payload in echo protocol and sends via configured platform."""

from __future__ import annotations

import aiohttp

from .config import SenderConfig
from .logging import get_logger
from .platform import platform_send_card, platform_send_message
from .protocol import format_echo


class WgrokSender:
    def __init__(self, config: SenderConfig) -> None:
        self._config = config
        self._session: aiohttp.ClientSession | None = None
        self._logger = get_logger(config.debug, "wgrok.sender")

    async def _ensure_session(self) -> aiohttp.ClientSession:
        if self._session is None:
            self._session = aiohttp.ClientSession()
        return self._session

    async def send(self, payload: str, card: dict | None = None) -> dict:
        """Format payload as echo message and send to the configured target.

        Args:
            payload: Text payload to send.
            card: Optional adaptive card JSON to attach.
        """
        session = await self._ensure_session()
        text = format_echo(self._config.slug, payload)
        platform = self._config.platform
        token = self._config.webex_token
        target = self._config.target
        self._logger.info(f"Sending to {target} via {platform}: {text}")
        if card is not None:
            self._logger.info("Including card/rich content attachment")
            return await platform_send_card(platform, token, target, text, card, session)
        return await platform_send_message(platform, token, target, text, session)

    async def close(self) -> None:
        """Clean up the HTTP session."""
        if self._session:
            await self._session.close()
            self._session = None
