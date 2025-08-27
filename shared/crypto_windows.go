//go:build windows
// +build windows

package shared

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"log"
	"math/big"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows-specific implementations of SecureKeyMemory and ZeroKey
func SecureKeyMemory(key []byte) {
	if len(key) == 0 {
		return
	}

	// Use Windows VirtualLock instead of mlock
	addr := uintptr(unsafe.Pointer(&key[0]))
	length := uintptr(len(key))

	err := windows.VirtualLock(addr, length)
	if err != nil {
		log.Printf("Warning: failed to lock memory: %v", err)
	}
}

func ZeroKey(key []byte) {
	if len(key) == 0 {
		return
	}

	// Use constant-time zeroing
	for i := range key {
		key[i] = 0
	}

	// Ensure compiler doesn't optimize this away
	runtime.KeepAlive(key)

	// Use Windows VirtualUnlock instead of munlock
	addr := uintptr(unsafe.Pointer(&key[0]))
	length := uintptr(len(key))

	err := windows.VirtualUnlock(addr, length)
	if err != nil {
		log.Printf("Warning: failed to unlock memory: %v", err)
	}
}

// The rest of the functions are the same as in the Unix version
// but need to be copied here for Windows build

const (
	KeyIDLength       = 32
	NonceSize         = 12
	SignatureSize     = ed25519.SignatureSize
	PublicKeySize     = 32
	PrivateKeySize    = 32
	SharedKeySize     = 32
	FingerprintLength = 16
)

type ECCSignature struct {
	R *big.Int
	S *big.Int
}

var (
	SharedLogger = PackageLogger("shared", "🔗 SHARED")
)

type KeyPair struct {
	ECDHPrivate *ecdh.PrivateKey
	ECDHPublic  *ecdh.PublicKey
	SignPrivate ed25519.PrivateKey
	SignPublic  ed25519.PublicKey
	ECDSAKey    *ecdsa.PrivateKey
	KeyID       string
}

// All other functions (SignMessage, VerifyMessageSignature, GenerateKeyPair, etc.)
// need to be copied here exactly as they appear in the Unix version
// [Copy all the remaining functions from the Unix version here]
