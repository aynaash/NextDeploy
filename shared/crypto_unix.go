//go:build !windows

package shared

import (
	"log"
	"runtime"
	"syscall"
	"unsafe"
)

func SecureKeyMemory(key []byte) {
	if len(key) == 0 {
		return
	}

	// #nosec G103
	if _, _, err := syscall.Syscall(syscall.SYS_MLOCK, uintptr(unsafe.Pointer(&key[0])), uintptr(len(key)), 0); err != 0 {
		log.Printf("Warning: failed to lock memory: %v", err)
	}
}

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
