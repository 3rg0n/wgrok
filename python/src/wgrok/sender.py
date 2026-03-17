"""WgrokSender - wraps payload in echo protocol and sends via configured platform."""

from __future__ import annotations

import aiohttp

from . import codec
from .config import SenderConfig
from .logging import get_logger
from .platform import platform_send_card, platform_send_message
from .protocol import ECHO_PREFIX, format_echo, format_flags

PLATFORM_LIMITS = {
    "webex": 7439,
    "slack": 4000,
    "discord": 2000,
    "irc": 400,
}


class WgrokSender:
    def __init__(self, config: SenderConfig) -> None:
        self._config = config
        self._session: aiohttp.ClientSession | None = None
        self._logger = get_logger(config.debug, "wgrok.sender")

    async def _ensure_session(self) -> aiohttp.ClientSession:
        if self._session is None:
            self._session = aiohttp.ClientSession()
        return self._session

    async def send(
        self,
        payload: str,
        card: dict | None = None,
        compress: bool = False,
        from_slug: str | None = None,
    ) -> dict | list[dict]:
        """Format payload as echo message and send to the configured target.

        Args:
            payload: Text payload to send.
            card: Optional adaptive card JSON to attach.
            compress: If True, gzip+base64 encode the payload.
            from_slug: Sender identifier (defaults to config slug).
        """
        session = await self._ensure_session()
        from_slug = from_slug or self._config.slug

        if compress:
            payload = codec.compress(payload)

        flags = format_flags(compress)
        text = format_echo(self._config.slug, from_slug, flags, payload)
        platform = self._config.platform
        token = self._config.webex_token
        target = self._config.target
        limit = PLATFORM_LIMITS.get(platform, 7439)

        if len(text.encode("utf-8")) > limit and card is None:
            # Auto-chunk: calculate overhead and split payload
            overhead = (
                len(ECHO_PREFIX.encode("utf-8"))
                + len(self._config.slug.encode("utf-8"))
                + len(from_slug.encode("utf-8"))
                + 3  # +3 for three colons in v2 format
            )
            max_payload = limit - overhead
            chunks = codec.chunk(payload, max_payload)
            self._logger.info(
                f"Payload exceeds {limit}B limit, sending {len(chunks)} chunks to {target} via {platform}"
            )
            results = []
            for i, ch in enumerate(chunks):
                chunk_flags = format_flags(compress, i + 1, len(chunks))
                chunk_text = format_echo(self._config.slug, from_slug, chunk_flags, ch)
                results.append(await platform_send_message(platform, token, target, chunk_text, session))
            return results

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
