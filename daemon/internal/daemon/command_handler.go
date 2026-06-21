package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aynaash/nextdeploy/daemon/internal/types"
	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/nextcore"
	"github.com/aynaash/nextdeploy/shared/updater"
)

const (
	baseDir       = "/opt/nextdeploy"
	appsDir       = "/opt/nextdeploy/apps"
	uploadsDir    = "/opt/nextdeploy/uploads"
	workTmpDir    = "/opt/nextdeploy/tmp"
	nextdeployDir = ".nextdeploy"
)

type CommandHandler struct {
	config         *types.DaemonConfig
	caddyManager   *CaddyManager
	processManager *ProcessManager
	stateManager   *StateManager
	auditLogger    *AuditLogger
	rateLimiter    *RateLimiter
	replayGuard    *ReplayGuard
	deployLocks    *appLocker
}

// appLocker serializes mutating operations (ship, rollback, destroy) per app so
// two overlapping operations for the same app cannot interleave their `current`
// symlink flips or persisted port writes. The rate limiter throttles request
// rate but does not serialize mutations — this does. Operations fail fast with a
// clear message rather than queueing, so the operator never waits on a silent stall.
type appLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newAppLocker() *appLocker {
	return &appLocker{locks: make(map[string]*sync.Mutex)}
}

// tryAcquire grabs the per-app lock without blocking. The returned release func
// is nil and ok is false if another operation already holds the lock.
func (a *appLocker) tryAcquire(app string) (release func(), ok bool) {
	a.mu.Lock()
	m, exists := a.locks[app]
	if !exists {
		m = &sync.Mutex{}
		a.locks[app] = m
	}
	a.mu.Unlock()

	if !m.TryLock() {
		return nil, false
	}
	return m.Unlock, true
}

func NewCommandHandler(config *types.DaemonConfig) *CommandHandler {
	statePath := "/var/lib/nextdeployd/state.json"
	if os.Geteuid() != 0 {
		home, _ := os.UserHomeDir()
		statePath = filepath.Join(home, ".nextdeploy", "state.json")
	}

	auditPath := filepath.Join(config.LogDir, "audit.log")

	rate := config.RateLimitRate
	if rate == 0 {
		rate = 10 // default 10 requests per second
	}
	burst := config.RateLimitBurst
	if burst == 0 {
		burst = 20
	}

	return &CommandHandler{
		config:         config,
		caddyManager:   NewCaddyManager(),
		processManager: NewProcessManager(),
		stateManager:   NewStateManager(statePath),
		auditLogger:    NewAuditLogger(auditPath),
		rateLimiter:    NewRateLimiter(rate, burst),
		replayGuard:    NewReplayGuard(5 * time.Minute),
		deployLocks:    newAppLocker(),
	}
}

var allowedCommands = map[string]struct{}{
	"setupCaddy":    {},
	"stopdaemon":    {},
	"restartDaemon": {},
	"ship":          {},
	"rollback":      {},
	"secrets":       {},
	"status":        {},
	"logs":          {},
	"destroy":       {},
	"stop":          {},
}

func (ch *CommandHandler) ValidateCommand(cmd types.Command) error {
	if _, ok := allowedCommands[cmd.Type]; !ok {
		return fmt.Errorf("command not allowed: %s", cmd.Type)
	}
	return nil
}

