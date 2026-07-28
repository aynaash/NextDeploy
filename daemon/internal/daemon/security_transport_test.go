package daemon

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/aynaash/nextdeploy/daemon/internal/types"
)

// signedCommand builds a command that passes signature + replay verification, so
// a test can reach the authz checks that come before dispatch.
func signedCommand(t *testing.T, secret, nonce string) types.Command {
	t.Helper()
	cmd := types.Command{
		Type:      "status",
		Args:      map[string]interface{}{},
		Timestamp: time.Now().Unix(),
		Nonce:     nonce,
	}
	payload, err := json.Marshal(map[string]interface{}{
		"type":      cmd.Type,
		"args":      cmd.Args,
		"timestamp": cmd.Timestamp,
		"nonce":     cmd.Nonce,
	})
	if err != nil {
		t.Fatal(err)
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	cmd.Signature = hex.EncodeToString(h.Sum(nil))
	return cmd
}

func newTestHandler(t *testing.T, whitelist []string) *CommandHandler {
	t.Helper()
	return &CommandHandler{
		config: &types.DaemonConfig{
			SecuritySecret: "test-secret",
			IPWhitelist:    whitelist,
		},
		rateLimiter: NewRateLimiter(100, 100),
		replayGuard: NewReplayGuard(5 * time.Minute),
		auditLogger: NewAuditLogger(filepath.Join(t.TempDir(), "audit.log")),
	}
}

func TestHandleCommandUnixSocketBypassesIPWhitelist(t *testing.T) {
	// The Unix socket is gated by filesystem permissions and carries no IP.
	// Adding ip_whitelist to lock down TCP must not lock out the primary local
	// control channel.
	ch := newTestHandler(t, []string{"203.0.113.0/24"})
	cmd := signedCommand(t, "test-secret", "nonce-local")

	resp := ch.HandleCommand(cmd, localSocketIdentity, true)
	if !resp.Success && resp.Message == "IP not whitelisted" {
		t.Fatal("unix-socket command was rejected by the IP whitelist")
	}
}

func TestHandleCommandTCPStillEnforcesIPWhitelist(t *testing.T) {
	ch := newTestHandler(t, []string{"203.0.113.0/24"})

	offList := signedCommand(t, "test-secret", "nonce-tcp-denied")
	resp := ch.HandleCommand(offList, "198.51.100.7:5555", false)
	if resp.Success || resp.Message != "IP not whitelisted" {
		t.Fatalf("off-whitelist TCP peer should be refused, got %+v", resp)
	}

	onList := signedCommand(t, "test-secret", "nonce-tcp-allowed")
	resp = ch.HandleCommand(onList, "203.0.113.9:5555", false)
	if !resp.Success && resp.Message == "IP not whitelisted" {
		t.Fatal("whitelisted TCP peer was refused")
	}
}

func TestHandleCommandLocalFlagDoesNotBypassSignature(t *testing.T) {
	// isLocal relaxes the IP gate only. Filesystem permissions decide who may
	// open the socket; the HMAC still decides whether a command is authentic.
	ch := newTestHandler(t, nil)
	cmd := signedCommand(t, "wrong-secret", "nonce-badsig")

	resp := ch.HandleCommand(cmd, localSocketIdentity, true)
	if resp.Success || resp.Message != "invalid command signature" {
		t.Fatalf("local peer with a bad signature should be refused, got %+v", resp)
	}
}

func TestRateLimiterEvictsIdleBuckets(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	clock := time.Now()
	rl.now = func() time.Time { return clock }
	rl.lastGC = clock

	for i := 0; i < 10000; i++ {
		rl.Allow(fmt.Sprintf("10.0.0.1:%d", 1024+i))
	}
	if len(rl.last) != 10000 {
		t.Fatalf("setup: len(last) = %d, want 10000", len(rl.last))
	}

	// Advance past idleTTL so every bucket above is stale, then touch one id.
	clock = clock.Add(rl.idleTTL + time.Second)
	rl.Allow("active-peer")

	if len(rl.last) != 1 {
		t.Errorf("after sweep len(last) = %d, want 1 (only the active peer)", len(rl.last))
	}
	if len(rl.tokens) != 1 {
		t.Errorf("after sweep len(tokens) = %d, want 1", len(rl.tokens))
	}
	if _, ok := rl.last["active-peer"]; !ok {
		t.Error("the active peer's bucket was swept")
	}
}

func TestRateLimiterKeepsActiveBuckets(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	clock := time.Now()
	rl.now = func() time.Time { return clock }
	rl.lastGC = clock

	// A peer touched just before the sweep must survive it.
	rl.Allow("busy")
	clock = clock.Add(rl.idleTTL + time.Second)
	rl.Allow("busy")
	rl.Allow("other")

	if _, ok := rl.last["busy"]; !ok {
		t.Error("recently-active bucket was evicted")
	}
}

func TestRateLimiterEvictionIsBehaviourallyTransparent(t *testing.T) {
	// Eviction is only safe because a fresh bucket starts full. Prove an evicted
	// id gets exactly the same allowance as one that merely refilled.
	rl := NewRateLimiter(10, 20)
	clock := time.Now()
	rl.now = func() time.Time { return clock }
	rl.lastGC = clock

	// Drain "drained" to empty.
	for rl.Allow("drained") { //nolint:revive // draining the bucket is the point
	}

	clock = clock.Add(rl.idleTTL + time.Second)

	allowed := 0
	for rl.Allow("drained") {
		allowed++
	}
	if allowed != int(rl.burst) {
		t.Errorf("post-eviction allowance = %d, want a full burst of %d", allowed, int(rl.burst))
	}
}
