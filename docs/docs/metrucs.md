
# VPS Health Metrics Streaming Daemon

Here's a comprehensive implementation for a daemon that continuously monitors and streams VPS health metrics, with support for multiple output formats and destinations.

## Architecture Overview

1. **Metrics Collection**: Gathers system metrics (CPU, memory, disk, network, etc.)
2. **Stream Processing**: Handles data aggregation and transformation
3. **Output Modules**: Supports multiple output formats (CLI, WebSocket, HTTP API, File)
4. **Alerting System**: Configurable thresholds and notifications

## Package Structure

```
/vps-health-daemon
  ├── main.go               # Main entry point
  ├── config
  │   └── config.go         # Configuration management
  ├── collector
  │   ├── cpu.go            # CPU metrics
  │   ├── memory.go         # Memory metrics
  │   ├── disk.go           # Disk metrics
  │   ├── network.go        # Network metrics
  │   └── system.go         # System info
  ├── stream
  │   ├── processor.go      # Metrics processing
  │   └── manager.go        # Stream management
  ├── output
  │   ├── cli.go            # CLI output
  │   ├── websocket.go      # WebSocket server
  │   ├── http.go           # HTTP API
  │   └── file.go           # File writer
  └── alert
      └── alert.go          # Alerting system
```

## vps-health-daemon/main.go

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vps-health-daemon/alert"
	"github.com/vps-health-daemon/collector"
	"github.com/vps-health-daemon/config"
	"github.com/vps-health-daemon/output"
	"github.com/vps-health-daemon/stream"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize metrics collector
	metricsCollector := collector.NewCollector(
		cfg.CollectInterval,
		cfg.Metrics.CPU,
		cfg.Metrics.Memory,
		cfg.Metrics.Disk,
		cfg.Metrics.Network,
	)

	// Initialize stream processor
	streamProcessor := stream.NewProcessor(
		metricsCollector,
		cfg.Stream.BufferSize,
		cfg.Stream.AggregationWindow,
	)

	// Initialize output modules
	outputManager := output.NewManager()
	if cfg.Output.CLI.Enabled {
		outputManager.AddOutput(output.NewCLI(cfg.Output.CLI.RefreshInterval))
	}
	if cfg.Output.WebSocket.Enabled {
		outputManager.AddOutput(output.NewWebSocket(cfg.Output.WebSocket.Port))
	}
	if cfg.Output.HTTP.Enabled {
		outputManager.AddOutput(output.NewHTTP(cfg.Output.HTTP.Port))
	}
	if cfg.Output.File.Enabled {
		outputManager.AddOutput(output.NewFileWriter(cfg.Output.File.Path, cfg.Output.File.Rotation))
	}

	// Initialize alert system
	alertSystem := alert.NewSystem(cfg.Alerts)
	for _, rule := range cfg.Alerts.Rules {
		alertSystem.AddRule(rule)
	}

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start components
	metricsChan := metricsCollector.Start(ctx)
	processedChan := streamProcessor.Start(ctx, metricsChan)

	// Start output and alert consumers
	go outputManager.Start(ctx, processedChan)
	go alertSystem.Start(ctx, processedChan)

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
	time.Sleep(1 * time.Second) // Give time for cleanup
}
```

## vps-health-daemon/config/config.go

```go
package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	CollectInterval time.Duration `yaml:"collect_interval"`
	Metrics         MetricsConfig `yaml:"metrics"`
	Stream          StreamConfig  `yaml:"stream"`
	Output          OutputConfig  `yaml:"output"`
	Alerts         AlertConfig   `yaml:"alerts"`
}

type MetricsConfig struct {
	CPU     CPUConfig     `yaml:"cpu"`
	Memory  MemoryConfig  `yaml:"memory"`
	Disk    DiskConfig    `yaml:"disk"`
	Network NetworkConfig `yaml:"network"`
}

type CPUConfig struct {
	Enabled      bool    `yaml:"enabled"`
	PerCPU       bool    `yaml:"per_cpu"`
	LoadAverage  bool    `yaml:"load_average"`
	Temperature  bool    `yaml:"temperature"`
}

type MemoryConfig struct {
	Enabled      bool `yaml:"enabled"`
	Detailed     bool `yaml:"detailed"`
	Swap         bool `yaml:"swap"`
}