func (ch *CommandHandler) HandleCommand(cmd types.Command, clientIdentity string) types.Response {
	// 1. Rate Limiting
	if !ch.rateLimiter.Allow(clientIdentity) {
		return types.Response{Success: false, Message: "rate limit exceeded"}
	}

	// 2. IP Whitelisting (for non-Unix socket identities)
	if !IsIPAllowed(clientIdentity, ch.config.IPWhitelist) {
		return types.Response{Success: false, Message: "IP not whitelisted"}
	}

	// 3. Signature Verification
	// We marshal the type, args, timestamp and nonce to verify the signature.
	// These four fields must match exactly what the client signed in client.go.
	payload, _ := json.Marshal(map[string]any{
		"type":      cmd.Type,
		"args":      cmd.Args,
		"timestamp": cmd.Timestamp,
		"nonce":     cmd.Nonce,
	})
	if !VerifySignature(payload, cmd.Signature, ch.config.SecuritySecret) {
		return types.Response{Success: false, Message: "invalid command signature"}
	}

	// 3b. Replay Protection: reject stale or already-seen commands. Because the
	// timestamp and nonce are part of the signed payload above, an attacker
	// cannot alter them without invalidating the signature.
	if err := ch.replayGuard.Check(cmd.Timestamp, cmd.Nonce); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("replay protection: %v", err)}
	}

	var resp types.Response
	switch cmd.Type {
	case "setupCaddy":
		resp = ch.setUpCaddy(cmd.Args)
	case "stopdaemon":
		resp = ch.stopDaemon(cmd.Args)
	case "restartDaemon":
		resp = ch.restartDaemon(cmd.Args)
	case "ship":
		resp = ch.handleShip(cmd.Args)
	case "rollback":
		resp = ch.handleRollback(cmd.Args)
	case "secrets":
		resp = ch.handleSecrets(cmd.Args)
	case "status":
		resp = ch.handleStatus(cmd.Args)
	case "logs":
		resp = ch.handleLogs(cmd.Args)
	case "destroy":
		resp = ch.handleDestroy(cmd.Args)
	case "stop":
		resp = ch.handleStopApp(cmd.Args)
	default:
		resp = types.Response{
			Success: false,
			Message: fmt.Sprintf("unknown command: %s", cmd.Type),
		}
	}

	// 4. Audit Logging
	ch.auditLogger.Log(AuditEntry{
		CommandType:    cmd.Type,
		ClientIdentity: clientIdentity,
		Result:         fmt.Sprintf("%v", resp.Success),
		ErrorDetails:   resp.Message,
		Args:           cmd.Args,
	})

	return resp
}

func (ch *CommandHandler) stopDaemon(args map[string]any) types.Response {
	log.Println("Stopping daemon...")
	ch.Shutdown()
	return types.Response{Success: true, Message: "daemon stopped"}
}

func (ch *CommandHandler) restartDaemon(args map[string]any) types.Response {
	log.Println("Restarting daemon...")

	execPath, err := os.Executable()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("failed to resolve executable path: %v", err),
		}
	}

	// #nosec G204
	cmd := exec.Command(execPath, "--foreground=true", "--config="+ch.config.ConfigPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("failed to start new daemon process: %v", err),
		}
	}

	log.Printf("New daemon process started (pid %d), shutting down current...", cmd.Process.Pid)

	// Minimal delay to let the new process bind the socket if necessary
	time.Sleep(100 * time.Millisecond)
	ch.Shutdown()

	return types.Response{Success: true, Message: "daemon restarted"}
}

func (ch *CommandHandler) Shutdown() {
	log.Println("Shutting down daemon...")
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		log.Printf("Failed to find own process: %v", err)
		return
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		log.Printf("Failed to send interrupt: %v", err)
	}
}

func (ch *CommandHandler) setUpCaddy(args map[string]any) types.Response {
	setup, ok := args["setup"].(bool)
	if !ok || !setup {
		return types.Response{
			Success: false,
			Message: "setupCaddy requires 'setup: true'",
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("failed to resolve home directory: %v", err),
		}
	}
	caddyfilePath := filepath.Join(homeDir, "app", nextdeployDir, "caddy", "Caddyfile")

	log.Printf("Reading Caddyfile from: %s", caddyfilePath)
	// #nosec G304
	caddyfileContent, err := os.ReadFile(caddyfilePath)
	if err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("failed to read Caddyfile at %s: %v", caddyfilePath, err),
		}
	}

	if err := os.WriteFile("/etc/caddy/Caddyfile", caddyfileContent, 0600); err != nil {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("failed to write /etc/caddy/Caddyfile: %v", err),
		}
	}

	systemctl := resolveTool("systemctl")
	if out, err := exec.Command("caddy", "reload", "--config", "/etc/caddy/Caddyfile").CombinedOutput(); err != nil {
		log.Printf("caddy reload failed (%v: %s), attempting systemctl start...", err, string(out))
		// #nosec G204
		if out2, err2 := exec.Command(systemctl, "start", "caddy").CombinedOutput(); err2 != nil {
			return types.Response{
				Success: false,
				Message: fmt.Sprintf("failed to start Caddy service: %v — %s", err2, string(out2)),
			}
		}
	}

	log.Println("Caddy configured and running.")
	return types.Response{Success: true, Message: "Caddy configured and running"}
}

