# Best Practices for Building a Two-Way Communication Daemon in Go

Here are the best practices for building a robust daemon in Go that facilitates two-way communication between a Next.js frontend and a Go CLI:

## 1. Architecture Design

### Communication Flow
```
Next.js Frontend ↔ Go Daemon ↔ Go CLI
```

### Recommended Patterns
- Use a client-server model where the daemon acts as the central hub
- Implement pub/sub pattern for event-driven communication
- Consider using gRPC for efficient internal communication

## 2. Inter-process Communication (IPC) Methods

### Between Next.js and Go Daemon:
- **REST API** (simple to implement, good for request/response)
  - Use HTTPS with proper authentication
  - Implement OpenAPI/Swagger for documentation
- **WebSockets** (for real-time, bidirectional communication)
  - Ideal for push notifications from daemon to frontend
- **gRPC-Web** (high performance, type-safe)

### Between CLI and Go Daemon:
- **Unix Domain Sockets** (fast, secure for local communication)
- **gRPC** (efficient, type-safe, supports streaming)
- **Custom TCP protocol** (if you need cross-machine communication)

## 3. Daemon Implementation Best Practices

### Core Daemon Structure
```go
package main

import (
    "context"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    // Context for graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Set up signal handling for graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    // Initialize components
    frontendServer := initFrontendServer()
    cliServer := initCLIServer()
    messageBus := initMessageBus()
    
    // Start servers
    go frontendServer.Run(ctx)
    go cliServer.Run(ctx)
    
    // Wait for shutdown signal
    <-sigChan
    log.Println("Shutting down gracefully...")
    
    // Cleanup resources
    frontendServer.Shutdown()
    cliServer.Shutdown()
    messageBus.Close()
}
```

## 4. Security Considerations

- Implement proper authentication for all endpoints
- Use TLS for all network communications
- Validate all inputs from both frontend and CLI
- Implement rate limiting to prevent abuse
- Use minimal required privileges for the daemon process

## 5. Error Handling & Logging

- Implement structured logging (use packages like `log/slog` or `zap`)
- Centralize error handling with appropriate error codes
- Implement health check endpoints
- Consider distributed tracing for debugging

## 6. Performance Considerations

- Use connection pooling for database/network connections
- Implement proper backpressure handling
- Profile memory usage (especially for long-running processes)
- Consider using worker pools for CPU-intensive tasks

## 7. Deployment & Maintenance

- Create proper systemd/launchd service files
- Implement configuration management (env vars, config files)
- Version your API/protocol for backward compatibility
- Set up monitoring and alerting

## 8. Example Communication Patterns

### Frontend to Daemon (REST API example)
```go
// In your daemon
func initFrontendServer() *http.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
        // Authenticate
        // Process request
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "ok",
            "data":   getStatusData(),
        })
    })
    
    return &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }
}
```

### CLI to Daemon (gRPC example)
```go
// protobuf definition
service DaemonService {
    rpc ExecuteCommand (CommandRequest) returns (CommandResponse);
    rpc StreamEvents (Empty) returns (stream Event);
}

// In your daemon
func initCLIServer() *grpc.Server {
    srv := grpc.NewServer()
    pb.RegisterDaemonServiceServer(srv, &daemonServer{})
    return srv
}
```

## 9. Testing Strategies

- Unit test core business logic
- Integration tests for communication protocols
- End-to-end tests for full workflow
- Load testing for performance validation

## 10. Documentation

- Document your API/protocol specifications
- Create developer guides for both frontend and CLI integration
- Maintain a changelog for protocol/API changes

By following these best practices, you'll create a robust, maintainable daemon that effectively bridges your Next.js frontend and Go CLI application.
# Advanced Best Practices & Design Principles for Daemon Development

Building on the foundational practices, here are more advanced guidelines following SOLID principles, clean architecture, and other proven design patterns:

## 1. SOLID Principles Implementation

### Single Responsibility Principle
- **Separate concerns** into distinct packages:
  ```
  /internal
    /transport  # communication protocols
    /service    # business logic
    /repository # data access
    /domain     # core models
  ```

### Open/Closed Principle
- Use interfaces for dependencies:
```go
type CommandHandler interface {
    Handle(ctx context.Context, cmd Command) (Response, error)
}

type EventPublisher interface {
    Publish(ctx context.Context, event Event) error
    Subscribe(ctx context.Context, topic string) (<-chan Event, error)
}
```

