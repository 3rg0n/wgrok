"""Send a test message through wgrok.

Usage:
    python run_sender.py hello world          # text-only
    python run_sender.py --card card.json     # text + adaptive card from file
"""

import asyncio
import json
import sys
from pathlib import Path

from wgrok.config import SenderConfig
from wgrok.sender import WgrokSender

ENV_FILE = Path(__file__).parent / ".env.sender"


async def main() -> None:
    args = sys.argv[1:]
    card = None

    # Parse --card flag
    if "--card" in args:
        idx = args.index("--card")
        card_path = Path(args[idx + 1])
        card = json.loads(card_path.read_text())
        args = args[:idx] + args[idx + 2 :]

    payload = " ".join(args) if args else "hello from wgrok"
    config = SenderConfig.from_env(str(ENV_FILE))
    sender = WgrokSender(config)
    try:
        print(f"Sending to {config.target} with slug '{config.slug}': {payload}")
        if card:
            print(f"  with adaptive card ({len(json.dumps(card))} bytes)")
        result = await sender.send(payload, card=card)
        print(f"Sent OK: {result.get('id', 'unknown')}")
    finally:
        await sender.close()


if __name__ == "__main__":
    asyncio.run(main())