func (ch *CommandHandler) handleShip(args map[string]any) types.Response {
	// Auto-update check before processing deployment
	// This ensures the daemon updates itself when a new version is available
	go func() {
		if err := updater.SelfUpdateDaemon(shared.Version); err != nil {
			if !strings.Contains(err.Error(), "up to date") {
				log.Printf("[ship] Warning: auto-update check failed: %v", err)
			}
		}
	}()

	tarballPath, ok := StringArg(args, "tarball")
	if !ok {
		return types.Response{Success: false, Message: "missing 'tarball' argument"}
	}

	// Path sanitization
	tarballPath = filepath.Clean(tarballPath)
	if !strings.HasPrefix(tarballPath, uploadsDir) {
		return types.Response{Success: false, Message: "security error: tarball path must be within uploads directory"}
	}

	log.Printf("[ship] Starting deployment from: %s", tarballPath)

	// Ensure workTmpDir exists
	if err := os.MkdirAll(workTmpDir, 0750); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to ensure tmp dir: %v", err)}
	}

	tmpDir, err := os.MkdirTemp(workTmpDir, "unpack-*")
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to create temp dir in %s: %v", workTmpDir, err)}
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	log.Printf("[ship] Extracting to %s...", tmpDir)
	if err := shared.ExtractTarGz(tarballPath, tmpDir); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("extraction failed: %v", err)}
	}

	meta, err := readMetadata(tmpDir)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("metadata error: %v", err)}
	}

	appName := Coalesce(meta.AppName, "default-app")
	if err := validateAppName(appName); err != nil {
		return types.Response{Success: false, Message: err.Error()}
	}

	// Serialize mutating ops per app: a concurrent ship/rollback/destroy for the
	// same app must not interleave symlink flips or port writes.
	release, ok := ch.deployLocks.tryAcquire(appName)
	if !ok {
		return types.Response{Success: false, Message: fmt.Sprintf("another deploy or rollback for %q is already in progress", appName)}
	}
	defer release()

	domain := Coalesce(meta.Domain, "localhost")
	if err := validateDomain(domain); err != nil {
		return types.Response{Success: false, Message: err.Error()}
	}

	outputMode := string(meta.OutputMode)

	log.Printf("[ship] App=%s domain=%s mode=%s pkg=%s", appName, domain, outputMode, meta.PackageManager)

	// Release IDs are {unix-timestamp}-{shortSha}. The leading timestamp keeps
	// lexicographic ordering aligned with chronological ordering (so the existing
	// sort.Strings rollback walk still works), while the trailing short SHA gives
	// a human-readable identity for operators and lets --toCommit lookups avoid
	// reading every metadata.json. Falls back to "nogit" when the build had no
	// git context (e.g. CI without a checkout).
	timestamp := time.Now().Unix()
	releaseID := fmt.Sprintf("%d-%s", timestamp, shortSha(meta.GitCommit))
	releaseDir := filepath.Join(appsDir, appName, "releases", releaseID)

	// #nosec G301 G703
	if err := os.MkdirAll(filepath.Dir(releaseDir), 0750); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to create releases dir: %v", err)}
	}

	// Ensure the app directory is owned by nextdeploy
	ch.ensureAppDirOwnership(appName)

	log.Printf("[ship] Moving %s -> %s", tmpDir, releaseDir)
	if err := os.Rename(tmpDir, releaseDir); err != nil {
		if isCrossDevice(err) {
			log.Printf("[ship] Cross-device rename, falling back to copy...")
			if err := copyDir(tmpDir, releaseDir); err != nil {
				return types.Response{Success: false, Message: fmt.Sprintf("failed to copy release: %v", err)}
			}
			_ = os.RemoveAll(tmpDir)
		} else {
			return types.Response{Success: false, Message: fmt.Sprintf("failed to move release: %v", err)}
		}
	}
	cleanupTmp = false

	// Fix permissions and ownership for the release directory
	ch.ensureDirPermissions(releaseDir)

	dopplerToken, _ := StringArg(args, "dopplerToken")
	log.Printf("[ship] %s extracted to %s, activating...", appName, releaseDir)

	ctx := ReleaseContext{
		AppName:          appName,
		Domain:           domain,
		ReleaseDir:       releaseDir,
		ReleaseID:        releaseID,
		OutputMode:       outputMode,
		DopplerToken:     dopplerToken,
		PackageManager:   meta.PackageManager,
		TarballPath:      tarballPath,
		DetectedFeatures: meta.DetectedFeatures,
		DistDir:          meta.DistDir,
		ExportDir:        meta.ExportDir,
		Resources:        meta.Resources,
		HealthPath:       meta.HealthPath,
	}
	return ch.activateRelease(ctx)
}

