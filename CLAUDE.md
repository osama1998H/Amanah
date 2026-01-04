# CLAUDE.md - AI Assistant Guide for Amanah

This document provides essential context for AI assistants working with the Amanah codebase.

## Project Overview

**Amanah** (Arabic: "trust" or "something entrusted for safekeeping") is a microservices-based payment system framework built in Go. It's designed for building Payment Service Providers (PSPs), fintech applications, and e-commerce platforms.

**Current Status:** Early-stage MVP with foundational architecture in place. Core authentication service is implemented; other services are scaffolded with placeholders.

## Quick Reference

| Aspect | Value |
|--------|-------|
| Language | Go 1.23+ |
| Module | `amanah` |
| Build | `go build ./...` |
| Test | `go test ./...` |
| Lint | `golangci-lint run` |
| Format | `gofmt -w .` |
| Run locally | `docker-compose up --build` |

## Codebase Structure

```
/home/user/Amanah/
├── services/           # Microservices (each independently deployable)
│   ├── api-gateway/    # Request routing, reverse proxy (port 8000)
│   ├── authentication/ # User login, token management (port 8080)
│   ├── transaction/    # Payment lifecycle management (port 8081)
│   ├── ledger/         # Immutable transaction recording
│   ├── notification/   # Email/SMS/push/webhook notifications
│   └── example/        # Reference implementation patterns
│
├── libs/               # Shared libraries used across services
│   ├── auth/           # Token generation and validation
│   ├── logging/        # Structured logging wrapper
│   ├── metrics/        # In-memory metrics collection
│   └── utils/          # General-purpose utilities
│
├── integrations/       # External payment gateway adapters
│   ├── stripe/         # Stripe API client (placeholder)
│   └── paypal/         # PayPal API client (placeholder)
│
├── configs/            # Environment-specific configurations
│   ├── development.yaml
│   ├── staging.yaml
│   └── production.yaml
│
├── deployments/        # Infrastructure as Code
│   ├── docker-compose/ # Local development orchestration
│   ├── kubernetes/     # K8s manifests and kustomization
│   └── terraform/      # Cloud infrastructure (placeholder)
│
├── docs/               # Project documentation
│   ├── architecture.md # System design overview
│   ├── api.md          # API specifications (OpenAPI/gRPC)
│   ├── setup.md        # Installation instructions
│   └── security.md     # Security policies and reporting
│
├── tests/              # Test directories
│   ├── e2e/            # End-to-end tests (empty)
│   └── integration/    # Integration tests (empty)
│
├── scripts/            # Automation scripts
│   ├── db-migrate.sh   # Database migrations
│   └── cleanup.sh      # Cleanup utilities
│
├── .github/workflows/  # CI/CD pipelines
│   ├── build.yml       # Main build pipeline
│   └── create_release.yml
│
├── go.mod              # Go module definition
├── Dockerfile          # Multi-stage Docker build
├── docker-compose.yml  # Local development setup
└── CONTRIBUTING.md     # Contribution guidelines
```

## Service Architecture

Each microservice follows a consistent internal structure:

```
services/<service-name>/
├── cmd/
│   └── main.go         # Entry point, server setup
├── handlers/           # HTTP request handlers
├── services/           # Business logic layer
├── repositories/       # Data access layer
├── models/             # Data models/entities
├── internal/           # Private packages
├── pkg/                # Public packages
├── configs/            # Service-specific config
├── tests/              # Unit tests
└── Dockerfile          # Service container build
```

## Development Workflows

### Running Locally

**With Docker (recommended):**
```bash
docker-compose up --build
```

**Without Docker:**
```bash
go run ./...
```

### Building

```bash
# Build all services
go build ./...

# Build specific service
go build ./services/authentication/cmd/...
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Run benchmarks
go test -bench=. ./...
```

### Linting

```bash
# Format code
gofmt -w .

# Static analysis
go vet ./...

# Full linting (requires golangci-lint)
golangci-lint run
```

## Coding Conventions

### Go Idioms
- Follow idiomatic Go naming: `CamelCase` for exported, `camelCase` for unexported
- Package names are lowercase, single words preferred
- Constructor functions: `New<Type>()` pattern
- Interface names: typically nouns or `-er` suffix (e.g., `Reader`, `Handler`)

### Error Handling
- Always check and handle errors
- Return errors up the call stack; don't panic
- Use descriptive error messages

### HTTP Handlers
- Use `net/http.ServeMux` for routing
- Handler signature: `func(w http.ResponseWriter, r *http.Request)`
- Return appropriate HTTP status codes
- Use `encoding/json` for JSON marshaling

### Concurrency
- Use `sync.Mutex` for shared state protection
- Always `defer mutex.Unlock()` after `Lock()`
- Prefer channels for communication between goroutines

### File Organization
- Keep functions focused and small
- Group related functionality in same package
- Use `internal/` for non-exported packages

## Key Patterns

### Shared Libraries Usage

**Authentication (libs/auth):**
```go
import "amanah/libs/auth"

manager := auth.NewManager()
token := manager.GenerateToken(userID)
isValid := manager.ValidateToken(token)
manager.RevokeToken(token)
```

**Logging (libs/logging):**
```go
import "amanah/libs/logging"

logging.Info("Operation completed")
logging.Error("Operation failed: %v", err)
```

**Metrics (libs/metrics):**
```go
import "amanah/libs/metrics"

counter := metrics.NewCounter()
counter.Inc("requests_total")
value := counter.Get("requests_total")
```

