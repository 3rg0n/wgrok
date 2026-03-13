"""WgrokReceiver - listens for response messages, matches slug, invokes handler callback."""

from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable

from webex_message_handler import WebexMessageHandler, WebexMessageHandlerConfig

from .allowlist import Allowlist
from .config import ReceiverConfig
from .logging import get_logger
from .protocol import parse_response


class WgrokReceiver:
    def __init__(self, config: ReceiverConfig, handler: Callable[[str, str], Awaitable[None]]) -> None:
        self._config = config
        self._allowlist = Allowlist(config.domains)
        self._handler_callback = handler
        self._logger = get_logger(config.debug, "wgrok.receiver")
        self._ws_handler: WebexMessageHandler | None = None
        self._stop_event: asyncio.Event = asyncio.Event()

    async def listen(self) -> None:
        """Connect to Webex and listen for response messages matching our slug."""
        wmh_config = WebexMessageHandlerConfig(token=self._config.webex_token, logger=self._logger)
        self._ws_handler = WebexMessageHandler(wmh_config)

        @self._ws_handler.on("message:created")
        async def on_message(message: dict) -> None:
            await self._on_message(message)

        self._logger.info(f"Receiver listening for slug: {self._config.slug}")
        await self._ws_handler.connect()
        self._logger.info("Receiver connected")
        await self._stop_event.wait()

    async def stop(self) -> None:
        """Disconnect from Webex."""
        self._stop_event.set()
        if self._ws_handler:
            await self._ws_handler.disconnect()
            self._ws_handler = None
        self._logger.info("Receiver stopped")

    async def _on_message(self, message) -> None:
        """Process an incoming message: check allowlist, parse response, match slug, call handler."""
        sender = message.person_email if hasattr(message, "person_email") else message.get("personEmail", "")
        raw_text = message.text if hasattr(message, "text") else message.get("text", "")
        text = (raw_text or "").strip()

        if not self._allowlist.is_allowed(sender):
            self._logger.warning(f"Rejected message from {sender}: not in allowlist")
            return

        try:
            slug, payload = parse_response(text)
        except ValueError:
            self._logger.debug(f"Ignoring unparseable message from {sender}")
            return

        if slug != self._config.slug:
            self._logger.debug(f"Ignoring message with slug {slug!r} (expected {self._config.slug!r})")
            return

        self._logger.info(f"Received payload for slug {slug!r} from {sender}")
        await self._handler_callback(slug, payload)