type ReleaseContext struct {
	AppName          string
	Domain           string
	ReleaseDir       string
	ReleaseID        string
	OutputMode       string
	DopplerToken     string
	PackageManager   string
	TarballPath      string
	DetectedFeatures *nextcore.DetectedFeatures
	DistDir          string
	ExportDir        string
	Resources        *config.ResourceLimits
	HealthPath       string
}

func (ch *CommandHandler) activateRelease(ctx ReleaseContext) types.Response {
	// Host runtime drift: records the baseline on first deploy; on later deploys
	// warns loudly if Node/glibc/arch changed out from under the compiled
	// artifact (e.g. an apt upgrade). Streams to the operator's CLI.
	for _, w := range CheckRuntimeDrift(ch.stateManager) {
		log.Printf("[drift] ⚠️  Host environment drift detected — %s. The artifact was compiled against a different runtime and may misbehave; re-prepare or rebuild if it crashes.", w)
	}

	currentSymlink := filepath.Join(appsDir, ctx.AppName, "current")
	var serviceName string
	var serviceGenerated bool
	var portAcquired int
	var err error

	// Port selection with persistence
	port := ch.stateManager.GetPort(ctx.AppName)
	var cleanupPort func() error
	if port == 0 || !isPortAvailable(port) {
		p, closePort, err := findFreePort()
		if err != nil {
			return types.Response{Success: false, Message: fmt.Sprintf("failed to allocate port: %v", err)}
		}
		cleanupPort = closePort
		portAcquired = p
		port = p
		ch.stateManager.SetPort(ctx.AppName, port)
		if err := ch.stateManager.Save(); err != nil {
			log.Printf("[activate] Warning: failed to save state: %v", err)
		}
		// Defer port cleanup in case of failure
		defer func() {
			if portAcquired != 0 && cleanupPort != nil {
				_ = cleanupPort()
			}
		}()
	}

	log.Printf("[activate] Allocated port %d for release %s", port, ctx.ReleaseID)

	serviceName, serviceGenerated, err = ch.processManager.GenerateServiceFile(
		ctx.AppName, ctx.ReleaseDir, ctx.OutputMode, ctx.DopplerToken, port, ctx.PackageManager, ctx.ReleaseID, ctx.Resources,
	)
	if err != nil {
		ch.stateManager.SetPort(ctx.AppName, 0)
		_ = ch.stateManager.Save()
		return types.Response{Success: false, Message: fmt.Sprintf("failed to generate service file: %v", err)}
	}

	if serviceGenerated {
		// Port release: we must close our listener BEFORE starting the service so it can bind to the port
		if portAcquired != 0 && cleanupPort != nil {
			log.Printf("[activate] Releasing reserved port %d before starting service", port)
			_ = cleanupPort()
			portAcquired = 0 // Mark as released
		}

		if err := ch.processManager.StartService(serviceName); err != nil {
			_ = ch.processManager.RemoveService(serviceName)
			ch.stateManager.SetPort(ctx.AppName, 0)
			_ = ch.stateManager.Save()
			return types.Response{Success: false, Message: fmt.Sprintf("failed to start service: %v", err)}
		}
	}

	log.Printf("[activate] Waiting for app to become healthy on port %d...", port)
	if err := waitForHealthy(port, ctx.HealthPath, 5*time.Minute); err != nil {
		log.Printf("[activate] Health check failed on port %d, cleaning up...", port)
		if serviceGenerated {
			_ = ch.processManager.RemoveService(serviceName)
		}
		// Release the port back to the pool
		ch.stateManager.SetPort(ctx.AppName, 0)
		_ = ch.stateManager.Save()
		return types.Response{Success: false, Message: fmt.Sprintf("health check failed after 5m: %v", err)}
	}

	// Port is now in use by the healthy service - close our listener if it was still open
	if portAcquired != 0 && cleanupPort != nil {
		_ = cleanupPort()
		portAcquired = 0 // Mark as released
	}

	// Port file for Caddy and other discovery tools (write after health check passes)
	portFilePath := filepath.Join(appsDir, ctx.AppName, "port")
	if err := os.WriteFile(portFilePath, fmt.Appendf(nil, "%d", port), 0644); err != nil {
		log.Printf("[activate] Warning: failed to write port file to %s: %v", portFilePath, err)
	}

	// Atomic symlink update
	tmpSymlink := currentSymlink + ".tmp"
	_ = os.Remove(tmpSymlink)
	if err := os.Symlink(ctx.ReleaseDir, tmpSymlink); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to create atomic symlink: %v", err)}
	}

	// Persistent Static Assets Sync
	// We sync .next/static to a shared directory so old client sessions don't break when Caddy switches roots
	sharedStaticDir := filepath.Join(appsDir, ctx.AppName, "shared_static")
	sourceStaticDir := filepath.Join(ctx.ReleaseDir, ctx.DistDir, "static")
	if _, err := os.Stat(sourceStaticDir); err == nil {
		if err := os.MkdirAll(sharedStaticDir, 0755); err == nil {
			// Use cp -R to copy assets, overwriting existing ones to ensure newest versions are served
			cpPath := resolveTool("cp")
			// #nosec G204
			cmd := exec.Command(cpPath, "-R", sourceStaticDir+string(filepath.Separator)+".", sharedStaticDir)
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("[activate] Warning: failed to sync shared static assets: %v - %s", err, string(out))
			} else {
				log.Printf("[activate] Synced static assets to %s", sharedStaticDir)
				_ = exec.Command(resolveTool("chown"), "-R", "nextdeploy:nextdeploy", sharedStaticDir).Run()
			}
		}
	}

	if err := os.Rename(tmpSymlink, currentSymlink); err != nil {
		_ = os.Remove(tmpSymlink)
		return types.Response{Success: false, Message: fmt.Sprintf("failed to rename atomic symlink: %v", err)}
	}

	if err := ch.caddyManager.EnsureMainCaddyfile(); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to update main Caddyfile: %v", err)}
	}

	if err := ch.caddyManager.GenerateConfig(ctx.AppName, ctx.Domain, ctx.OutputMode, port, currentSymlink, ctx.DetectedFeatures, ctx.DistDir, ctx.ExportDir); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to configure Caddy: %v", err)}
	}

	if err := ch.caddyManager.Validate(); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("Caddy validation failed: %v", err)}
	}
	_ = ch.caddyManager.Reload()

	if services, err := ch.processManager.FindAppServices(ctx.AppName); err == nil {
		for _, s := range services {
			if s != serviceName {
				log.Printf("[activate] Cleaning up old service: %s", s)
				_ = ch.processManager.RemoveService(s)
			}
		}
	}

	if err := pruneReleases(ctx.AppName, 5); err != nil {
		log.Printf("[activate] Warning: failed to prune releases: %v", err)
	}

	if ctx.TarballPath != "" {
		_ = os.Remove(ctx.TarballPath)
	}

	return types.Response{
		Success: true,
		Message: fmt.Sprintf("Successfully activated release %s for %s", ctx.ReleaseID, ctx.AppName),
	}
}

