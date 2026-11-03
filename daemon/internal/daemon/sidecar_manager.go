package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aynaash/nextdeploy/shared/nextcore"
)

const sidecarPortFile = "sidecar.port"

type SidecarManager struct {
	mu        sync.Mutex
	instances map[string]*managedSidecar
	appsRoot  string
	logger    *log.Logger
}

type managedSidecar struct {
	appName string
	release string
	port    int
	server  *http.Server
	handler atomic.Pointer[SidecarHandler]
}

func NewSidecarManager() *SidecarManager {
	return newSidecarManager(appsDir)
}

func newSidecarManager(appsRoot string) *SidecarManager {
	return &SidecarManager{instances: make(map[string]*managedSidecar), appsRoot: appsRoot, logger: log.Default()}
}

func (sm *SidecarManager) SetLogger(logger *log.Logger) {
	if logger == nil {
		logger = log.Default()
	}
	sm.logger = logger
}

func (sm *SidecarManager) Upsert(appName, releaseDir, sharedStaticDir string, nodePort int, meta *nextcore.NextCorePayload) (int, error) {
	if err := validateAppName(appName); err != nil {
		return 0, err
	}
	if meta == nil {
		return 0, fmt.Errorf("sidecar metadata is required")
	}
	appDir := filepath.Join(sm.appsRoot, appName)
	release, err := filepath.Abs(releaseDir)
	if err != nil {
		return 0, fmt.Errorf("resolve sidecar release directory: %w", err)
	}
	handler, err := NewSidecarHandler(release, sharedStaticDir, filepath.Join(appDir, "cache", "images", filepath.Base(release)), nodePort, meta)
	if err != nil {
		return 0, err
	}
	handler.logger = sm.logger

	key := sidecarInstanceKey(appName, release)
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if existing, ok := sm.instances[key]; ok {
		old := existing.handler.Swap(handler)
		if old != nil {
			old.Close()
		}
		sm.logger.Printf("[gosidecar] hot-swapped route table for %s on 127.0.0.1:%d -> node 127.0.0.1:%d", appName, existing.port, nodePort)
		return existing.port, nil
	}
	if err := os.MkdirAll(appDir, 0o750); err != nil {
		return 0, fmt.Errorf("create sidecar app directory: %w", err)
	}

	portPath := filepath.Join(release, ".nextdeploy", sidecarPortFile)
	if err := os.MkdirAll(filepath.Dir(portPath), 0o750); err != nil {
		return 0, fmt.Errorf("create release sidecar state directory: %w", err)
	}
	port, err := readSidecarPort(portPath)
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	address := "127.0.0.1:0"
	if err == nil {
		address = fmt.Sprintf("127.0.0.1:%d", port)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return 0, fmt.Errorf("bind sidecar listener at %s: %w", address, err)
	}
	port = listener.Addr().(*net.TCPAddr).Port
	if err := writeSidecarPort(portPath, port); err != nil {
		_ = listener.Close()
		return 0, err
	}

	instance := &managedSidecar{appName: appName, release: release, port: port}
	instance.handler.Store(handler)
	instance.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == sidecarHealthURI {
				w.WriteHeader(http.StatusOK)
				return
			}
			if h := instance.handler.Load(); h != nil {
				h.ServeHTTP(w, r)
				return
			}
			http.Error(w, "sidecar initializing", http.StatusServiceUnavailable)
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	sm.instances[key] = instance
	go func() {
		sm.logger.Printf("[gosidecar] started in-daemon sidecar for %s on 127.0.0.1:%d (node=127.0.0.1:%d)", appName, port, nodePort)
		if err := instance.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			sm.logger.Printf("[gosidecar] listener error for %s: %v", appName, err)
			sm.removeFailedInstance(key, instance)
		}
	}()
	return port, nil
}

func sidecarInstanceKey(appName, releaseDir string) string {
	return appName + "\x00" + filepath.Clean(releaseDir)
}

func (sm *SidecarManager) removeFailedInstance(key string, failed *managedSidecar) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.instances[key] == failed {
		delete(sm.instances, key)
	}
}

func readSidecarPort(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid sidecar port in %s", path)
	}
	return port, nil
}

func writeSidecarPort(path string, port int) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sidecar-port-*")
	if err != nil {
		return fmt.Errorf("create sidecar port file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set sidecar port file permissions: %w", err)
	}
	if _, err := fmt.Fprintf(tmp, "%d\n", port); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write sidecar port file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync sidecar port file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close sidecar port file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("persist sidecar port: %w", err)
	}
	return nil
}

func (sm *SidecarManager) Stop(appName string) error {
	sm.mu.Lock()
	var stopped []*managedSidecar
	for key, instance := range sm.instances {
		if instance.appName == appName {
			stopped = append(stopped, instance)
			delete(sm.instances, key)
		}
	}
	sm.mu.Unlock()
	var errs []error
	for _, instance := range stopped {
		if err := sm.stopInstance(instance); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (sm *SidecarManager) StopRelease(appName, releaseDir string) error {
	release, err := filepath.Abs(releaseDir)
	if err != nil {
		return fmt.Errorf("resolve sidecar release directory: %w", err)
	}
	key := sidecarInstanceKey(appName, release)
	sm.mu.Lock()
	instance, ok := sm.instances[key]
	if ok {
		delete(sm.instances, key)
	}
	sm.mu.Unlock()
	if !ok {
		return nil
	}
	return sm.stopInstance(instance)
}

func (sm *SidecarManager) stopInstance(instance *managedSidecar) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := instance.server.Shutdown(ctx)
	if handler := instance.handler.Load(); handler != nil {
		handler.Close()
	}
	if shutdownErr != nil {
		return fmt.Errorf("shut down sidecar for %s release %s: %w", instance.appName, instance.release, shutdownErr)
	}
	sm.logger.Printf("[gosidecar] stopped sidecar for %s release %s (port %d)", instance.appName, instance.release, instance.port)
	return nil
}

func (sm *SidecarManager) Shutdown() {
	sm.mu.Lock()
	instances := make([]*managedSidecar, 0, len(sm.instances))
	for key, instance := range sm.instances {
		instances = append(instances, instance)
		delete(sm.instances, key)
	}
	sm.mu.Unlock()
	for _, instance := range instances {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := instance.server.Shutdown(ctx); err != nil {
			sm.logger.Printf("[gosidecar] shutdown %s release %s: %v", instance.appName, instance.release, err)
		}
		cancel()
		if handler := instance.handler.Load(); handler != nil {
			handler.Close()
		}
	}
}
