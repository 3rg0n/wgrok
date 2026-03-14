# wgrok

A message bus protocol over social messaging platforms. Uses platform APIs (Webex, Slack, Discord) as transport to allow agents, services, and orchestrators to communicate across firewalls without inbound webhooks.

## Protocol

wgrok defines a layered message format:

### Layer 0 — Direct messaging

```
{slug}:{payload}
```

Pure pub/sub. Agents listen for messages matching their slug, like NATS subjects. No bot routing needed.

### Layer 1 — Verb routing

```
./{verb}:{slug}:{payload}
```

Messages sent to a bot that interprets the verb and acts on it. The `./` prefix signals "this is a command". The bot strips the verb after processing and delivers `{slug}:{payload}` to the destination.

### Built-in verbs

| Verb | Behavior | Status |
|------|----------|--------|
| `echo` | Reflect message back to sender | Implemented |
| `route` | Forward to internal system by slug | Reserved |
| `broadcast` | Fan out to all matching subscribers | Reserved |
| `store` | Persist payload for later retrieval | Reserved |

### How echo works

```
Sender                      Bot                        Receiver
  │                          │                          │
  │  ./echo:myslug:payload   │                          │
  ├─────────────────────────►│                          │
  │     (REST API POST)      │   myslug:payload         │
  │                          ├─────────────────────────►│
  │                          │     (WebSocket)          │
  │                          │                          │
  │                    Validates allowlist               │
  │                    Strips ./echo:                    │
  │                    Replies to sender                 │
```

1. **Sender** wraps payload as `./echo:{slug}:{payload}` and POSTs to the bot.
2. **Bot** validates allowlist, strips `./echo:`, replies with `{slug}:{payload}`.
3. **Receiver** listens on WebSocket, matches slug, delivers payload to your handler.

## Use cases

**Firewall traversal** — GitHub Actions orchestrator talks to on-prem orchestrator. Both share a token, echo bot relays through Webex. No inbound ports needed.

**Multi-agent pub/sub** — Multiple agents on one account, each with a unique slug. Agents ignore messages not matching their slug. NATS-style message bus over a chat platform.

**Orchestrator routing** — Bot receives `./route:{slug}:{payload}` and forwards to an internal system based on slug mapping. (Future — verb `route`.)

## Languages

Implemented in four languages with identical behavior and shared test cases:

| Language | Directory | Package |
|----------|-----------|---------|
| Python | `python/` | `wgrok` |
| Go | `go/` | `github.com/3rg0n/wgrok/go` |
| TypeScript | `ts/` | `wgrok` |
| Rust | `rust/` | `wgrok` |

## Quick start

### 1. Register a bot

Go to [developer.webex.com](https://developer.webex.com) and create a bot. You need two tokens:
- One for the **bot** (the relay service)
- One **shared token** for senders and receivers

### 2. Configure environment

```bash
cp python/.env.example python/.env   # or go/, ts/, rust/
```

```env
# Sender / Receiver
WGROK_TOKEN=<shared token>
WGROK_TARGET=bot@webex.bot
WGROK_SLUG=myagent
WGROK_DOMAINS=example.com

# Bot (separate .env)
WGROK_TOKEN=<bot token>
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

## Allowlist

The `WGROK_DOMAINS` environment variable controls who can send messages through the system:

| Pattern | Matches |
|---------|---------|
| `example.com` | `*@example.com` |
| `*@example.com` | Any user at example.com |
| `user@example.com` | Exact match only |

## Transport bindings

The protocol is transport-agnostic. Any platform with a REST API for sending and a persistent connection for receiving works:

| Platform | Send | Receive | Status |
|----------|------|---------|--------|
| Webex | REST `/v1/messages` | Mercury WebSocket | Implemented |
| Slack | `chat.postMessage` | Socket Mode WebSocket | Planned |
| Discord | REST `/channels/{id}/messages` | Gateway WebSocket | Planned |

## Specification

The formal protocol specification is in [`asyncapi.yaml`](asyncapi.yaml) (AsyncAPI 3.0.0).

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

### Cross-language test report
```bash
bash tests/run_all.sh
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

Fix a bug in a test case — it applies to all languages. Add a new case — all languages must pass it.

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
├── asyncapi.yaml     # Protocol specification
├── .plan/            # Design docs
└── README.md
```

## License

MIT

## Contributing

Contributions welcome. All four language implementations must maintain feature parity — if you add a feature to one language, add it to all four with shared test cases in `tests/`.
