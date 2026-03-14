"""WgrokRouterBot - listens for messages, validates allowlist, strips prefix, relays back.

Supports Mode B (echo back to sender) and Mode C (route to registered agent).
When WGROK_ROUTES is configured, registered slugs are routed to their target bot.
Unregistered slugs fall back to Mode B (echo back to sender).

Optionally exposes a webhook endpoint when WGROK_WEBHOOK_PORT is configured.
"""

from __future__ import annotations

import asyncio
from typing import Any

import aiohttp
from webex_message_handler import WebexMessageHandler, WebexMessageHandlerConfig

from .allowlist import Allowlist
from .config import BotConfig
from .logging import get_logger
from .protocol import format_response, is_echo, parse_echo
from .webex import extract_cards, get_message, send_card, send_message


class WgrokRouterBot:
    def __init__(self, config: BotConfig) -> None:
        self._config = config
        self._allowlist = Allowlist(config.domains)
        self._routes = config.routes
        self._logger = get_logger(config.debug, "wgrok.router_bot")
        self._handler: WebexMessageHandler | None = None
        self._session: aiohttp.ClientSession | None = None
        self._stop_event: asyncio.Event = asyncio.Event()
        self._webhook_runner: Any = None

    async def run(self) -> None:
        """Connect to Webex and listen for echo messages. Optionally start webhook endpoint."""
        self._session = aiohttp.ClientSession()
        wmh_config = WebexMessageHandlerConfig(token=self._config.webex_token, logger=self._logger)
        self._handler = WebexMessageHandler(wmh_config)

        @self._handler.on("message:created")
        async def on_message(message) -> None:
            await self._on_message(message)

        self._logger.info("Router bot starting")

        if self._config.webhook_port is not None:
            await self._start_webhook()

        await self._handler.connect()
        self._logger.info("Router bot connected")
        await self._stop_event.wait()

    async def stop(self) -> None:
        """Disconnect from Webex and clean up."""
        self._stop_event.set()
        if self._webhook_runner:
            await self._webhook_runner.cleanup()
            self._webhook_runner = None
        if self._handler:
            await self._handler.disconnect()
            self._handler = None
        if self._session:
            await self._session.close()
            self._session = None
        self._logger.info("Router bot stopped")

    async def _start_webhook(self) -> None:
        """Start the HTTP webhook endpoint."""
        from aiohttp import web

        app = web.Application()
        app.router.add_post("/wgrok", self._handle_webhook)
        runner = web.AppRunner(app)
        await runner.setup()
        site = web.TCPSite(runner, "0.0.0.0", self._config.webhook_port)
        await site.start()
        self._webhook_runner = runner
        self._logger.info(f"Webhook endpoint listening on port {self._config.webhook_port}")

    async def _handle_webhook(self, request) -> Any:
        """Handle an inbound webhook POST request."""
        from aiohttp import web

        # Authenticate
        if self._config.webhook_secret:
            auth = request.headers.get("Authorization", "")
            expected = f"Bearer {self._config.webhook_secret}"
            if auth != expected:
                self._logger.warning("Webhook request rejected: invalid authorization")
                return web.json_response({"error": "unauthorized"}, status=401)

        try:
            body = await request.json()
        except Exception:
            return web.json_response({"error": "invalid json"}, status=400)

        text = body.get("text", "").strip()
        sender = body.get("from", "")

        if not text or not sender:
            return web.json_response({"error": "missing text or from"}, status=400)

        # Process through the same pipeline as WebSocket messages
        msg = {"text": text, "personEmail": sender, "id": ""}
        await self._on_message(msg)
        return web.json_response({"status": "ok"})

    def _resolve_target(self, slug: str, sender: str) -> str:
        """Resolve where to send the response. Mode C: registry lookup. Mode B: echo back to sender."""
        if slug in self._routes:
            target = self._routes[slug]
            self._logger.info(f"Route resolved: {slug!r} -> {target}")
            return target
        return sender

    async def _on_message(self, message) -> None:
        """Process an incoming message: check allowlist, parse echo, relay response."""
        sender = message.person_email if hasattr(message, "person_email") else message.get("personEmail", "")
        msg_id = message.id if hasattr(message, "id") else message.get("id", "")
        raw_text = message.text if hasattr(message, "text") else message.get("text", "")
        text = (raw_text or "").strip()

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
        target = self._resolve_target(slug, sender)

        # Check for card attachments on the original message
        cards = await self._fetch_cards(msg_id)

        if cards:
            self._logger.info(f"Relaying to {target}: {response} (with {len(cards)} card(s))")
            await send_card(self._config.webex_token, target, response, cards[0], self._session)
        else:
            self._logger.info(f"Relaying to {target}: {response}")
            await send_message(self._config.webex_token, target, response, self._session)

    async def _fetch_cards(self, message_id: str) -> list[dict]:
        """Fetch card attachments from the original message via REST API."""
        if not message_id:
            return []
        try:
            full_msg = await get_message(self._config.webex_token, message_id, self._session)
            return extract_cards(full_msg)
        except Exception as e:
            self._logger.debug(f"Could not fetch message attachments: {e}")
            return []
