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

        async def on_receive(slug: str, payload: str, cards: list[dict], from_slug: str) -> None:
            received_payloads.append((slug, payload, cards, from_slug))

        sender = WgrokSender(e2e_sender_config)
        bot = WgrokRouterBot(e2e_bot_config)
        receiver = WgrokReceiver(e2e_receiver_config, on_receive)

        sent_messages: list[dict] = []

        async def capture_send(platform, token, to_email, text, session=None):
            sent_messages.append({"platform": platform, "token": token, "to": to_email, "text": text})
            return {"id": f"msg-{len(sent_messages)}"}

        with (
            patch("wgrok.sender.platform_send_message", side_effect=capture_send),
            patch("wgrok.router_bot.platform_send_message", side_effect=capture_send),
            patch.object(bot, "_fetch_cards", return_value=[]),
            patch.object(receiver, "_fetch_cards", return_value=[]),
        ):
            await sender.send("hello")
            assert len(sent_messages) == 1
            assert sent_messages[0]["text"] == "./echo:e2e-slug:e2e-slug:-:hello"
            assert sent_messages[0]["to"] == "routerbot@example.com"

            from wgrok.listener import IncomingMessage
            echo_input = IncomingMessage(
                sender="user@example.com",
                text=sent_messages[0]["text"],
                msg_id="m1",
                platform="webex",
                cards=[]
            )
            await bot._on_incoming(echo_input)
            assert len(sent_messages) == 2
            assert sent_messages[1]["text"] == "e2e-slug:e2e-slug:-:hello"
            assert sent_messages[1]["to"] == "user@example.com"

            receiver_input = IncomingMessage(
                sender="routerbot@example.com",
                text=sent_messages[1]["text"],
                msg_id="m2",
                platform="webex",
                cards=[]
            )
            await receiver._on_incoming(receiver_input)
            assert len(received_payloads) == 1
            assert received_payloads[0] == ("e2e-slug", "hello", [], "e2e-slug")

        await sender.close()

    async def test_full_flow_with_card(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Full round-trip with an adaptive card attachment."""
        received: list[tuple] = []
        card = {"type": "AdaptiveCard", "body": [{"type": "TextBlock", "text": "Hello"}]}

        async def on_receive(slug: str, payload: str, cards: list[dict], from_slug: str) -> None:
            received.append((slug, payload, cards, from_slug))

        sender = WgrokSender(e2e_sender_config)
        bot = WgrokRouterBot(e2e_bot_config)
        receiver = WgrokReceiver(e2e_receiver_config, on_receive)

        sent: list[dict] = []

        async def capture_card(platform, token, to_email, text, card_json, session=None):
            sent.append({"text": text, "to": to_email, "card": card_json})
            return {"id": "x"}

        with (
            patch("wgrok.sender.platform_send_card", side_effect=capture_card),
            patch("wgrok.router_bot.platform_send_card", side_effect=capture_card),
            patch.object(bot, "_fetch_cards", return_value=[card]),
            patch.object(receiver, "_fetch_cards", return_value=[card]),
        ):
            # Sender sends with card
            await sender.send("form-data", card=card)
            assert len(sent) == 1
            assert sent[0]["text"] == "./echo:e2e-slug:e2e-slug:-:form-data"
            assert sent[0]["card"] == card

            # Echo bot relays with card
            from wgrok.listener import IncomingMessage
            await bot._on_incoming(IncomingMessage(
                sender="user@example.com",
                text=sent[0]["text"],
                msg_id="m1",
                platform="webex",
                cards=[]
            ))
            assert len(sent) == 2
            assert sent[1]["text"] == "e2e-slug:e2e-slug:-:form-data"
            assert sent[1]["card"] == card

            # Receiver gets message + card
            await receiver._on_incoming(IncomingMessage(
                sender="bot@example.com",
                text=sent[1]["text"],
                msg_id="m2",
                platform="webex",
                cards=[card]
            ))
            assert len(received) == 1
            assert received[0] == ("e2e-slug", "form-data", [card], "e2e-slug")

        await sender.close()

    async def test_full_flow_with_colons_in_payload(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Verify colons in the payload survive the full round-trip."""
        received: list[tuple] = []

        async def on_receive(slug: str, payload: str, cards: list[dict], from_slug: str) -> None:
            received.append((slug, payload, cards, from_slug))

        sender = WgrokSender(e2e_sender_config)
        bot = WgrokRouterBot(e2e_bot_config)
        receiver = WgrokReceiver(e2e_receiver_config, on_receive)

        sent: list[dict] = []

        async def capture(platform, token, to_email, text, session=None):
            sent.append({"text": text, "to": to_email})
            return {"id": "x"}

        with (
            patch("wgrok.sender.platform_send_message", side_effect=capture),
            patch("wgrok.router_bot.platform_send_message", side_effect=capture),
            patch.object(bot, "_fetch_cards", return_value=[]),
            patch.object(receiver, "_fetch_cards", return_value=[]),
        ):
            await sender.send("key:value:extra")

            from wgrok.listener import IncomingMessage
            await bot._on_incoming(IncomingMessage(
                sender="user@example.com",
                text=sent[0]["text"],
                msg_id="m1",
                platform="webex",
                cards=[]
            ))

            await receiver._on_incoming(IncomingMessage(
                sender="bot@example.com",
                text=sent[1]["text"],
                msg_id="m2",
                platform="webex",
                cards=[]
            ))

            assert received == [("e2e-slug", "key:value:extra", [], "e2e-slug")]

        await sender.close()

    async def test_disallowed_sender_blocks_flow(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Messages from disallowed senders are dropped at the router bot."""
        bot = WgrokRouterBot(e2e_bot_config)
        sent: list[dict] = []

        async def capture(platform, token, to_email, text, session=None):
            sent.append({"text": text})
            return {"id": "x"}

        with patch("wgrok.router_bot.platform_send_message", side_effect=capture):
            from wgrok.listener import IncomingMessage
            await bot._on_incoming(IncomingMessage(
                sender="hacker@evil.com",
                text="./echo:e2e-slug:relay:-:pwned",
                msg_id="m1",
                platform="webex",
                cards=[]
            ))
            assert len(sent) == 0

    async def test_wrong_slug_ignored_by_receiver(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Receiver ignores messages with non-matching slugs."""
        received: list[tuple] = []
        receiver = WgrokReceiver(e2e_receiver_config, lambda s, p, c, f: received.append((s, p, c, f)))

        from wgrok.listener import IncomingMessage
        await receiver._on_incoming(IncomingMessage(
            sender="bot@example.com",
            text="wrong-slug:relay:-:payload",
            msg_id="m1",
            platform="webex",
            cards=[]
        ))
        assert len(received) == 0