### Liskov Substitution
- Design interfaces that can have multiple implementations:
```go
type Storage interface {
    Get(key string) ([]byte, error)
    Set(key string, value []byte) error
    // Not: Connect() or Close() - those belong to a different interface
}
```

## 2. Clean Architecture Layers

### Domain Layer (Innermost)
```go
// domain/event.go
package domain

type Event struct {
    ID        string
    Type      string
    Timestamp time.Time
    Payload   interface{}
}

// domain/repository.go
type EventRepository interface {
    Save(ctx context.Context, event Event) error
    FindByType(ctx context.Context, eventType string) ([]Event, error)
}
```

### Use Case Layer
```go
// service/event_service.go
package service

type EventService struct {
    repo    domain.EventRepository
    pubsub  PubSub
}

func (s *EventService) ProcessEvent(ctx context.Context, e domain.Event) error {
    if err := s.repo.Save(ctx, e); err != nil {
        return fmt.Errorf("saving event: %w", err)
    }
    
    if err := s.pubsub.Publish(ctx, e); err != nil {
        return fmt.Errorf("publishing event: %w", err)
    }
    
    return nil
}
```

## 3. Dependency Injection

- Use wire or dig for dependency management:
```go
// wire.go
func InitializeEventService() (*service.EventService, error) {
    wire.Build(
        postgres.NewEventRepository,
        nats.NewPubSub,
        service.NewEventService,
    )
    return &service.EventService{}, nil
}
```

## 4. Event-Driven Architecture

### Event Bus Implementation
```go
type EventBus struct {
    mu     sync.RWMutex
    subs   map[string][]chan<- Event
}

func (b *EventBus) Subscribe(topic string) <-chan Event {
    ch := make(chan Event, 100) // Buffered channel
    b.mu.Lock()
    defer b.mu.Unlock()
    b.subs[topic] = append(b.subs[topic], ch)
    return ch
}

func (b *EventBus) Publish(ctx context.Context, event Event) error {
    b.mu.RLock()
    defer b.mu.RUnlock()
    
    for _, ch := range b.subs[event.Type] {
        select {
        case ch <- event:
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Handle channel full scenario
        }
    }
    return nil
}
```

## 5. CQRS Pattern

Separate command and query responsibilities:
```go
// Command side
type CommandService struct {
    repo domain.Repository
    bus  EventBus
}

func (s *CommandService) HandleCreateUser(cmd CreateUserCommand) error {
    // Validate command
    // Create user aggregate
    // Persist
    // Publish events
}

// Query side
type UserQueryService struct {
    cache Cache
    db    ReadDB
}

func (s *UserQueryService) GetUserByID(id string) (UserView, error) {
    // Check cache first
    // Fallback to DB
    // Return DTO optimized for reading
}
```

## 6. Circuit Breaker Pattern

```go
type CircuitBreaker struct {
    maxFailures int
    timeout     time.Duration
    lastFailure time.Time
    failureCount int
    mu          sync.Mutex
}

func (cb *CircuitBreaker) Execute(f func() error) error {
    cb.mu.Lock()
    
    if cb.failureCount >= cb.maxFailures {
        if time.Since(cb.lastFailure) < cb.timeout {
            cb.mu.Unlock()
            return ErrCircuitOpen
        }
        // Timeout expired, try again
        cb.failureCount = 0
    }
    
    cb.mu.Unlock()
    
    err := f()
    
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if err != nil {
        cb.failureCount++
        cb.lastFailure = time.Now()
    } else {
        cb.failureCount = 0
    }
    
    return err
}
```

## 7. State Machine Pattern

For complex workflows:
```go
type State string

const (
    StateIdle      State = "idle"
    StateProcessing State = "processing"
    StateWaiting    State = "waiting"
)

type StateMachine struct {
    current State
    mu      sync.Mutex
}

func (sm *StateMachine) Transition(to State) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    switch sm.current {
    case StateIdle:
        if to != StateProcessing {
            return ErrInvalidTransition
        }
    case StateProcessing:
        if to != StateWaiting && to != StateIdle {
            return ErrInvalidTransition
        }
    case StateWaiting:
        if to != StateProcessing {
            return ErrInvalidTransition
        }
    default:
        return ErrUnknownState
    }
    
    sm.current = to
    return nil
}
```

## 8. Middleware Pattern

