# wgrok

An ngrok-style tunnel that uses the Webex API as a message bus. Enables agents, services, and registered endpoints to send and receive messages between systems even when firewalls or network rules prevent direct webhooks.

## How it works

```
Sender                    Echo Bot                   Receiver
  │                          │                          │
  │  ./echo:myslug:payload   │                          │
  ├─────────────────────────►│                          │
  │     (Webex message)      │   myslug:payload         │
  │                          ├─────────────────────────►│
  │                          │   (Webex message back     │
  │                          │    to sender's email)     │
```

1. **Sender** wraps your payload as `./echo:{slug}:{payload}` and sends it to the echo bot via Webex API.
2. **Echo bot** listens on a WebSocket (via [webex-message-handler](https://github.com/3rg0n/webex-message-handler)), validates the sender against an allowlist, strips the `./echo:` prefix, and replies to the sender.
3. **Receiver** listens on the same WebSocket, checks the allowlist, matches the slug, and delivers the payload to your handler.

The `{slug}` acts as a message bus tag — agents and services only act on messages matching their configured slug.

## Languages

Implemented in four languages with identical behavior and shared test cases:

| Language | Directory | Package |
|----------|-----------|---------|
| Python | `python/` | `wgrok` |
| Go | `go/` | `github.com/3rg0n/wgrok/go` |
| TypeScript | `ts/` | `wgrok` |
| Rust | `rust/` | `wgrok` |

## Quick start

### 1. Register a Webex bot

Go to [developer.webex.com](https://developer.webex.com) and create a bot. You'll need two tokens:
- One for the **echo bot** (the relay service)
- One **shared token** for senders and receivers

### 2. Configure environment

```bash
cp python/.env.example python/.env   # or go/, ts/, rust/
```

```env
# Sender / Receiver
WGROK_TOKEN=<shared webex token>
WGROK_TARGET=echobot@webex.bot
WGROK_SLUG=myagent
WGROK_DOMAINS=example.com

# Echo bot (separate .env)
WGROK_TOKEN=<echo bot token>
WGROK_DOMAINS=example.com

# Optional
WGROK_DEBUG=true
```

### 3. Use in your project

**Python**
```python
from wgrok import WgrokSender, WgrokReceiver, SenderConfig, ReceiverConfig

# Send
sender = WgrokSender(SenderConfig.from_env())
await sender.send("hello world")
await sender.close()

# Receive
async def handler(slug, payload, cards):
    print(f"Got: {payload}")

receiver = WgrokReceiver(ReceiverConfig.from_env(), handler)
await receiver.listen()
```

**Go**
```go
import wgrok "github.com/3rg0n/wgrok/go"

// Send
cfg, _ := wgrok.SenderConfigFromEnv()
sender := wgrok.NewSender(cfg)
sender.Send("hello world", nil)

// Receive
rcfg, _ := wgrok.ReceiverConfigFromEnv()
receiver := wgrok.NewReceiver(rcfg, func(slug, payload string, cards []interface{}) {
    fmt.Printf("Got: %s\n", payload)
})
receiver.Listen(ctx)
```

**TypeScript**
```typescript
import { WgrokSender, WgrokReceiver, senderConfigFromEnv, receiverConfigFromEnv } from 'wgrok';

// Send
const sender = new WgrokSender(senderConfigFromEnv());
await sender.send('hello world');

// Receive
const receiver = new WgrokReceiver(receiverConfigFromEnv(), (slug, payload, cards) => {
  console.log(`Got: ${payload}`);
});
await receiver.listen();
```

**Rust**
```rust
use wgrok::{WgrokSender, WgrokReceiver, SenderConfig, ReceiverConfig};

// Send
let cfg = SenderConfig::from_env()?;
let sender = WgrokSender::new(cfg);
sender.send("hello world", None).await?;

// Receive
let cfg = ReceiverConfig::from_env()?;
let receiver = WgrokReceiver::new(cfg, Box::new(|slug, payload, cards| {
    println!("Got: {payload}");
}));
receiver.listen(shutdown_rx).await?;
```

## Message protocol (v1.0)

```
Sending:   ./echo:{slug}:{payload}
Receiving: {slug}:{payload}
```

- `{slug}` — routing tag that receivers match against
- `{payload}` — arbitrary content (commands, JSON, OTEL traces, webhooks, etc.)
- Adaptive cards are supported as optional attachments alongside the text payload

## Allowlist

The `WGROK_DOMAINS` environment variable controls who can send messages through the system:

| Pattern | Matches |
|---------|---------|
| `example.com` | `*@example.com` |
| `*@example.com` | Any user at example.com |
| `user@example.com` | Exact match only |

## Build and test

### Python
```bash
cd python
pip install -e ".[dev]"
ruff check src/ tests/
pytest tests/ -v
```

### Go
```bash
cd go
go test ./... -v
go run ./cmd/sender <payload>
go run ./cmd/echobot
go run ./cmd/receiver
```

### TypeScript
```bash
cd ts
npm install
npx tsc --noEmit
npm test
```

### Rust
```bash
cd rust
cargo build
cargo test
cargo clippy
```

## Testing architecture

Test cases are defined once as JSON in `tests/` and consumed by all four languages via thin shims:

```
tests/
├── protocol_cases.json
├── allowlist_cases.json
├── config_cases.json
├── webex_cases.json
├── sender_cases.json
├── echo_bot_cases.json
└── receiver_cases.json
```

Fix a bug in a test case, it applies to all languages. Add a new case, all languages must pass it.

Each language also has HTTP-level tests for the Webex client and NDJSON logging tests that are inherently language-specific.

## Project structure

```
wgrok/
├── python/           # Python SDK
│   ├── src/wgrok/    # Source modules
│   └── tests/        # Test shims
├── go/               # Go SDK
│   ├── cmd/          # Runner commands
│   └── *_test.go     # Test shims
├── ts/               # TypeScript SDK
│   ├── src/          # Source modules
│   └── tests/        # Test shims
├── rust/             # Rust SDK
│   ├── src/          # Source modules
│   └── tests/        # Test shims
├── tests/            # Shared JSON test cases
├── .plan/            # Design docs
├── CLAUDE.md         # AI assistant instructions
└── README.md
```

## License

MIT

## Contributing

Contributions welcome. All four language implementations must maintain feature parity — if you add a feature to one language, add it to all four with shared test cases in `tests/`.
