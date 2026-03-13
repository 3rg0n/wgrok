"""E2E test: sender -> echo bot -> receiver full round-trip.

Tests the full protocol without any real Webex API calls by simulating
the message flow through each component's internal handler.
"""

from unittest.mock import patch

from wgrok.echo_bot import WgrokEchoBot
from wgrok.receiver import WgrokReceiver
from wgrok.sender import WgrokSender


class TestFullFlow:
    async def test_sender_to_echo_bot_to_receiver(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Simulate the full message flow:
        1. Sender formats ./echo:e2e-slug:hello and "sends" it
        2. Echo bot receives, validates, strips prefix, "replies" with e2e-slug:hello
        3. Receiver gets e2e-slug:hello, matches slug, invokes handler
        """
        received_payloads: list[tuple[str, str]] = []

        async def on_receive(slug: str, payload: str) -> None:
            received_payloads.append((slug, payload))

        sender = WgrokSender(e2e_sender_config)
        bot = WgrokEchoBot(e2e_bot_config)
        receiver = WgrokReceiver(e2e_receiver_config, on_receive)

        sent_messages: list[dict] = []

        async def capture_send(token, to_email, text, session=None):
            sent_messages.append({"token": token, "to": to_email, "text": text})
            return {"id": f"msg-{len(sent_messages)}"}

        with (
            patch("wgrok.sender.send_message", side_effect=capture_send),
            patch("wgrok.echo_bot.send_message", side_effect=capture_send),
        ):
            # Step 1: Sender sends
            await sender.send("hello")
            assert len(sent_messages) == 1
            assert sent_messages[0]["text"] == "./echo:e2e-slug:hello"
            assert sent_messages[0]["to"] == "echobot@example.com"

            # Step 2: Echo bot processes the message (simulating Webex delivery)
            echo_input = {
                "personEmail": "user@example.com",
                "text": sent_messages[0]["text"],
            }
            await bot._on_message(echo_input)
            assert len(sent_messages) == 2
            assert sent_messages[1]["text"] == "e2e-slug:hello"
            assert sent_messages[1]["to"] == "user@example.com"

            # Step 3: Receiver processes the echo bot's reply
            receiver_input = {
                "personEmail": "echobot@example.com",
                "text": sent_messages[1]["text"],
            }
            await receiver._on_message(receiver_input)
            assert len(received_payloads) == 1
            assert received_payloads[0] == ("e2e-slug", "hello")

        await sender.close()

    async def test_full_flow_with_colons_in_payload(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Verify colons in the payload survive the full round-trip."""
        received: list[tuple[str, str]] = []

        async def on_receive(slug: str, payload: str) -> None:
            received.append((slug, payload))

        sender = WgrokSender(e2e_sender_config)
        bot = WgrokEchoBot(e2e_bot_config)
        receiver = WgrokReceiver(e2e_receiver_config, on_receive)

        sent: list[dict] = []

        async def capture(token, to_email, text, session=None):
            sent.append({"text": text, "to": to_email})
            return {"id": "x"}

        with (
            patch("wgrok.sender.send_message", side_effect=capture),
            patch("wgrok.echo_bot.send_message", side_effect=capture),
        ):
            await sender.send("key:value:extra")

            await bot._on_message({
                "personEmail": "user@example.com",
                "text": sent[0]["text"],
            })

            await receiver._on_message({
                "personEmail": "bot@example.com",
                "text": sent[1]["text"],
            })

            assert received == [("e2e-slug", "key:value:extra")]

        await sender.close()

    async def test_disallowed_sender_blocks_flow(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Messages from disallowed senders are dropped at the echo bot."""
        bot = WgrokEchoBot(e2e_bot_config)
        sent: list[dict] = []

        async def capture(token, to_email, text, session=None):
            sent.append({"text": text})
            return {"id": "x"}

        with patch("wgrok.echo_bot.send_message", side_effect=capture):
            await bot._on_message({
                "personEmail": "hacker@evil.com",
                "text": "./echo:e2e-slug:pwned",
            })
            assert len(sent) == 0

    async def test_wrong_slug_ignored_by_receiver(
        self, e2e_sender_config, e2e_bot_config, e2e_receiver_config
    ):
        """Receiver ignores messages with non-matching slugs."""
        received: list[tuple[str, str]] = []
        receiver = WgrokReceiver(e2e_receiver_config, lambda s, p: received.append((s, p)))

        await receiver._on_message({
            "personEmail": "bot@example.com",
            "text": "wrong-slug:payload",
        })
        assert len(received) == 0
