"""Run the wgrok receiver, listening for messages matching our slug."""

import asyncio
from pathlib import Path

from wgrok.config import ReceiverConfig
from wgrok.receiver import WgrokReceiver

ENV_FILE = Path(__file__).parent / ".env.receiver"


async def on_message(slug: str, payload: str) -> None:
    print(f"[RECEIVED] slug={slug!r} payload={payload!r}")


async def main() -> None:
    config = ReceiverConfig.from_env(str(ENV_FILE))
    receiver = WgrokReceiver(config, on_message)
    print(f"Receiver listening for slug '{config.slug}' (domains: {config.domains})")
    try:
        await receiver.listen()
    except KeyboardInterrupt:
        pass
    finally:
        await receiver.stop()
        print("Receiver stopped")


if __name__ == "__main__":
    asyncio.run(main())
