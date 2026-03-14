# wgrok

A message bus protocol over social messaging platforms. Uses platform APIs (Webex, Slack, Discord) as transport to allow agents, services, and orchestrators to communicate across firewalls without inbound webhooks.

## Protocol

All wgrok messages are colon-delimited text prefixed with `./`:

```
./{context}:{...rest}
```

The `./` prefix signals "this is a wgrok command". What follows depends on the deployment mode.

### Mode A — Platform Bot

```
./{app}:{payload}
```

A central bot acts as a gateway for a large engineering organization. Developers don't need to know how to get SSL certs, create Webex bots, submit IT approvals, or configure LDAP. They use one SDK that talks to one bot.

```
Developer ──./jira:create ticket──► Platform Bot ──► Jira webhook (internal)
Developer ◄──./jira:PROJ-456 created────────────── Platform Bot
```

The `{app}` identifier maps to a registered backend service. The bot routes the payload to the app's internal webhook and relays responses back.

### Mode B — Agent Bus

```
./{verb}:{slug}:{payload}
```

No central routing bot. Multiple agents share the same messaging token — all see all messages. The verb tells a relay agent what to do, and the slug identifies which agent should process the result.

```
Sender ──./echo:deploy-agent:start deploy──► Echo Bot
                                                │
Receiver ◄──deploy-agent:start deploy────────── Echo Bot
(only processes because slug matches "deploy-agent")
```

The verb (`echo`) tells the relay agent to reflect. The slug (`deploy-agent`) is how agents filter: "is this message for me?"

### Layer 0 — Base format

```
{slug}:{payload}
```

After bot processing, the `./` prefix is stripped. Receivers always consume this format regardless of which mode produced it. Also used for direct agent-to-agent messaging without any bot.

### Built-in verbs (Mode B)

| Verb | Behavior | Status |
|------|----------|--------|
| `echo` | Reflect message back to sender | Implemented |
| `route` | Forward to internal system by slug | Reserved |
| `broadcast` | Fan out to all matching subscribers | Reserved |
| `store` | Persist payload for later retrieval | Reserved |

## Use cases

**Developer platform** — 76,000 engineers, one bot, every internal API behind it. `./jira:...`, `./deploy:...`, `./grafana:...` — developers integrate with one SDK instead of navigating SSL, LDAP, Webex bot creation, and IT security approvals for each service.

**Firewall traversal** — GitHub Actions orchestrator talks to on-prem orchestrator. Both share a token, echo bot relays through Webex. `./echo:{slug}:{payload}` traverses the firewall without inbound ports.

**Multi-agent pub/sub** — Multiple agents on one account, each with a unique slug. All agents see all messages but only process their own slug. NATS-style message bus over a chat platform.

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
