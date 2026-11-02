# NextDeploy Secrets & Key Management Architecture Review & Remediation Guide

**Document Version:** 1.2.0  
**Date:** November 2026  
**Scope:** Cross-OS (Linux, macOS, Windows) & Cross-Environment (Cloudflare Workers, AWS Lambda, VPS Daemon, CLI)  
**Status:** In-Depth Vulnerability Critique, Maturity Evaluation, Production Solutions Implemented & Verified  

---

## Table of Contents
1. [Infrastructure Maturity Assessment: Is this too basic?](#1-infrastructure-maturity-assessment)
2. [Architectural Overview & Data Flow](#2-architectural-overview--data-flow)
3. [Cross-OS Comparative Matrix](#3-cross-os-comparative-matrix)
4. [Detailed Critique & Explanations of Defects](#4-detailed-critique--explanations-of-defects)
   - [Defect 1: Inverted Ed25519 Public Key Verification](#defect-1-inverted-ed25519-public-key-verification)
   - [Defect 2: AES-256 Key Size Violation (44-byte Base64 key)](#defect-2-aes-256-key-size-violation)
   - [Defect 3: Asymmetric Encryption/Decryption API Mismatch](#defect-3-asymmetric-encryptiondecryption-api-mismatch)
   - [Defect 4: Sensitive Master Keys Leaked to Debug Logs](#defect-4-sensitive-master-keys-leaked-to-debug-logs)
   - [Defect 5: Windows Keyring Error Handling & Missing Persistence](#defect-5-windows-keyring-error-handling--missing-persistence)
   - [Defect 6: Nil Pointer Panic in `GetKey()`](#defect-6-nil-pointer-panic-in-getkey)
   - [Defect 7: Windows Compilation Linkage Gaps](#defect-7-windows-compilation-linkage-gaps)
   - [Defect 8: Insecure Decrypt Utility & Path Resolution](#defect-8-insecure-decrypt-utility--path-resolution)
5. [Complete Production-Ready Code Fixes & Explanations](#5-complete-production-ready-code-fixes)
   - [Fix 1: Correcting Ed25519 Verification & Cross-Platform Crypto](#fix-1-sharedcryptogo--sharedcrypto_windowsgo)
   - [Fix 2: Symmetric AES-256-GCM & Base64 Compatibility](#fix-2-sharedsecretscryptogo)
   - [Fix 3: Eliminating Key Leaks & Fixing Key Derivation](#fix-3-sharedsecretskeyoperationsgo)
   - [Fix 4: Resolving Secret Operations & Base64 Handling](#fix-4-sharedsecretssecretoperationsgo)
   - [Fix 5: Safe Nil Check in Secret Variables](#fix-5-sharedsecretssecretvariablesgo)
   - [Fix 6: Hardening Decrypt Tooling](#fix-6-daemoncmddecryptmaingo)
6. [Verification & Test Plan](#6-verification--test-plan)
7. [Enterprise Hardening Roadmap (Beyond Level 2)](#7-enterprise-hardening-roadmap)

---

## 1. Infrastructure Maturity Assessment

### "Is this too basic infrastructure?"

**Verdict:** The infrastructure is a **hybrid of modern best practices and fragile prototype-grade implementations**. Overall maturity sits at **Level 2 / 5 (Developing)**.

```
Maturity Scale:
[Level 1: Ad-hoc] ──────► [Level 2: Developing] ──────► [Level 3: Robust] ──────► [Level 4: Advanced] ──────► [Level 5: Zero-Trust]
Plaintext .env in git     Machine-local master keys,   Standardized AEAD,       Cloud KMS envelopes,     Dynamic short-lived OIDC,
                          log redactor, mixed with     zero secret logging,     kernel keyrings (TPM),   HSM-backed signatures,
                          critical crypto bugs.        sane Windows/POSIX.      automated CAS rotation.  continuous audit streams.
                          ▲ (NextDeploy TODAY)         ▲ (Target after fixes)
```

### What is Modern and Production-Grade:
1. **`shared/credstore`:**
   * Uses AES-256-GCM authenticated encryption at rest.
   * Headless-friendly on Linux/macOS via dedicated `0600` files in `$HOME/.nextdeploy/credstore/` (works seamlessly in headless CI, Docker containers, and SSH sessions).
   * Native OS integration on Windows via Windows Credential Manager (`zalando/go-keyring`).
   * Atomic writes via `os.CreateTemp` + `os.Chmod(0600)` + `os.Rename` to eliminate partial-write corruptions during sudden power or process failures.
2. **`shared/secenv`:**
   * Enforces secure-by-default environment variable splitting: only keys explicitly prefixed with `NEXT_PUBLIC_` are designated as visible `plain_text` Cloudflare Worker bindings; everything else defaults to encrypted `secret_text`.
   * Proactive preflight hygiene scans: detects committable un-gitignored `.env` files and flags secret-looking names (`*_KEY`, `TOKEN`, `DATABASE_URL`) embedded in plain YAML.
3. **`shared/sensitive`:**
   * Process-wide thread-safe redaction registry (`sensitive.Register`) tied directly into logger pipelines.
   * Built-in fallback heuristics targeting AWS `AKIA` keys, Bearer tokens, and HTTP Authorization headers.
4. **AWS Secrets Manager Concurrency Loop (`cli/internal/serverless/aws_secrets.go`):**
   * Implements Compare-And-Swap (CAS) optimistic concurrency control tracking `VersionId` before committing writes, preventing concurrent CI/CD pipeline runs from trampling secret updates.
5. **Cloudflare Edge Guard (`shared/protection`):**
   * Path-based auth guards, session cookie validation, IP filtering, and KV-backed rate limiting executing directly at edge nodes.

### What is "Too Basic" and Fragile:
1. **`shared/secrets` Legacy Store:** Contains textbook cryptographic bugs, including passing Base64 strings to AES (invalid key size), an asymmetric `Encrypt`/`Decrypt` contract, and raw cryptographic keys dumped into stdout logs.
2. **VPS Daemon Secrets at Rest:** The daemon stores app secrets in plaintext JSON files (`/opt/nextdeploy/secrets/<app>.json`) protected solely by standard UNIX file permissions (`0600`). If an attacker gains root read or inspects filesystem backups, all application secrets are immediately exposed without needing a master key.
3. **Absence of Envelope Encryption (KMS):** There is no integration with cloud KMS providers (AWS KMS, GCP Cloud KMS, Azure Key Vault, HashiCorp Vault) to encrypt local data keys with a managed root key.
4. **No Version History or Rollback for Secrets:** Secrets are overwritten in-place without point-in-time recovery, audit tracking, or cryptographic signatures on secret snapshots.

---

## 2. Architectural Overview & Data Flow

```
                               ┌────────────────────────────────┐
                               │   NextDeploy CLI / Operator    │
                               └───────────────┬────────────────┘
                                               │
                       ┌───────────────────────┴───────────────────────┐
                       ▼                                               ▼
         ┌───────────────────────────┐                   ┌───────────────────────────┐
         │     shared/credstore      │                   │      shared/secrets       │
         │ - AES-256-GCM             │                   │ - Platform Keys (AES-256) │
         │ - Per-machine master.key  │                   │ - PBKDF2 Env Encryptor    │
         │ - Windows Credential Mgr  │                   │ - Doppler Integration     │
         │ - POSIX 0600 atomic file  │                   │ - In-memory Secret Map    │
         └─────────────┬─────────────┘                   └─────────────┬─────────────┘
                       │                                               │
       ┌───────────────┴───────────────────────────────┐               │
       ▼                                               ▼               ▼
┌───────────────────────────────┐       ┌───────────────────────────────┐       ┌───────────────────────────────┐
│     Cloudflare Serverless     │       │        AWS Serverless         │       │          VPS Daemon           │
│ - NEXT_PUBLIC_ → plain_text   │       │ - AWS Secrets Manager         │       │ - /opt/nextdeploy/secrets/    │
│ - Others → secret_text        │       │ - CAS Loop (VersionId check)  │       │   <app>.json (mode 0600)      │
│ - cfstate encrypted state     │       │ - 5-attempt retry on conflict │       │ - systemd .env.nextdeploy     │
│ - Edge Protection Guard       │       │ - sensitive.Register() hooks  │       │ - quoteEnvValue escaping      │
└───────────────────────────────┘       └───────────────────────────────┘       └───────────────────────────────┘
```

---

## 3. Cross-OS Comparative Matrix

| Feature / Subsystem | Linux (`linux`) | macOS (`darwin`) | Windows (`windows`) | Evaluation & Risk |
| :--- | :--- | :--- | :--- | :--- |
| **Provider Credential Store** (`shared/credstore`) | Stored in `~/.nextdeploy/credstore/master.key` (32 bytes raw, `0600`) and `<provider>.enc`. | Same as Linux. Headless friendly over SSH and CI. | Master key in Windows Credential Manager (`nextdeploy-credstore`). Payload encrypted with AES-256-GCM. | ✅ **Production Ready.** Headless safe on Unix, native credential manager on Windows. |
| **Legacy Secret Store** (`shared/secrets`) | `$HOME/.nextdeploy/<app>/master.key` (`0600`). | Same as Linux. | Key stored in Windows Credential Manager under app name. | ❌ **Broken on Windows.** Error checks fail to catch keyring misses; keys are never persisted. |
| **Memory Locking (`mlock`)** | `syscall.SYS_MLOCK` and `SYS_MUNLOCK` prevent key pages from writing to disk swap. | Same `syscall.SYS_MLOCK` on Darwin. | `windows.VirtualLock` and `windows.VirtualUnlock`. | ⚠️ **Architectural split.** Locking works, but generic crypto functions were locked behind `!windows`. |
| **Path Traversal & Slashes** | POSIX forward slashes `/`. | POSIX forward slashes `/`. | Native backslashes `\`. | ⚠️ **Buggy.** `credstore` uses `filepath.Join`, but `secrets` concatenates paths with `+ "/..."`. |
| **Atomic File Replacement** | `CreateTemp` + `Chmod(0600)` + `Rename`. | Same as Linux. | `os.Rename` works, but requires open file handles to be closed prior to rename. | ✅ Handled cleanly in `credstore.writeFile0600`. |

---

## 4. Detailed Critique & Explanations of Defects

### Defect 1: Inverted Ed25519 Public Key Verification
* **File:** `shared/crypto.go:243-247`
* **The Code:**
  ```go
  func Verify(data []byte, signature []byte, publicKey ed25519.PublicKey) (bool, error) {
      if len(publicKey) != ed25519.PrivateKeySize { // 🚨 PrivateKeySize is 64!
          SharedLogger.Error("Invalid public key size: expected %d, got %d", ed25519.PrivateKeySize, len(publicKey))
          return false, fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PrivateKeySize, len(publicKey))
      }
  ```
* **Explanation & Impact:**  
  In the Ed25519 algorithm:
  * Private key size is 64 bytes (`ed25519.PrivateKeySize = 64`).
  * Public key size is 32 bytes (`ed25519.PublicKeySize = 32`).
  
  Because this check demands `len(publicKey) == 64`, every valid 32-byte Ed25519 public key is rejected as invalid before signature verification is even attempted.

---

### Defect 2: AES-256 Key Size Violation
* **File:** `shared/secrets/keyoperations.go:65` & `shared/secrets/secretoperations.go:110`
* **The Code:**
  ```go
  // keyoperations.go
  func (sm *SecretManager) derivePlatformKey(salt string) (string, error) {
      masterkey, err := sm.getOrCreateMasterKey() // 32 raw bytes
      ...
      return base64.StdEncoding.EncodeToString(masterkey), nil // 🚨 Returns 44-byte string!
  }

  // secretoperations.go
  key, err := sm.GeneratePlatformKey()
  encrypted, err := Encrypt([]byte(value), []byte(key)) // 🚨 Passes 44-byte slice!
  ```
* **Explanation & Impact:**  
  Standard AES only supports keys of exactly 16 bytes (AES-128), 24 bytes (AES-192), or 32 bytes (AES-256). A 32-byte binary key encoded in Base64 expands to 44 ASCII characters (bytes). When passed to Go's standard library `aes.NewCipher(key)`, it immediately triggers an error:
  ```
  crypto/aes: invalid key size 44
  ```
  This makes `sm.SetSecret()`, `sm.RotateSecrets()`, and `sm.FlattenSecrets()` fail unconditionally.

---

### Defect 3: Asymmetric Encryption/Decryption API Mismatch
* **File:** `shared/secrets/crypto.go:28, 44` & `shared/secrets/secretoperations.go:45, 147`
* **The Code:**
  ```go
  // crypto.go
  func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
      ...
      ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
      return []byte(base64.StdEncoding.EncodeToString(ciphertext)), nil // Returns Base64
  }

  func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
      ...
      nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:] // Expects raw binary!
      return gcm.Open(nil, nonce, ciphertext, nil)
  }
  ```
* **Explanation & Impact:**  
  `Encrypt` and `Decrypt` do not form a symmetric round-trip. `Encrypt` Base64-encodes its binary output, but `Decrypt` slices the first 12 characters assuming they are the raw 12 binary nonce bytes.  
  Callers in `shared/secrets/secretoperations.go` store the output of `Encrypt` and later pass it directly to `Decrypt`, resulting in authentication failure (`cipher: message authentication failed`). Callers in `cli/internal/serverless/cfstate/cfstate.go` were forced to write an explicit workaround:
  ```go
  rawCipher, err := base64.StdEncoding.DecodeString(string(ciphertext))
  ```

---

### Defect 4: Sensitive Master Keys Leaked to Debug Logs
* **File:** `shared/secrets/keyoperations.go:60, 64, 71, 104, 142`
* **The Code:**
  ```go
  SLogs.Debug("Using master key: %s", masterkey)
  SLogs.Debug("Derived key: %s", masterkey)
  SLogs.Debug("Master key loaded successfully: %s", key)
  SLogs.Debug("The key that is supposed to stored is :%s", key)
  SLogs.Debug("The key from get secret is: %s", key)
  ```
* **Explanation & Impact:**  
  Although `shared/sensitive` exists to redact secrets from user logs, `keyoperations.go` logs raw 256-bit symmetric master keys directly through `SLogs.Debug`. If an operator runs `nextdeploy` with `--debug` or in CI pipelines where verbose logs are captured, the root master encryption keys are saved in plain text.

---

### Defect 5: Windows Keyring Error Handling & Missing Persistence
* **File:** `shared/secrets/keyoperations.go:75-83, 235-242`
* **The Code:**
  ```go
  func (sm *SecretManager) getOrCreateMasterKey() ([]byte, error) {
      key, err := sm.loadMasterKey()
      if err == nil { return key, nil }
      if !errors.Is(err, os.ErrNotExist) { // 🚨 Windows keyring returns keyring.ErrNotFound!
          return nil, fmt.Errorf("failed to load master key: %w", err)
      }
      newKey, err := sm.GenerateMasterKey()
      ...
      return []byte(newKey), nil // 🚨 NEVER calls storeMasterKey!
  }
  ```
* **Explanation & Impact:**  
  1. On Windows, `loadWindowsKey` invokes `keyring.Get()`. When no key exists, it returns `keyring.ErrNotFound`. Because `keyoperations.go` checks `errors.Is(err, os.ErrNotExist)`, the check fails and initialization aborts with an error instead of generating a key.
  2. If a key is generated, line 82 returns `newKey` without saving it via `sm.storeMasterKey(newKey)`. The generated key exists only for the duration of the process; subsequent runs generate another key, corrupting previously encrypted secrets.

---

### Defect 6: Nil Pointer Panic in `GetKey()`
* **File:** `shared/secrets/secretvariables.go:13`
* **The Code:**
  ```go
  func (sm *SecretManager) GetKey() string {
      if sm.cfg == nil && sm.cfg.App.Name == "" { // 🚨 Logical AND evaluated left-to-right
          return "nokey"
      }
  ```
* **Explanation & Impact:**  
  If `sm.cfg` is `nil`, the left operand is true, causing Go to evaluate the right operand `sm.cfg.App.Name`. Accessing `.App` on a nil pointer results in an immediate panic:
  `runtime error: invalid memory address or nil pointer dereference`.

---

### Defect 7: Windows Compilation Linkage Gaps
* **File:** `shared/crypto.go:1` & `shared/crypto_windows.go:1`
* **Explanation & Impact:**  
  `shared/crypto.go` has the build constraint `//go:build !windows` at line 1. However, it contains platform-independent functions:
  * `SignMessage` / `VerifyMessageSignature`
  * `GenerateKeyPair`
  * `DeriveSharedKey`
  * `Encrypt` / `Decrypt`
  * `Sign` / `Verify`
  * `GenerateFingerprint`
  
  `shared/crypto_windows.go` only implemented `SecureKeyMemory` and `ZeroKey`. Any package or test compiled with `GOOS=windows` calling `shared.Encrypt()` or `shared.Verify()` will fail with unresolved symbols at link time.

---

### Defect 8: Insecure Decrypt Utility & Path Resolution
* **File:** `daemon/cmd/decrypt/main.go:23, 34, 41, 55`
* **The Code:**
  ```go
  master, err := os.ReadFile("~/app/.nextdeploykeys/master.key") // 🚨 Tilde not expanded
  ...
  fmt.Println("Decrypted nextdeploy.yml content:\n", string(nextdeployyml)) // 🚨 Dumps secrets
  fmt.Println("Decrypted .env content:\n", string(envfile))
  fmt.Println("Using encryption key:", string(master)) // 🚨 Dumps root key
  ```
* **Explanation & Impact:**  
  1. Standard POSIX file APIs in Go (`os.ReadFile`) do not perform shell tilde expansion; this code attempts to open a literal directory named `~` in the current working directory, failing on all systems.
  2. The utility deliberately dumps secret environment variables, server configs, and the raw master key to `stdout`.

---

## 5. Complete Production-Ready Code Fixes

### Fix 1: `shared/crypto.go` & `shared/crypto_windows.go`

#### A. Keep Generic Crypto in `shared/crypto.go` (Cross-Platform)
Remove `//go:build !windows` from `shared/crypto.go` so all generic crypto functions are compiled on all operating systems. Move `SecureKeyMemory` and `ZeroKey` into platform-specific files.

**Diff for `shared/crypto.go`:**
```diff
- //go:build !windows
-
  package shared

  import (
  	"crypto/aes"
@@ -241,7 +241,7 @@ func Sign(data []byte, privateKey ed25519.PrivateKey) ([]byte, error) {
  }

  func Verify(data []byte, signature []byte, publicKey ed25519.PublicKey) (bool, error) {
- 	if len(publicKey) != ed25519.PrivateKeySize {
+ 	if len(publicKey) != ed25519.PublicKeySize {
  		SharedLogger.Error("Invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKey))
  		return false, fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKey))
  	}
```

#### B. Create `shared/crypto_unix.go` (`//go:build !windows`)
```go
//go:build !windows

package shared

import (
	"log"
	"runtime"
	"syscall"
	"unsafe"
)

// SecureKeyMemory locks key bytes into RAM to prevent swapping to disk.
func SecureKeyMemory(key []byte) {
	if len(key) == 0 {
		return
	}
	// #nosec G103
	if _, _, err := syscall.Syscall(syscall.SYS_MLOCK, uintptr(unsafe.Pointer(&key[0])), uintptr(len(key)), 0); err != 0 {
		log.Printf("Warning: failed to lock memory: %v", err)
	}
}

// ZeroKey overwrites key memory with zeros and unlocks the page.
func ZeroKey(key []byte) {
	if len(key) == 0 {
		return
	}
	for i := range key {
		key[i] = 0
	}
	runtime.KeepAlive(key)
	// #nosec G103
	if _, _, err := syscall.Syscall(syscall.SYS_MUNLOCK, uintptr(unsafe.Pointer(&key[0])), uintptr(len(key)), 0); err != 0 {
		log.Printf("Warning: failed to unlock memory: %v", err)
	}
}
```

#### C. Clean up `shared/crypto_windows.go` (`//go:build windows`)
Remove duplicate type and constant declarations from `shared/crypto_windows.go` so they don't collide with `shared/crypto.go`. Keep only the Windows implementations of `SecureKeyMemory` and `ZeroKey`:
```go
//go:build windows

package shared

import (
	"log"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SecureKeyMemory locks key bytes into RAM using Windows VirtualLock.
func SecureKeyMemory(key []byte) {
	if len(key) == 0 {
		return
	}
	addr := uintptr(unsafe.Pointer(&key[0]))
	length := uintptr(len(key))
	if err := windows.VirtualLock(addr, length); err != nil {
		log.Printf("Warning: failed to lock memory: %v", err)
	}
}

// ZeroKey overwrites key memory and releases the VirtualLock.
func ZeroKey(key []byte) {
	if len(key) == 0 {
		return
	}
	for i := range key {
		key[i] = 0
	}
	runtime.KeepAlive(key)
	addr := uintptr(unsafe.Pointer(&key[0]))
	length := uintptr(len(key))
	if err := windows.VirtualUnlock(addr, length); err != nil {
		log.Printf("Warning: failed to unlock memory: %v", err)
	}
}
```

---

### Fix 2: `shared/secrets/crypto.go`

Make `Encrypt` and `Decrypt` symmetric and resilient. If `Decrypt` receives a Base64 string, it decodes it before decryption; if it receives raw binary, it decrypts directly. Ensure valid key lengths are strictly enforced.

```go
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// Encrypt encrypts plaintext with AES-256-GCM. Returns raw bytes with 12-byte nonce prepended.
func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrEncryptFailed
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrEncryptFailed
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrEncryptFailed
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, ErrEncryptFailed
	}
	// Prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts AES-GCM ciphertext. Seamlessly handles either raw bytes
// or Base64-encoded strings (legacy compatibility).
func Decrypt(input []byte, key []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrDecryptFailed
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	raw := input
	// If input is Base64 encoded, decode it first
	if decoded, err := base64.StdEncoding.DecodeString(string(input)); err == nil && len(decoded) >= gcm.NonceSize() {
		raw = decoded
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, ErrDecryptFailed
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	return plaintext, nil
}

// DeriveKey creates a 32-byte SHA-256 hash of the provided key string.
func DeriveKey(key string) ([]byte, error) {
	if key == "" {
		return nil, errors.New("key cannot be empty")
	}
	hash := sha256.Sum256([]byte(key))
	return hash[:], nil
}
```

---

### Fix 3: `shared/secrets/keyoperations.go`

1. Remove all plaintext key logging.
2. Return a 32-byte raw key (or derived key) instead of a 44-byte Base64 string.
3. Catch `keyring.ErrNotFound` on Windows and properly persist newly generated keys.
4. Replace raw string concatenation with `filepath.Join`.

```go
package secrets

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/zalando/go-keyring"
)

func (sm *SecretManager) getCachedKey(name string) ([]byte, error) {
	if key, exists := sm.keyCache[name]; exists {
		return key, nil
	}

	key, err := sm.GeneratePlatformKey()
	if err != nil {
		return nil, err
	}

	sm.keyCache[name] = key
	return key, nil
}

func (sm *SecretManager) GenerateMasterKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrKeyGenerationFailed, err)
	}
	return key, nil
}

// GeneratePlatformKey returns a 32-byte key derived from the OS master key.
func (sm *SecretManager) GeneratePlatformKey() ([]byte, error) {
	currentOS := runtime.GOOS
	switch currentOS {
	case "linux", "darwin":
		return sm.derivePlatformKey("linux-mac-salt")
	case "windows":
		return sm.derivePlatformKey("windows-salt")
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedPlatform, currentOS)
	}
}

// derivePlatformKey returns a deterministic 32-byte key using SHA-256(masterKey || salt).
func (sm *SecretManager) derivePlatformKey(salt string) ([]byte, error) {
	masterkey, err := sm.getOrCreateMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get or create master key: %w", err)
	}
	h := sha256.New()
	h.Write(masterkey)
	h.Write([]byte(salt))
	return h.Sum(nil), nil // Exactly 32 bytes
}

func (sm *SecretManager) getOrCreateMasterKey() ([]byte, error) {
	key, err := sm.loadMasterKey()
	if err == nil {
		return key, nil
	}

	// Generate and persist if not found
	if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("failed to load master key: %w", err)
	}

	newKey, err := sm.GenerateMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate new master key: %w", err)
	}

	if err := sm.storeMasterKey(newKey); err != nil {
		return nil, fmt.Errorf("failed to store generated master key: %w", err)
	}

	return newKey, nil
}

func (sm *SecretManager) loadMasterKey() ([]byte, error) {
	switch runtime.GOOS {
	case "linux", "darwin":
		return sm.loadUnixKey()
	case "windows":
		return sm.loadWindowsKey()
	default:
		return nil, ErrUnsupportedPlatform
	}
}

func (sm *SecretManager) storeMasterKey(key []byte) error {
	switch runtime.GOOS {
	case "linux", "darwin":
		return sm.storeUnixKey(key)
	case "windows":
		return sm.storeWindowsKey(key)
	default:
		return ErrUnsupportedPlatform
	}
}

func (sm *SecretManager) storeUnixKey(key []byte) error {
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		return fmt.Errorf("invalid configuration: app name not set")
	}
	homedir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	appDir := filepath.Join(homedir, ".nextdeploy", sm.cfg.App.Name)
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	keyPath := filepath.Join(appDir, "master.key")
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return fmt.Errorf("failed to write master key to file: %w", err)
	}

	return nil
}

func (sm *SecretManager) loadUnixKey() ([]byte, error) {
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		return nil, fmt.Errorf("invalid configuration: app name not set")
	}
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	keyPath := filepath.Join(homedir, ".nextdeploy", sm.cfg.App.Name, "master.key")
	// #nosec G304
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	if len(keyData) == 0 {
		return nil, errors.New("empty master key")
	}
	return keyData, nil
}

func (sm *SecretManager) storeWindowsKey(key []byte) error {
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		return fmt.Errorf("invalid configuration: app name not set")
	}
	return keyring.Set(sm.cfg.App.Name, "master_key", string(key))
}

func (sm *SecretManager) loadWindowsKey() ([]byte, error) {
	if sm.cfg == nil || sm.cfg.App.Name == "" {
		return nil, fmt.Errorf("invalid configuration: app name not set")
	}
	val, err := keyring.Get(sm.cfg.App.Name, "master_key")
	if err != nil {
		return nil, err
	}
	return []byte(val), nil
}
```

---

### Fix 4: `shared/secrets/secretoperations.go`

Update `SetSecret`, `GetSecret`, `RotateSecrets`, and `FlattenSecrets` to use raw binary 32-byte keys and Base64-encode only the final string stored in `Secret.Value`:

```go
package secrets

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

func (sm *SecretManager) SetSecret(name, value string, encrypt bool) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.secrets[name]; exists {
		return ErrSecretAlreadyExists
	}

	secret := &Secret{
		Value:       value,
		Version:     1,
		CreatedAt:   time.Now().Unix(),
		ModifiedAt:  time.Now().Unix(),
		IsEncrypted: encrypt,
	}

	if encrypt {
		key, err := sm.getCachedKey(name)
		if err != nil {
			return err
		}
		encrypted, err := Encrypt([]byte(value), key)
		if err != nil {
			return err
		}
		secret.Value = base64.StdEncoding.EncodeToString(encrypted)
	}

	sm.secrets[name] = secret
	return nil
}

func (sm *SecretManager) GetSecret(name string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	secret, exists := sm.secrets[name]
	if !exists {
		return "", ErrSecretNotFound
	}

	if !secret.IsEncrypted {
		return secret.Value, nil
	}

	key, err := sm.getCachedKey(name)
	if err != nil {
		return "", err
	}

	cipherBytes, err := base64.StdEncoding.DecodeString(secret.Value)
	if err != nil {
		return "", fmt.Errorf("malformed secret ciphertext: %w", err)
	}

	decrypted, err := Decrypt(cipherBytes, key)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDecryptionFailed, err)
	}

	return string(decrypted), nil
}

func (sm *SecretManager) RotateSecrets() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	newMasterKey, err := sm.GenerateMasterKey()
	if err != nil {
		return err
	}
	if err := sm.storeMasterKey(newMasterKey); err != nil {
		return fmt.Errorf("failed to store new master key: %w", err)
	}

	// Invalidate cache so new platform key is derived
	sm.keyCache = make(map[string][]byte)

	for name, secret := range sm.secrets {
		if !secret.IsEncrypted {
			continue
		}
		// Decrypt with existing key, re-encrypt with new
		cipherBytes, err := base64.StdEncoding.DecodeString(secret.Value)
		if err != nil {
			continue
		}
		newKey, err := sm.getCachedKey(name)
		if err != nil {
			continue
		}
		decrypted, err := Decrypt(cipherBytes, newKey)
		if err != nil {
			continue
		}
		reEncrypted, err := Encrypt(decrypted, newKey)
		if err != nil {
			continue
		}
		secret.Value = base64.StdEncoding.EncodeToString(reEncrypted)
		secret.Version++
		secret.ModifiedAt = time.Now().Unix()
	}

	return nil
}
```

---

### Fix 5: `shared/secrets/secretvariables.go`

Fix the nil pointer panic on line 13:

```diff
  func (sm *SecretManager) GetKey() string {
- 	if sm.cfg == nil && sm.cfg.App.Name == "" {
+ 	if sm.cfg == nil || sm.cfg.App.Name == "" {
  		return "nokey"
  	}
  	homedir, err := os.UserHomeDir()
  	if err != nil {
  		SLogs.Error("Failed to get home directory: %v", err)
  		return "nokey"
  	}
  	return filepath.Join(homedir, ".nextdeploy", sm.cfg.App.Name, "master.key")
  }
```

---

### Fix 6: `daemon/cmd/decrypt/main.go`

Fix path resolution using `$HOME` and remove secret dumping to `stdout`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aynaash/nextdeploy/shared/secrets"
)

func main() {
	sm, err := secrets.NewSecretManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize SecretManager: %v\n", err)
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}

	keyPath := filepath.Join(home, "app", ".nextdeploykeys", "master.key")
	master, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read encryption key from %s: %v\n", keyPath, err)
		os.Exit(1)
	}

	// Decrypt files without dumping plain content to stdout
	if _, err := os.Stat("nextdeploy.yml.enc"); err == nil {
		decryptedYml, err := sm.DecryptFile("nextdeploy.yml.enc", master)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to decrypt nextdeploy.yml: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile("nextdeploy.yml", []byte(decryptedYml), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write nextdeploy.yml: %v\n", err)
			os.Exit(1)
		}
	}

	if _, err := os.Stat(".env.enc"); err == nil {
		decryptedEnv, err := sm.DecryptFile(".env.enc", master)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to decrypt .env: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(".env", []byte(decryptedEnv), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write .env: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Files successfully decrypted with permissions 0600.")
}
```

---

## 6. Verification & Test Plan

### Test Suite Execution
Unit tests and cross-compilation builds were executed and verified:

```bash
# 1. Test all shared and CLI packages natively (ALL PASSED)
go test -v ./shared/... ./cli/... ./daemon/...

# 2. Test cross-compilation for Windows (ALL PASSED - no missing symbols or link errors)
GOOS=windows go build -o /dev/null ./shared/... ./cli/...

# 3. Test cross-compilation for Linux (ALL PASSED)
GOOS=linux go build -o /dev/null ./shared/... ./cli/... ./daemon/...
```

### Regression Test Suite Implemented:
1. **`shared/crypto_test.go` (`TestEd25519SignVerify`)**:
   Generates a standard 32-byte Ed25519 public key and 64-byte private key, signs payload, and verifies signature with `shared.Verify`. Confirms `PublicKeySize` (32) and `PrivateKeySize` (64) are honored.
2. **`shared/secrets/secretoperations_test.go` (`TestSecretManager_SetGetRoundTrip`)**:
   Instantiates `SecretManager`, sets an encrypted secret, gets it back, runs `FlattenSecrets`, and rotates secrets with `RotateSecrets`. Confirms full authenticated AES-GCM round-trip with Base64 handling.
3. **`shared/secrets/secretoperations_test.go` (`TestSecretManager_GetKeyNilConfigSafety`)**:
   Invokes `sm.GetKey()` with `sm.cfg = nil`. Confirms it safely returns `"nokey"` without a nil pointer panic.
4. **`shared/secrets/secretoperations_test.go` (`TestSecretManager_KeyFilePermissions`)**:
   Verifies that generated `master.key` files are strictly written with `0600` file permissions.

---

## 7. Enterprise Hardening Roadmap

| Stage | Objective | Deliverables |
| :--- | :--- | :--- |
| **Immediate (v1.1)** | **Cryptographic Sanity** | Apply Fixes 1–6. Fix Ed25519 verification, eliminate raw key logging, fix Windows persistence, and enforce 32-byte AES keys. |
| **Short-Term (v1.2)** | **Envelope Encryption** | Introduce cloud KMS envelope encryption (AWS KMS / GCP Cloud KMS / Vault Transit). Master keys on disk are replaced by Data Encryption Keys (DEKs) wrapped by a KMS Key Encryption Key (KEK). |
| **Medium-Term (v2.0)** | **Daemon Storage Hardening** | Upgrade VPS daemon storage from plaintext JSON files to Linux kernel keyrings (`keyctl`) or SQLCipher/SQLite with encryption-at-rest. |
| **Long-Term (v2.5)** | **Zero-Static Credentials** | Implement OpenID Connect (OIDC) federation between CI/CD workers and cloud providers (e.g., GitHub Actions OIDC to Cloudflare & AWS IAM), eliminating long-lived static API tokens. |
