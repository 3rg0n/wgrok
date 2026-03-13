"""Send a test message through wgrok."""

import asyncio
import sys
from pathlib import Path

from wgrok.config import SenderConfig
from wgrok.sender import WgrokSender

ENV_FILE = Path(__file__).parent / ".env.sender"


async def main() -> None:
    payload = " ".join(sys.argv[1:]) if len(sys.argv) > 1 else "hello from wgrok"
    config = SenderConfig.from_env(str(ENV_FILE))
    sender = WgrokSender(config)
    try:
        print(f"Sending to {config.target} with slug '{config.slug}': {payload}")
        result = await sender.send(payload)
        print(f"Sent OK: {result.get('id', 'unknown')}")
    finally:
        await sender.close()


if __name__ == "__main__":
    asyncio.run(main())
