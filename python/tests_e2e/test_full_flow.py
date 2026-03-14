"""E2E test: sender -> router bot -> receiver full round-trip.

Tests the full protocol without any real Webex API calls by simulating
the message flow through each component's internal handler.
"""

from unittest.mock import patch

from wgrok.router_bot import WgrokRouterBot
from wgrok.receiver import WgrokReceiver
from wgrok.sender import WgrokSender


class TestFullFlow:
    async def test_sender_to_router_bot_to_receiver(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Full text-only round-trip: sender -> router bot -> receiver."""
        received_payloads: list[tuple] = []

        async def on_receive(slug: str, payload: str, cards: list[dict]) -> None:
            received_payloads.append((slug, payload, cards))

        sender = WgrokSender(e2e_sender_config)
        bot = WgrokRouterBot(e2e_bot_config)
        receiver = WgrokReceiver(e2e_receiver_config, on_receive)

        sent_messages: list[dict] = []

        async def capture_send(token, to_email, text, session=None):
            sent_messages.append({"token": token, "to": to_email, "text": text})
            return {"id": f"msg-{len(sent_messages)}"}

        with (
            patch("wgrok.sender.send_message", side_effect=capture_send),
            patch("wgrok.router_bot.send_message", side_effect=capture_send),
            patch.object(bot, "_fetch_cards", return_value=[]),
            patch.object(receiver, "_fetch_cards", return_value=[]),
        ):
            await sender.send("hello")
            assert len(sent_messages) == 1
            assert sent_messages[0]["text"] == "./echo:e2e-slug:hello"
            assert sent_messages[0]["to"] == "routerbot@example.com"

            echo_input = {"personEmail": "user@example.com", "text": sent_messages[0]["text"], "id": "m1"}
            await bot._on_message(echo_input)
            assert len(sent_messages) == 2
            assert sent_messages[1]["text"] == "e2e-slug:hello"
            assert sent_messages[1]["to"] == "user@example.com"

            receiver_input = {"personEmail": "routerbot@example.com", "text": sent_messages[1]["text"], "id": "m2"}
            await receiver._on_message(receiver_input)
            assert len(received_payloads) == 1
            assert received_payloads[0] == ("e2e-slug", "hello", [])

        await sender.close()

    async def test_full_flow_with_card(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Full round-trip with an adaptive card attachment."""
        received: list[tuple] = []
        card = {"type": "AdaptiveCard", "body": [{"type": "TextBlock", "text": "Hello"}]}

        async def on_receive(slug: str, payload: str, cards: list[dict]) -> None:
            received.append((slug, payload, cards))

        sender = WgrokSender(e2e_sender_config)
        bot = WgrokRouterBot(e2e_bot_config)
        receiver = WgrokReceiver(e2e_receiver_config, on_receive)

        sent: list[dict] = []

        async def capture_msg(token, to_email, text, session=None):
            sent.append({"text": text, "to": to_email, "card": None})
            return {"id": "x"}

        async def capture_card(token, to_email, text, card_json, session=None):
            sent.append({"text": text, "to": to_email, "card": card_json})
            return {"id": "x"}

        with (
            patch("wgrok.sender.send_card", side_effect=capture_card),
            patch("wgrok.router_bot.send_card", side_effect=capture_card),
            patch.object(bot, "_fetch_cards", return_value=[card]),
            patch.object(receiver, "_fetch_cards", return_value=[card]),
        ):
            # Sender sends with card
            await sender.send("form-data", card=card)
            assert len(sent) == 1
            assert sent[0]["text"] == "./echo:e2e-slug:form-data"
            assert sent[0]["card"] == card

            # Echo bot relays with card
            await bot._on_message({"personEmail": "user@example.com", "text": sent[0]["text"], "id": "m1"})
            assert len(sent) == 2
            assert sent[1]["text"] == "e2e-slug:form-data"
            assert sent[1]["card"] == card

            # Receiver gets message + card
            await receiver._on_message({"personEmail": "bot@example.com", "text": sent[1]["text"], "id": "m2"})
            assert len(received) == 1
            assert received[0] == ("e2e-slug", "form-data", [card])

        await sender.close()

    async def test_full_flow_with_colons_in_payload(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Verify colons in the payload survive the full round-trip."""
        received: list[tuple] = []

        async def on_receive(slug: str, payload: str, cards: list[dict]) -> None:
            received.append((slug, payload, cards))

        sender = WgrokSender(e2e_sender_config)
        bot = WgrokRouterBot(e2e_bot_config)
        receiver = WgrokReceiver(e2e_receiver_config, on_receive)

        sent: list[dict] = []

        async def capture(token, to_email, text, session=None):
            sent.append({"text": text, "to": to_email})
            return {"id": "x"}

        with (
            patch("wgrok.sender.send_message", side_effect=capture),
            patch("wgrok.router_bot.send_message", side_effect=capture),
            patch.object(bot, "_fetch_cards", return_value=[]),
            patch.object(receiver, "_fetch_cards", return_value=[]),
        ):
            await sender.send("key:value:extra")

            await bot._on_message({"personEmail": "user@example.com", "text": sent[0]["text"], "id": "m1"})

            await receiver._on_message({"personEmail": "bot@example.com", "text": sent[1]["text"], "id": "m2"})

            assert received == [("e2e-slug", "key:value:extra", [])]

        await sender.close()

    async def test_disallowed_sender_blocks_flow(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Messages from disallowed senders are dropped at the router bot."""
        bot = WgrokRouterBot(e2e_bot_config)
        sent: list[dict] = []

        async def capture(token, to_email, text, session=None):
            sent.append({"text": text})
            return {"id": "x"}

        with patch("wgrok.router_bot.send_message", side_effect=capture):
            await bot._on_message({
                "personEmail": "hacker@evil.com",
                "text": "./echo:e2e-slug:pwned",
            })
            assert len(sent) == 0

    async def test_wrong_slug_ignored_by_receiver(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Receiver ignores messages with non-matching slugs."""
        received: list[tuple] = []
        receiver = WgrokReceiver(e2e_receiver_config, lambda s, p, c: received.append((s, p, c)))

        await receiver._on_message({
            "personEmail": "bot@example.com",
            "text": "wrong-slug:payload",
        })
        assert len(received) == 0