func (ch *CommandHandler) handleRollback(args map[string]any) types.Response {
	appName, ok := StringArg(args, "appName")
	if !ok {
		return types.Response{Success: false, Message: "missing 'appName' argument"}
	}
	if err := validateAppName(appName); err != nil {
		return types.Response{Success: false, Message: err.Error()}
	}

	release, ok := ch.deployLocks.tryAcquire(appName)
	if !ok {
		return types.Response{Success: false, Message: fmt.Sprintf("another deploy or rollback for %q is already in progress", appName)}
	}
	defer release()

	// Optional rollback selectors. JSON-over-socket decodes numbers as float64.
	steps := 1
	if v, ok := args["steps"].(float64); ok && v > 0 {
		steps = int(v)
	}
	toCommit, _ := StringArg(args, "toCommit")
	if toCommit != "" && steps != 1 {
		return types.Response{Success: false, Message: "--toCommit and --steps are mutually exclusive"}
	}

	releasesDir := filepath.Join(appsDir, appName, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to read releases: %v", err)}
	}

	var releases []string
	for _, e := range entries {
		if e.IsDir() {
			releases = append(releases, e.Name())
		}
	}

	if len(releases) < 2 {
		return types.Response{Success: false, Message: "not enough releases to rollback"}
	}

	// Lex sort works because releaseID is {unix-ts}-{shortSha}; the timestamp
	// prefix preserves chronological order. Legacy bare-timestamp release dirs
	// from older daemon versions still sort correctly because they share the
	// same numeric leading segment.
	sort.Strings(releases)

	previousReleaseID, err := resolveRollbackTarget(releasesDir, releases, steps, toCommit)
	if err != nil {
		return types.Response{Success: false, Message: err.Error()}
	}
	previousReleaseDir := filepath.Join(releasesDir, previousReleaseID)

	meta, err := readMetadata(previousReleaseDir)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to read metadata of previous release: %v", err)}
	}

	domain := Coalesce(meta.Domain, "localhost")
	outputMode := string(meta.OutputMode)
	dopplerToken, _ := StringArg(args, "dopplerToken")

	log.Printf("[rollback] Reverting %s to release %s", appName, previousReleaseID)

	ctx := ReleaseContext{
		AppName:          appName,
		Domain:           domain,
		ReleaseDir:       previousReleaseDir,
		ReleaseID:        previousReleaseID,
		OutputMode:       outputMode,
		DopplerToken:     dopplerToken,
		PackageManager:   meta.PackageManager,
		TarballPath:      "",
		DetectedFeatures: meta.DetectedFeatures,
		DistDir:          meta.DistDir,
		ExportDir:        meta.ExportDir,
		Resources:        meta.Resources,
		HealthPath:       meta.HealthPath,
	}
	return ch.activateRelease(ctx)
}