For cross-cutting concerns:
```go
type Middleware func(Handler) Handler

func LoggingMiddleware(next Handler) Handler {
    return func(ctx context.Context, req Request) (Response, error) {
        start := time.Now()
        log.Printf("Starting request: %v", req)
        
        resp, err := next(ctx, req)
        
        log.Printf("Completed request in %v: err=%v", time.Since(start), err)
        return resp, err
    }
}

func Chain(middlewares ...Middleware) Middleware {
    return func(final Handler) Handler {
        for i := len(middlewares) - 1; i >= 0; i-- {
            final = middlewares[i](final)
        }
        return final
    }
}
```

## 9. Configuration Management

```go
type Config struct {
    HTTP struct {
        Addr         string        `env:"HTTP_ADDR" envDefault:":8080"`
        ReadTimeout  time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"5s"`
        WriteTimeout time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"10s"`
    }
    
    Database struct {
        URL string `env:"DB_URL" envDefault:"postgres://localhost:5432/daemon"`
    }
    
    NATS struct {
        URL string `env:"NATS_URL" envDefault:"nats://localhost:4222"`
    }
}

func LoadConfig() (*Config, error) {
    var cfg Config
    if err := env.Parse(&cfg); err != nil {
        return nil, fmt.Errorf("parsing config: %w", err)
    }
    return &cfg, nil
}
```

## 10. Observability

### Structured Logging
```go
type Logger interface {
    Debug(ctx context.Context, msg string, fields ...Field)
    Info(ctx context.Context, msg string, fields ...Field)
    Error(ctx context.Context, msg string, fields ...Field)
}

type Field struct {
    Key   string
    Value interface{}
}

// Example usage
logger.Info(ctx, "processing request",
    Field{Key: "request_id", Value: requestID},
    Field{Key: "duration_ms", Value: duration.Milliseconds()},
)
```

### Distributed Tracing
```go
func InstrumentedHandler(tracer trace.Tracer, next Handler) Handler {
    return func(ctx context.Context, req Request) (Response, error) {
        ctx, span := tracer.Start(ctx, "handler")
        defer span.End()
        
        // Add request details to span
        span.SetAttributes(
            attribute.String("request.id", req.ID),
            attribute.String("request.type", req.Type),
        )
        
        return next(ctx, req)
    }
}
```

## 11. Testing Patterns

### Table-Driven Tests
```go
func TestCommandHandler(t *testing.T) {
    tests := []struct {
        name        string
        input       Command
        setup       func(*MockRepository)
        expected    Response
        expectedErr error
    }{
        {
            name:  "successful command",
            input: Command{ID: "123"},
            setup: func(mr *MockRepository) {
                mr.On("Save", mock.Anything, mock.Anything).Return(nil)
            },
            expected: Response{Status: "ok"},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &MockRepository{}
            if tt.setup != nil {
                tt.setup(repo)
            }
            
            handler := NewCommandHandler(repo)
            result, err := handler.Handle(context.Background(), tt.input)
            
            assert.Equal(t, tt.expectedErr, err)
            assert.Equal(t, tt.expected, result)
            repo.AssertExpectations(t)
        })
    }
}
```

### Golden File Testing for Protocols
```go
func TestProtocolMessages(t *testing.T) {
    testCases := []struct {
        name string
        msg  Message
    }{
        {"status_request", StatusRequest{RequestID: "abc123"}},
        {"data_response", DataResponse{Data: []byte("test")}},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            goldfile := filepath.Join("testdata", tc.name+".golden")
            
            // Encode and compare with golden file
            data, err := encodeMessage(tc.msg)
            require.NoError(t, err)
            
            if *update {
                require.NoError(t, os.WriteFile(goldfile, data, 0644))
            }
            
            expected, err := os.ReadFile(goldfile)
            require.NoError(t, err)
            require.Equal(t, string(expected), string(data))
        })
    }
}
```

## 12. Performance Optimization

### Object Pool Pattern
```go
type BufferPool struct {
    pool sync.Pool
}

func NewBufferPool(defaultSize int) *BufferPool {
    return &BufferPool{
        pool: sync.Pool{
            New: func() interface{} {
                return bytes.NewBuffer(make([]byte, 0, defaultSize))
            },
        },
    }
}

func (p *BufferPool) Get() *bytes.Buffer {
    return p.pool.Get().(*bytes.Buffer)
}

func (p *BufferPool) Put(b *bytes.Buffer) {
    b.Reset()
    p.pool.Put(b)
}
```

These advanced practices will help you build a daemon that is:
- Highly maintainable through clear separation of concerns
- Extensible through proper abstraction
- Reliable through robust error handling
- Observable through comprehensive instrumentation
- Performant through careful resource management

The key is to balance these patterns with your specific requirements - not every system needs every pattern, but understanding these gives you tools to solve complex architectural challenges.
