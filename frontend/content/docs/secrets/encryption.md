---
title: Local Authenticated Encryption
lede: Mathematical and implementation details of NextDeploy's ChaCha20-Poly1305 and AES-256-GCM authenticated secret encryption.
status: ready
sourceRef: shared/secrets/crypto.go, shared/crypto.go
related:
  - title: Secrets Overview
    href: /docs/secrets/overview
  - title: Doppler Integration
    href: /docs/secrets/doppler
---

## Cryptographic Primitives

NextDeploy utilizes **Authenticated Encryption with Associated Data (AEAD)** to ensure both confidentiality and cryptographic integrity for all stored secrets:

1. **Primary Cipher**: `ChaCha20-Poly1305` (RFC 8439).
   - High speed on ARM64 and x86_64 architectures without hardware AES acceleration.
   - Immune to cache-timing attacks.
2. **Alternative Cipher**: `AES-256-GCM` (NIST SP 800-38D).
   - Compliant with strict enterprise FIPS requirements.
3. **Key Derivation Function (KDF)**: `Argon2id` (RFC 9106) with:
   - Memory: 64 MB (`memory = 64 * 1024`)
   - Iterations: 3
   - Parallelism: 4
   - Salt: 16 cryptographically secure random bytes generated via `crypto/rand`.

---

## Vault File Format (`.secrets`)

When you run `nextdeploy secrets set`, the secret payload is encrypted into a binary vault file:

```
┌──────────────────────────────────────────────────────────────┐
│                    VAULT BINARY LAYOUT                       │
├──────────────┬──────────────┬──────────────┬─────────────────┤
│ Magic (4B)   │ Version (1B) │ Salt (16B)   │ Nonce (12B)     │
├──────────────┴──────────────┴──────────────┴─────────────────┤
│ Ciphertext + Poly1305 / GCM Authentication Tag (Variable)    │
└──────────────────────────────────────────────────────────────┘
```

1. **Magic Bytes**: `0x4E 0x44 0x53 0x43` (`NDSC` for NextDeploy Secret Container).
2. **Version**: Current specification version (`0x01`).
3. **Salt**: Unique random salt for Argon2id key derivation.
4. **Nonce**: 96-bit (12-byte) random initialization vector. A new nonce is generated for every single write operation.
5. **Tag**: 128-bit authentication tag. Any tampering or bit-flipping causes decryption to fail immediately.

---

## Key Management & Environment Variables

NextDeploy resolves the master encryption key from the following sources in order:

1. Environment variable: `NEXTDEPLOY_ENCRYPTION_KEY` (64-character hexadecimal string).
2. Secure local key file: `~/.nextdeploy/master.key` (permission `0600`).
3. Automated key generation: If neither exists, NextDeploy generates a cryptographically random 256-bit key and saves it to `~/.nextdeploy/master.key`.

### CI/CD Deployment Key
In automated CI/CD pipelines (GitHub Actions, GitLab CI), add `NEXTDEPLOY_ENCRYPTION_KEY` to your repository secrets. During deployment, the CLI uses this key to decrypt secrets in memory and inject them directly into your deployment target.

---

## Key Rotation

Rotate your vault's encryption key with zero downtime:

```bash
nextdeploy secrets rotate --new-key <hex_key>
```
NextDeploy decrypts the existing vault using the current master key, re-derives a new key with fresh salt, and re-encrypts all secrets with a new nonce.