// shortSha returns a 7-char prefix of a git commit hash, or "nogit" when the
// commit is unavailable. Kept identical in spirit to the AWS-side helper so
// release identifiers are consistent across deploy targets.
func shortSha(full string) string {
	if len(full) >= 7 {
		return full[:7]
	}
	if full == "" {
		return "nogit"
	}
	return full
}

// resolveRollbackTarget picks the release directory to roll back to. The
// `releases` slice must already be sorted oldest-first. The active deployment
// is the last entry; it is never returned.
//
//   - toCommit (full or short SHA): walks releases newest-first, matching by
//     release-dir suffix first (cheap, no I/O), then by reading each
//     metadata.json (covers legacy releases). Errors if not found within the
//     retention window.
//   - steps (default 1): returns the entry `steps` positions before the active
//     one. Errors if `steps` exceeds available history.
func resolveRollbackTarget(releasesDir string, releases []string, steps int, toCommit string) (string, error) {
	if len(releases) < 2 {
		return "", fmt.Errorf("not enough releases to rollback (found %d, need at least 2)", len(releases))
	}

	if toCommit != "" {
		needle := strings.ToLower(strings.TrimSpace(toCommit))
		// Walk newest-first, skipping the active deployment (last entry).
		for i := len(releases) - 2; i >= 0; i-- {
			id := releases[i]
			// Fast path: releaseID is "{ts}-{shortSha}" since the rename, so a
			// suffix prefix-match avoids touching disk.
			if dash := strings.LastIndex(id, "-"); dash >= 0 {
				if strings.HasPrefix(strings.ToLower(id[dash+1:]), needle) {
					return id, nil
				}
			}
			// Fallback: legacy bare-timestamp releases — read metadata.json.
			meta, err := readMetadata(filepath.Join(releasesDir, id))
			if err == nil && strings.HasPrefix(strings.ToLower(meta.GitCommit), needle) {
				return id, nil
			}
		}
		return "", fmt.Errorf("commit %q not found in release history (retention window may have pruned it)", toCommit)
	}

	if steps <= 0 {
		steps = 1
	}
	if len(releases) < steps+1 {
		return "", fmt.Errorf("not enough releases to rollback %d step(s) (found %d, need at least %d)", steps, len(releases), steps+1)
	}
	return releases[len(releases)-1-steps], nil
}

// ExtractTarGz is now handled universally by shared.ExtractTarGz

