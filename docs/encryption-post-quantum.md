# AES-256-GCM vs ChaCha20-Poly1305 — Post-Quantum Notes

## Is AES-256-GCM post-quantum?

**Yes.** AES-256 is already considered post-quantum safe for symmetric encryption.

- Grover's algorithm (quantum) halves symmetric key strength: AES-256 → 128-bit effective security
- 128-bit is still unbreakable (2^128 operations)
- AES-NI hardware acceleration works the same — it's the same AES instruction set
- The mode of operation (GCM, CTR, etc.) doesn't change the quantum story

The post-quantum threat is to **asymmetric** crypto (RSA, ECDH, key exchange), not symmetric ciphers. Since wgrok uses a pre-shared symmetric key (`WGROK_ENCRYPT_KEY`), there's no asymmetric key exchange to attack.

## Comparison

| | AES-256-GCM | ChaCha20-Poly1305 |
|--|-------------|-------------------|
| Key size | 256-bit | 256-bit |
| Post-quantum | Yes (128-bit effective) | Yes (128-bit effective) |
| AES-NI hardware | Yes — fast on modern CPUs | No — software only |
| Without AES-NI | Slow (older ARM, IoT) | Fast everywhere |
| Used by | TLS, AWS, most enterprise | WireGuard, Signal, TLS |
| Nonce size | 12 bytes | 12 bytes |
| Auth tag | 16 bytes (GHASH) | 16 bytes (Poly1305) |

## When to use which

- **AES-256-GCM** — CPUs with AES-NI (all modern x86, most ARM servers). Faster with hardware acceleration.
- **ChaCha20-Poly1305** — Devices without AES-NI (older phones, embedded, IoT). Fast in pure software.

WireGuard uses ChaCha20 because it targets embedded/mobile where AES-NI isn't guaranteed. Signal uses it for the same reason — mobile-first.

## wgrok's choice

AES-256-GCM is the right choice for wgrok:

- Runs on servers and dev machines (all have AES-NI)
- Pre-shared symmetric key — no asymmetric key exchange to worry about
- 256-bit key → 128-bit post-quantum effective security → unbreakable
- Hardware-accelerated on all target platforms

No changes needed for post-quantum readiness.

## Post-quantum threat model

| Crypto type | Quantum threat | wgrok impact |
|-------------|---------------|--------------|
| Symmetric (AES-256) | Grover halves key → 128-bit | Safe — 128-bit is enough |
| Asymmetric key exchange (RSA, ECDH) | Shor breaks it completely | N/A — wgrok uses pre-shared keys |
| Hash (SHA-256) | Grover halves → 128-bit | Safe — used internally by GCM |

## If you ever need to switch

Swapping to ChaCha20-Poly1305 would be straightforward — same key size (256-bit), same nonce size (12 bytes), same tag size (16 bytes), same wire format `base64(nonce || ciphertext || tag)`. The only change would be the cipher construction in each language's `codec.encrypt`/`codec.decrypt`.

| Language | ChaCha20 library |
|----------|-----------------|
| Python | `cryptography` (same lib, different cipher) |
| Go | `golang.org/x/crypto/chacha20poly1305` |
| TypeScript | `node:crypto` (same module, `chacha20-poly1305` algorithm) |
| Rust | `chacha20poly1305` crate |
