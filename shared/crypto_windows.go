//go:build windows

package shared

import (
	"log"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func SecureKeyMemory(key []byte) {
	if len(key) == 0 {
		return
	}

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

	for i := range key {
		key[i] = 0
	}

	runtime.KeepAlive(key)
	addr := uintptr(unsafe.Pointer(&key[0]))
	length := uintptr(len(key))

	err := windows.VirtualUnlock(addr, length)
	if err != nil {
		log.Printf("Warning: failed to unlock memory: %v", err)
	}
}