func readMetadata(unpackDir string) (*nextcore.NextCorePayload, error) {
	candidates := []string{
		filepath.Join(unpackDir, nextdeployDir, "metadata.json"),
		filepath.Join(unpackDir, "metadata.json"),
	}
	for _, path := range candidates {
		// #nosec G304
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var meta nextcore.NextCorePayload
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		return &meta, nil
	}
	return nil, fmt.Errorf("metadata.json not found in tarball (checked %v)", candidates)
}

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func findFreePort() (int, func() error, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, fmt.Errorf("failed to find free port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return port, ln.Close, nil
}

// waitForHealthy gates the cutover. It first waits for the port to accept a TCP
// connection (fast-fail while the process is still starting), then escalates to
// an HTTP GET on healthPath and requires a status < 500. A Next.js process can
// accept TCP while returning 500 on every request (bad env, failed DB connect,
// broken ISR) — the HTTP probe stops the daemon from flipping `current` onto a
// release that is "up" but serving errors. healthPath defaults to "/".
func waitForHealthy(port int, healthPath string, timeout time.Duration) error {
	if healthPath == "" {
		healthPath = "/"
	}
	if !strings.HasPrefix(healthPath, "/") {
		healthPath = "/" + healthPath
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := fmt.Sprintf("http://%s%s", addr, healthPath)
	deadline := time.Now().Add(timeout)

	client := &http.Client{Timeout: 3 * time.Second}
	backoff := 100 * time.Millisecond
	maxBackoff := 2 * time.Second

	for time.Now().Before(deadline) {
		// Gate 1: TCP dial — cheap liveness while the process is still binding.
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			_ = conn.Close()

			// Gate 2: HTTP probe — require the app to actually serve a non-5xx
			// response before we consider it healthy.
			resp, herr := client.Get(url)
			if herr == nil {
				_ = resp.Body.Close()
				if resp.StatusCode < 500 {
					return nil
				}
				log.Printf("[activate] %s returned %d, still waiting for a healthy response...", url, resp.StatusCode)
			}
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	return fmt.Errorf("app did not become healthy on %s within %s", url, timeout)
}

func pruneReleases(appName string, keep int) error {
	releasesDir := filepath.Join(appsDir, appName, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read releases dir: %w", err)
	}

	// Collect directory names and sort explicitly before trimming. ReadDir
	// returns lexically-sorted names, but relying on that ordering is brittle:
	// a single non-conforming dir name (legacy bare-timestamp, manual dir) could
	// otherwise prune the wrong release. The release-ID scheme {unix-ts}-{sha}
	// makes lexical order == chronological order, so sort.Strings is canonical —
	// mirroring the explicit sort in handleRollback.
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	for _, name := range releasesToPrune(names, keep) {
		path := filepath.Join(releasesDir, name)
		log.Printf("[prune] Removing old release: %s", path)
		if err := os.RemoveAll(path); err != nil {
			log.Printf("[prune] Warning: failed to remove %s: %v", path, err)
		}
	}
	return nil
}

// releasesToPrune returns the release IDs to delete, keeping the newest `keep`
// by release-ID order. Pure and sort-stable so it can be unit-tested
// independently of the filesystem.
func releasesToPrune(names []string, keep int) []string {
	if len(names) <= keep {
		return nil
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	return sorted[:len(sorted)-keep]
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			// #nosec G301 G703
			return os.MkdirAll(target, 0750)
		}
		if d.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", path, err)
			}
			// #nosec G301 G703
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	// #nosec G304
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// #nosec G301 G703
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	// #nosec G304 G703
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func isCrossDevice(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid cross-device link")
}

func (ch *CommandHandler) ensureAppDirOwnership(appName string) {
	appDir := filepath.Join(appsDir, appName)
	ch.ensureDirPermissions(appDir)
}

func (ch *CommandHandler) ensureDirPermissions(root string) {
	findPath := resolveTool("find")
	chownPath := resolveTool("chown")
	chmodPath := resolveTool("chmod")

	// Optimization: Skip chown if already correct.
	// #nosec G204
	chownCmd := exec.Command(findPath, root, "(", "!", "-user", "nextdeploy", "-o", "!", "-group", "nextdeploy", ")", "-exec", chownPath, "nextdeploy:nextdeploy", "{}", "+")
	if out, err := chownCmd.CombinedOutput(); err != nil {
		log.Printf("[ship] Warning: optimized chown failed: %v - %s", err, string(out))
		// #nosec G204
		_ = exec.Command(chownPath, "-R", "nextdeploy:nextdeploy", root).Run()
	}

	// #nosec G204
	chmodDirCmd := exec.Command(findPath, root, "-type", "d", "!", "-perm", "0750", "-exec", chmodPath, "0750", "{}", "+")
	if out, err := chmodDirCmd.CombinedOutput(); err != nil {
		log.Printf("[ship] Warning: failed to chmod dirs in %s: %v - %s", root, err, string(out))
	}

	// #nosec G204
	chmodFileCmd := exec.Command(findPath, root, "-type", "f", "!", "-perm", "0644", "-exec", chmodPath, "0644", "{}", "+")
	if out, err := chmodFileCmd.CombinedOutput(); err != nil {
		log.Printf("[ship] Warning: failed to chmod files in %s: %v - %s", root, err, string(out))
	}
}
func (ch *CommandHandler) handleDestroy(args map[string]any) types.Response {
	appName, ok := StringArg(args, "appName")
	if !ok {
		return types.Response{Success: false, Message: "missing 'appName' argument"}
	}

	release, ok := ch.deployLocks.tryAcquire(appName)
	if !ok {
		return types.Response{Success: false, Message: fmt.Sprintf("another deploy or rollback for %q is already in progress", appName)}
	}
	defer release()

	log.Printf("[destroy] Destroying app: %s", appName)

	var errors []string

	// 1. Stop and remove services
	services, err := ch.processManager.FindAppServices(appName)
	if err != nil {
		errors = append(errors, fmt.Sprintf("failed to find app services: %v", err))
	} else {
		for _, s := range services {
			log.Printf("[destroy] Removing service: %s", s)
			if err := ch.processManager.RemoveService(s); err != nil {
				errors = append(errors, fmt.Sprintf("failed to remove service %s: %v", s, err))
			}
		}
	}

	// 2. Remove Caddy configuration
	if err := ch.caddyManager.RemoveConfig(appName); err != nil {
		log.Printf("[destroy] Warning: failed to remove Caddy config: %v", err)
		errors = append(errors, fmt.Sprintf("failed to remove Caddy config: %v", err))
	}
	_ = ch.caddyManager.Reload()

	// 3. Remove application files
	appDir := filepath.Join(appsDir, appName)
	if _, err := os.Stat(appDir); err == nil {
		if err := os.RemoveAll(appDir); err != nil {
			log.Printf("[destroy] Warning: failed to remove app directory %s: %v", appDir, err)
			errors = append(errors, fmt.Sprintf("failed to remove app directory: %v", err))
		}
	}

	// 4. Remove uploads
	if entries, err := os.ReadDir(uploadsDir); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), appName) {
				path := filepath.Join(uploadsDir, entry.Name())
				log.Printf("[destroy] Removing upload artifact: %s", path)
				_ = os.Remove(path)
			}
		}
	}

	// 5. Remove logs
	// We check common log locations
	logPaths := []string{
		fmt.Sprintf("/var/log/containers/%s.log", appName),
		filepath.Join("/var/log/containers", appName),
	}
	for _, lp := range logPaths {
		if _, err := os.Stat(lp); err == nil {
			log.Printf("[destroy] Removing log artifact: %s", lp)
			if err := os.RemoveAll(lp); err != nil {
				log.Printf("[destroy] Warning: failed to remove log %s: %v", lp, err)
			}
		}
	}

	// 6. Clean up state
	ch.stateManager.SetPort(appName, 0)
	if err := ch.stateManager.Save(); err != nil {
		errors = append(errors, fmt.Sprintf("failed to save state: %v", err))
	}

	if len(errors) > 0 {
		msg := fmt.Sprintf("App %s destruction completed with warnings:\n- %s", appName, strings.Join(errors, "\n- "))
		log.Printf("[destroy] %s", msg)
		return types.Response{Success: true, Message: msg}
	}

	log.Printf("[destroy] App %s successfully destroyed with deep cleanup", appName)
	return types.Response{Success: true, Message: fmt.Sprintf("app %s successfully destroyed with deep cleanup", appName)}
}

func (ch *CommandHandler) handleStopApp(args map[string]any) types.Response {
	appName, ok := StringArg(args, "appName")
	if !ok {
		return types.Response{Success: false, Message: "missing 'appName' argument"}
	}

	log.Printf("[stop] Stopping app: %s", appName)

	services, err := ch.processManager.FindAppServices(appName)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to find app services: %v", err)}
	}

	if len(services) == 0 {
		return types.Response{Success: false, Message: fmt.Sprintf("no services found for app %s", appName)}
	}

	var errors []string
	for _, s := range services {
		log.Printf("[stop] Stopping service: %s", s)
		if err := ch.processManager.StopService(s); err != nil {
			errors = append(errors, fmt.Sprintf("failed to stop service %s: %v", s, err))
		}
	}

	if len(errors) > 0 {
		return types.Response{
			Success: false,
			Message: fmt.Sprintf("Failed to stop some services for %s:\n- %s", appName, strings.Join(errors, "\n- ")),
		}
	}

	return types.Response{Success: true, Message: fmt.Sprintf("App %s stopped successfully", appName)}
}
