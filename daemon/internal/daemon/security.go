package daemon

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"
)

type RateLimiter struct {
	tokens map[string]float64
	last   map[string]time.Time
	mu     sync.Mutex
	rate   float64
	burst  float64
}

func NewRateLimiter(rate, burst float64) *RateLimiter {
	return &RateLimiter{
		tokens: make(map[string]float64),
		last:   make(map[string]time.Time),
		rate:   rate,
		burst:  burst,
	}
}

func (rl *RateLimiter) Allow(id string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	// Initialize if not present
	if _, ok := rl.tokens[id]; !ok {
		rl.tokens[id] = rl.burst
		rl.last[id] = now
	}

	// Refill tokens
	elapsed := now.Sub(rl.last[id]).Seconds()
	rl.tokens[id] += elapsed * rl.rate
	if rl.tokens[id] > rl.burst {
		rl.tokens[id] = rl.burst
	}
	rl.last[id] = now

	if rl.tokens[id] >= 1.0 {
		rl.tokens[id] -= 1.0
		return true
	}
	return false
}

func VerifySignature(payload []byte, signature, secret string) bool {
	if secret == "" {
		// Fail closed. An empty secret means the daemon is misconfigured —
		// it must never be treated as "authentication disabled". The daemon
		// generates and persists a random secret on startup
		// (config.EnsureSecuritySecret), so this branch only rejects.
		return false
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// ReplayGuard rejects commands whose signed timestamp is outside an allowed
// skew window, and commands whose nonce has already been seen within that
// window. Combined with the HMAC signature (which covers the timestamp and
// nonce), this prevents a captured command from being replayed.
type ReplayGuard struct {
	mu     sync.Mutex
	seen   map[string]time.Time
	window time.Duration
}

func NewReplayGuard(window time.Duration) *ReplayGuard {
	return &ReplayGuard{
		seen:   make(map[string]time.Time),
		window: window,
	}
}

// Check validates freshness and uniqueness. ts is a Unix timestamp (seconds)
// and nonce is a per-command random value; both are part of the signed payload.
func (rg *ReplayGuard) Check(ts int64, nonce string) error {
	if ts == 0 {
		return fmt.Errorf("missing command timestamp")
	}
	if nonce == "" {
		return fmt.Errorf("missing command nonce")
	}

	now := time.Now()
	skew := now.Sub(time.Unix(ts, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > rg.window {
		return fmt.Errorf("command timestamp outside %s window", rg.window)
	}

	rg.mu.Lock()
	defer rg.mu.Unlock()

	for n, t := range rg.seen {
		if now.Sub(t) > rg.window {
			delete(rg.seen, n)
		}
	}
	if _, ok := rg.seen[nonce]; ok {
		return fmt.Errorf("replayed command nonce")
	}
	rg.seen[nonce] = now
	return nil
}

func IsIPAllowed(ipStr string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true // No whitelist means all are allowed (standard behavior for local deployments)
	}

	// Remove port if present
	host, _, err := net.SplitHostPort(ipStr)
	if err == nil {
		ipStr = host
	}

	clientIP := net.ParseIP(ipStr)
	if clientIP == nil {
		return false
	}

	for _, cidr := range whitelist {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			if ipNet.Contains(clientIP) {
				return true
			}
		} else {
			// Try exact IP match if not a CIDR
			if cidr == ipStr {
				return true
			}
		}
	}
	return false
}
