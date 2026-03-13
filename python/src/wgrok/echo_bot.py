"""WgrokEchoBot - listens for echo messages, validates allowlist, strips prefix, relays back."""

from __future__ import annotations

import aiohttp
from webex_message_handler import WebexMessageHandler

from .allowlist import Allowlist
from .config import BotConfig
from .logging import get_logger
from .protocol import format_response, is_echo, parse_echo
from .webex import send_message


class WgrokEchoBot:
    def __init__(self, config: BotConfig) -> None:
        self._config = config
        self._allowlist = Allowlist(config.domains)
        self._logger = get_logger(config.debug, "wgrok.echo_bot")
        self._handler: WebexMessageHandler | None = None
        self._session: aiohttp.ClientSession | None = None

    async def run(self) -> None:
        """Connect to Webex and listen for echo messages."""
        self._session = aiohttp.ClientSession()
        self._handler = WebexMessageHandler(self._config.webex_token, logger=self._logger)

        @self._handler.on("message:created")
        async def on_message(message: dict) -> None:
            await self._on_message(message)

        self._logger.info("Echo bot starting")
        await self._handler.listen()

    async def stop(self) -> None:
        """Disconnect from Webex and clean up."""
        if self._handler:
            await self._handler.close()
            self._handler = None
        if self._session:
            await self._session.close()
            self._session = None
        self._logger.info("Echo bot stopped")

    async def _on_message(self, message: dict) -> None:
        """Process an incoming message: check allowlist, parse echo, relay response."""
        sender = message.get("personEmail", "")
        text = message.get("text", "").strip()

        if not self._allowlist.is_allowed(sender):
            self._logger.warning(f"Rejected message from {sender}: not in allowlist")
            return

        if not is_echo(text):
            self._logger.debug(f"Ignoring non-echo message from {sender}")
            return

        try:
            slug, payload = parse_echo(text)
        except ValueError as e:
            self._logger.error(f"Failed to parse echo message: {e}")
            return

        response = format_response(slug, payload)
        self._logger.info(f"Relaying to {sender}: {response}")
        await send_message(self._config.webex_token, sender, response, self._session)