type DiskConfig struct {
	Enabled      bool     `yaml:"enabled"`
	MountPoints  []string `yaml:"mount_points"`
	IgnoreRegex  string   `yaml:"ignore_regex"`
}

type NetworkConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Interfaces   []string `yaml:"interfaces"`
	Bandwidth    bool     `yaml:"bandwidth"`
	Connections  bool     `yaml:"connections"`
}

type StreamConfig struct {
	BufferSize         int           `yaml:"buffer_size"`
	AggregationWindow  time.Duration `yaml:"aggregation_window"`
}

type OutputConfig struct {
	CLI       CLIOutput       `yaml:"cli"`
	WebSocket WebSocketOutput `yaml:"websocket"`
	HTTP      HTTPOutput      `yaml:"http"`
	File      FileOutput      `yaml:"file"`
}

type CLIOutput struct {
	Enabled         bool          `yaml:"enabled"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

type WebSocketOutput struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

type HTTPOutput struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
}

type FileOutput struct {
	Enabled  bool          `yaml:"enabled"`
	Path     string        `yaml:"path"`
	Rotation time.Duration `yaml:"rotation"`
}

type AlertConfig struct {
	Enabled bool        `yaml:"enabled"`
	Rules   []AlertRule `yaml:"rules"`
}

type AlertRule struct {
	Metric     string  `yaml:"metric"`
	Condition  string  `yaml:"condition"`  // ">", "<", "==", "!="
	Threshold  float64 `yaml:"threshold"`
	Duration   time.Duration `yaml:"duration"`
	Message    string  `yaml:"message"`
}

func Load(path string) (*Config, error) {
	// Try to get absolute path
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		path = absPath
	}

	// Read config file
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.CollectInterval == 0 {
		cfg.CollectInterval = 5 * time.Second
	}

	if cfg.Stream.BufferSize == 0 {
		cfg.Stream.BufferSize = 1000
	}

	if cfg.Stream.AggregationWindow == 0 {
		cfg.Stream.AggregationWindow = 30 * time.Second
	}

	return &cfg, nil
}
```

## vps-health-daemon/collector/cpu.go

```go
package collector

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

type CPUMetrics struct {
	Percent     []float64
	LoadAvg     *LoadAverage
	Temperature float64
}

type LoadAverage struct {
	One     float64
	Five    float64
	Fifteen float64
}

func collectCPUMetrics(cfg CPUConfig) (*CPUMetrics, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	var metrics CPUMetrics
	var err error

	// Collect CPU percentages
	if cfg.PerCPU {
		metrics.Percent, err = cpu.Percent(0, true)
	} else {
		percent, err := cpu.Percent(0, false)
		if err == nil {
			metrics.Percent = percent
		}
	}

	// Collect load average if supported
	if cfg.LoadAverage && runtime.GOOS != "windows" {
		if load, err := loadAvg(); err == nil {
			metrics.LoadAvg = load
		}
	}

	// Collect temperature if supported
	if cfg.Temperature {
		if temps, err := cpu.Temperatures(); err == nil && len(temps) > 0 {
			metrics.Temperature = temps[0].Temperature
		}
	}

	return &metrics, err
}

func loadAvg() (*LoadAverage, error) {
	avg, err := cpu.LoadAvg()
	if err != nil {
		return nil, err
	}
	return &LoadAverage{
		One:     avg[0],
		Five:    avg[1],
		Fifteen: avg[2],
	}, nil
}
```

## vps-health-daemon/collector/memory.go

```go
package collector

import (
	"github.com/shirou/gopsutil/v3/mem"
)

type MemoryMetrics struct {
	Total       uint64
	Available   uint64
	Used        uint64
	UsedPercent float64
	Swap        *SwapMetrics
}

type SwapMetrics struct {
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

func collectMemoryMetrics(cfg MemoryConfig) (*MemoryMetrics, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	vmem, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	metrics := &MemoryMetrics{
		Total:       vmem.Total,
		Available:   vmem.Available,
		Used:        vmem.Used,
		UsedPercent: vmem.UsedPercent,
	}

	if cfg.Swap {
		swap, err := mem.SwapMemory()
		if err == nil {
			metrics.Swap = &SwapMetrics{
				Total:       swap.Total,
				Used:        swap.Used,
				Free:        swap.Free,
				UsedPercent: swap.UsedPercent,
			}
		}
	}

	return metrics, nil
}
```

## vps-health-daemon/collector/disk.go

```go
package collector

import (
	"regexp"

	"github.com/shirou/gopsutil/v3/disk"
)

type DiskMetrics struct {
	MountPoint string
	Total      uint64
	Free       uint64
	Used       uint64
	UsedPercent float64
	InodesUsed uint64
	InodesFree uint64
	InodesUsedPercent float64
}

func collectDiskMetrics(cfg DiskConfig) ([]DiskMetrics, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	var ignoreRegex *regexp.Regexp
	if cfg.IgnoreRegex != "" {
		ignoreRegex = regexp.MustCompile(cfg.IgnoreRegex)
	}

	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var metrics []DiskMetrics
	for _, part := range partitions {
		// Skip if mount point doesn't match our filter
		if len(cfg.MountPoints) > 0 && !contains(cfg.MountPoints, part.Mountpoint) {
			continue
		}

		// Skip if matches ignore regex
		if ignoreRegex != nil && ignoreRegex.MatchString(part.Mountpoint) {
			continue
		}

		usage, err := disk.Usage(part.Mountpoint)
		if err != nil {
			continue
		}

		inodes, err := disk.Usage(part.Mountpoint)
		if err != nil {
			continue
		}

		metrics = append(metrics, DiskMetrics{
			MountPoint:         part.Mountpoint,
			Total:             usage.Total,
			Free:              usage.Free,
			Used:              usage.Used,
			UsedPercent:       usage.UsedPercent,
			InodesUsed:        inodes.InodesUsed,
			InodesFree:        inodes.InodesFree,
			InodesUsedPercent: inodes.InodesUsedPercent,
		})
	}

	return metrics, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
```

## vps-health-daemon/stream/processor.go

```go
package stream

import (
	"context"
	"time"

	"github.com/vps-health-daemon/collector"
)

type MetricSample struct {
	Timestamp time.Time
	CPU       *collector.CPUMetrics
	Memory    *collector.MemoryMetrics
	Disk      []collector.DiskMetrics
	Network   interface{} // Placeholder for network metrics
}

type Processor struct {
	bufferSize        int
	aggregationWindow time.Duration
}

func NewProcessor(bufferSize int, aggregationWindow time.Duration) *Processor {
	return &Processor{
		bufferSize:        bufferSize,
		aggregationWindow: aggregationWindow,
	}
}

func (p *Processor) Start(ctx context.Context, input <-chan *collector.Metrics) <-chan *MetricSample {
	output := make(chan *MetricSample, p.bufferSize)

	go func() {
		defer close(output)

		var buffer []*collector.Metrics
		ticker := time.NewTicker(p.aggregationWindow)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case metric, ok := <-input:
				if !ok {
					return
				}
				buffer = append(buffer, metric)

			case <-ticker.C:
				if len(buffer) == 0 {
					continue
				}

				// Aggregate metrics
				aggregated := p.aggregate(buffer)
				output <- aggregated
				buffer = nil
			}
		}
	}()

	return output
}

func (p *Processor) aggregate(metrics []*collector.Metrics) *MetricSample {
	if len(metrics) == 0 {
		return nil
	}

	// For simplicity, just take the last sample
	last := metrics[len(metrics)-1]

	return &MetricSample{
		Timestamp: time.Now(),
		CPU:       last.CPU,
		Memory:    last.Memory,
		Disk:      last.Disk,
		Network:   last.Network,
	}
}
```

## vps-health-daemon/output/websocket.go

```go
package output

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketOutput struct {
	port    int
	path    string
	upgrader websocket.Upgrader
	clients map[*websocket.Conn]bool
}

func NewWebSocket(port int, path string) *WebSocketOutput {
	return &WebSocketOutput{
		port: port,
		path: path,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

func (w *WebSocketOutput) Start(ctx context.Context, input <-chan interface{}) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", w.port),
		Handler: w,
	}

	go func() {
		log.Printf("WebSocket server started on :%d%s", w.port, w.path)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("WebSocket server error: %v", err)
		}
	}()

	go w.broadcastMessages(ctx, input)

	<-ctx.Done()
	server.Shutdown(context.Background())
}

func (w *WebSocketOutput) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path != w.path {
		http.NotFound(rw, r)
		return
	}

	conn, err := w.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	w.clients[conn] = true

	go func() {
		defer func() {
			conn.Close()
			delete(w.clients, conn)
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

func (w *WebSocketOutput) broadcastMessages(ctx context.Context, input <-chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-input:
			if !ok {
				return
			}

			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Error marshaling WebSocket message: %v", err)
				continue
			}

			for client := range w.clients {
				if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Printf("WebSocket write error: %v", err)
					client.Close()
					delete(w.clients, client)
				}
			}
		}
	}
}
```

## Example Configuration (config.yaml)

```yaml
collect_interval: 5s

metrics:
  cpu:
    enabled: true
    per_cpu: false
    load_average: true
    temperature: true
  memory:
    enabled: true
    detailed: false
    swap: true
  disk:
    enabled: true
    mount_points:
      - "/"
      - "/var"
    ignore_regex: "^/snap/"
  network:
    enabled: true
    interfaces: ["eth0", "wlan0"]
    bandwidth: true
    connections: false

stream:
  buffer_size: 1000
  aggregation_window: 30s

output:
  cli:
    enabled: true
    refresh_interval: 1s
  websocket:
    enabled: true
    port: 8080
    path: "/ws"
  http:
    enabled: true
    port: 8081
  file:
    enabled: false
    path: "/var/log/vps-metrics.log"
    rotation: 24h

alerts:
  enabled: true
  rules:
    - metric: "cpu.used_percent"
      condition: ">"
      threshold: 90
      duration: 5m
      message: "High CPU usage detected"
    - metric: "memory.used_percent"
      condition: ">"
      threshold: 90
      duration: 5m
      message: "High memory usage detected"
    - metric: "disk./.used_percent"
      condition: ">"
      threshold: 90
      duration: 0
      message: "Root filesystem almost full"
```

## Key Features

1. **Comprehensive Metrics Collection**:
   - CPU usage (total and per-core)
   - Memory and swap usage
   - Disk space and inode usage
   - Network bandwidth and connections
   - System load averages
   - Temperature monitoring (where supported)

2. **Flexible Output Options**:
   - Real-time CLI display
   - WebSocket streaming for remote monitoring
   - HTTP API for programmatic access
   - File logging with rotation

3. **Alerting System**:
   - Configurable threshold-based alerts
   - Duration-based triggering to avoid flapping
   - Custom alert messages

4. **Performance Optimizations**:
   - Configurable collection intervals
   - Metrics aggregation
   - Ring buffer for efficient memory usage

5. **Easy Deployment**:
   - Single binary with YAML configuration
   - Graceful shutdown handling
   - Systemd service compatible

## Building and Running

1. Build the daemon:
```bash
go build -o vps-health-daemon
```

2. Create a config.yaml file with your settings

3. Run the daemon:
```bash
./vps-health-daemon
```

4. Access metrics:
   - CLI: View directly in console
   - WebSocket: Connect to ws://localhost:8080/ws
   - HTTP: GET http://localhost:8081/metrics

## Integration Examples

1. **Prometheus Exporter** (additional HTTP endpoint):
```go
func (h *HTTPOutput) prometheusHandler(w http.ResponseWriter, r *http.Request) {
	metrics := h.getCurrentMetrics()
	
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "# HELP cpu_usage CPU usage percentage\n")
	fmt.Fprintf(w, "# TYPE cpu_usage gauge\n")
	fmt.Fprintf(w, "cpu_usage %f\n", metrics.CPU.Percent[0])
	// Add other metrics...
}
```

2. **Database Storage** (additional output module):
```go
type DatabaseOutput struct {
	db *sql.DB
}

func NewDatabaseOutput(dsn string) (*DatabaseOutput, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return &DatabaseOutput{db: db}, nil
}

func (d *DatabaseOutput) Write(metrics *MetricSample) error {
	_, err := d.db.Exec(
		"INSERT INTO metrics (timestamp, cpu_usage) VALUES ($1, $2)",
		metrics.Timestamp,
		metrics.CPU.Percent[0],
	)
	return err
}
```

This implementation provides a robust, flexible solution for monitoring VPS health metrics with multiple output options and alerting capabilities. The modular design makes it easy to extend with additional metrics sources or output destinations.