### Service Communication
- Services communicate via HTTP REST APIs
- API Gateway routes requests to appropriate services
- Future: Message queues for async communication

## CI/CD Pipeline

The GitHub Actions workflow (`.github/workflows/build.yml`) runs on every push:

1. **Static Analysis** - `golangci-lint run`
2. **Testing** - `go test` with coverage reporting
3. **Build** - Compile to `build/app`
4. **Docker** - Build and push to Docker Hub
5. **Benchmarks** - Performance testing (parallel with tests)
6. **Notify** - Failure notifications

**Required Secrets:**
- `DOCKER_USERNAME` / `DOCKER_PASSWORD` - Docker Hub credentials
- `PAT_TOKEN` - GitHub token for releases

## Service Ports

| Service | Default Port |
|---------|--------------|
| API Gateway | 8000 |
| Authentication | 8080 |
| Transaction | 8081 |
| Ledger | 8081 |
| Notification | 8081 |
| Redis | 6379 |

## Common AI Assistant Tasks

### Adding a New Endpoint

1. Add handler in `services/<service>/handlers/`
2. Register route in the service's `main.go`
3. Implement business logic in `services/<service>/services/`
4. Add tests in `services/<service>/tests/`

### Adding a New Service

1. Create directory structure under `services/<new-service>/`
2. Implement `cmd/main.go` with HTTP server
3. Add Dockerfile for the service
4. Update `docker-compose.yml` if needed
5. Add Kubernetes manifests in `deployments/kubernetes/`

### Adding a New Shared Library

1. Create package under `libs/<new-lib>/`
2. Implement with proper exports
3. Add tests
4. Import as `amanah/libs/<new-lib>`

### Modifying Configuration

- Environment configs: `configs/*.yaml`
- Service-specific: `services/<service>/configs/`
- Docker: `docker-compose.yml` or service Dockerfiles

## Security Considerations

- **Never commit secrets** - Use environment variables or secret managers
- **Validate all inputs** - Prevent injection attacks
- **Use TLS** - Encrypt data in transit
- **Authentication required** - Enforce at service boundaries
- **Audit logging** - Log security-relevant events
- **Vulnerability reporting** - osama.muhammed.iq@gmail.com

## Project Roadmap (from todo.md)

### Completed
- [x] Git repository and module setup
- [x] Project structure definition
- [x] Authentication service (basic)
- [x] Shared libraries (auth, logging, metrics)
- [x] API Gateway skeleton
- [x] Docker/Compose support
- [x] GitHub Actions CI/CD

### Pending
- [ ] Transaction management API
- [ ] Account abstraction and KYC
- [ ] Payment routing (Stripe/PayPal integration)
- [ ] Notification delivery system
- [ ] Ledger and double-entry accounting
- [ ] Event-driven messaging
- [ ] Rate limiting and security hardening
- [ ] Integration and E2E tests
- [ ] PCI DSS compliance

## Ralph Wiggum - Autonomous Loops

This project includes the Ralph Wiggum hooks protocol for autonomous, long-running task execution.

### Quick Start

**Start an autonomous loop:**
```bash
# Using the direct script (recommended for AI assistants)
.claude/scripts/start-ralph-loop.sh "Your task description" 20 "DONE"

# Arguments: <prompt> [max_iterations] [completion_promise]
```

**Check loop status:**
```bash
.claude/scripts/ralph-state.sh status
```

**Cancel a loop:**
```bash
.claude/scripts/ralph-state.sh cancel
```

### Available Commands

| Command | Description |
|---------|-------------|
| `/ralph-loop` | Start autonomous loop (slash command) |
| `/cancel-ralph` | Cancel active loop |
| `/ralph-status` | Check loop status |

### How It Works

1. Loop is initialized with a task prompt and iteration limits
2. Claude works on the task until it attempts to stop
3. The Stop hook intercepts and checks completion criteria
4. If not complete, Claude is prompted to continue
5. Loop repeats until criteria met or max iterations reached

### Writing Effective Prompts

```bash
.claude/scripts/start-ralph-loop.sh \
  "Implement feature X. Requirements:
   - Requirement 1
   - Requirement 2
   Run tests after each change. Output COMPLETE when done." \
  30 \
  "COMPLETE"
```

### Files

```
.claude/
├── hooks/
│   ├── stop-hook.sh              # Intercepts stop attempts
│   └── user-prompt-submit-hook.sh # Injects loop context
├── scripts/
│   ├── ralph-state.sh            # State management
│   ├── start-ralph-loop.sh       # Direct loop initiation
│   └── check-ralph-continue.sh   # Check continuation status
├── commands/
│   ├── ralph-loop.md             # /ralph-loop command
│   ├── cancel-ralph.md           # /cancel-ralph command
│   └── ralph-status.md           # /ralph-status command
└── settings.json                 # Hook configuration
```

## Tips for AI Assistants

1. **Read before modifying** - Always read existing code before suggesting changes
2. **Follow existing patterns** - Match the established code style and architecture
3. **Keep it simple** - Avoid over-engineering; make minimal necessary changes
4. **Test changes** - Run `go test ./...` and `go build ./...` to verify
5. **Use shared libs** - Leverage existing libraries in `libs/` before creating new ones
6. **Check TODOs** - Review `todo.md` for project priorities
7. **Document APIs** - Update `docs/api.md` when adding endpoints
8. **Security first** - Consider security implications of all changes
9. **Use ralph-loop for large tasks** - For multi-step implementations, use the autonomous loop
