# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- Optional AES-256-GCM encryption across all four languages
  - On/off mode: set `WGROK_ENCRYPT_KEY` to enable, omit to disable
  - `e` flag in protocol wire format indicates encrypted payload
  - Pipeline: compress → encrypt → chunk (send), reassemble → decrypt → decompress (receive)
  - 32-byte base64-encoded symmetric key via environment variable

## [0.3.0] - 2026-03-17

### Changed
- **Protocol v2**: Four-field wire format `./echo:{to}:{from}:{flags}:{payload}`
  - `from` field enables return addressing and reply-to patterns
  - `flags` field carries compression (`z`), chunking (`N/M`), and metadata
  - Breaking change from v1 two-field format

### Added
- Payload codec with gzip+base64 compression and auto-chunking
- Platform-aware message limits (Webex 7439B, Slack 4000, Discord 2000, IRC 400)
- `compress` parameter on sender for automatic gzip+base64 encoding
- Receiver auto-decompression and chunk reassembly

## [0.2.0] - 2026-03-16

### Added
- Platform listener abstraction for WebSocket/TCP receive across all languages
- Slack, Discord, and IRC transport bindings (send + receive)
- Multi-platform support with `WGROK_{PLATFORM}_TOKENS` environment variables
- 429 retry handling with exponential backoff for all platforms

### Changed
- Renamed EchoBot to RouterBot across all languages
- Updated all dependencies to latest versions

### Fixed
- Go response body leak in retry loop
- webex-message-handler v0.5.1 for Go ping loop panic fix

## [0.1.0] - 2026-03-14

### Added
- Initial implementation in Python, Go, TypeScript, and Rust
- Mode B (agent bus) — shared token, agents self-select by slug
- Mode C (registered agents) — `WGROK_ROUTES` maps slugs to bot identities
- Webhook endpoint for non-WebSocket integrations (`WGROK_WEBHOOK_PORT`)
- Domain-based allowlist/ACL via `WGROK_DOMAINS`
- Adaptive card support (send, relay, receive)
- Outbound proxy support via `WGROK_PROXY`
- AsyncAPI 3.0.0 protocol specification
- Shared JSON test cases consumed by all four languages
- Cross-language test runner (`tests/run_all.sh`)
