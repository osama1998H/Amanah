# Amanah Payment Gateway

A production-ready, enterprise-grade payment gateway framework built with Go. Designed for high availability, security, and scalability in financial transaction processing.

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![CI/CD](https://github.com/osama1998H/Amanah/actions/workflows/ci.yml/badge.svg)](https://github.com/osama1998H/Amanah/actions)

## Features

### Core Capabilities
- **Transaction Management** - Full lifecycle management for financial transactions with ACID compliance
- **Account Abstraction** - Comprehensive account management with multi-currency support
- **Payment Routing** - Intelligent routing across multiple payment providers with failover
- **Notification Engine** - Event-driven notifications (email, SMS, push, webhooks)
- **Immutable Ledger** - Audit-compliant transaction logging

### Security
- JWT-based authentication with refresh token rotation
- Role-Based Access Control (RBAC) with granular permissions
- API key management with rate limiting and rotation
- Input validation and SQL injection prevention
- CORS and security headers middleware

### Resilience
- Circuit breaker pattern for external service calls
- Retry with exponential backoff and jitter
- Bulkhead pattern for concurrency limiting
- Request timeout management
- Graceful shutdown handling

### Observability
- Structured JSON logging with sensitive data masking
- Prometheus-compatible metrics (counters, gauges, histograms)
- Distributed tracing support (Jaeger/OpenTelemetry ready)
- Health check endpoints (liveness/readiness)

## Quick Start

### Prerequisites
- Go 1.23 or higher
- Docker and Docker Compose
- Make (optional, for convenience commands)

### Local Development

1. **Clone the repository**
   ```bash
   git clone https://github.com/osama1998H/Amanah.git
   cd Amanah
   ```

2. **Start infrastructure services**
   ```bash
   docker-compose up -d postgres redis rabbitmq
   ```

3. **Run the gateway**
   ```bash
   go run ./cmd/gateway
   ```

4. **Verify it's running**
   ```bash
   curl http://localhost:8080/health/live
   ```

### Using Docker Compose (Full Stack)

```bash
# Start all services including monitoring
docker-compose up -d

# View logs
docker-compose logs -f gateway

# Stop all services
docker-compose down
```

### Using Make

```bash
# Download dependencies
make deps

# Run tests
make test

# Build binaries
make build

# Build Docker image
make docker-build

# Start services
make up
```

## Project Structure

```
Amanah/
├── cmd/                    # Application entry points
│   ├── api/               # REST API server
│   └── gateway/           # API gateway server
├── libs/                   # Shared libraries
│   ├── auth/              # JWT, RBAC, API key management
│   ├── cache/             # Caching utilities
│   ├── config/            # Configuration management
│   ├── database/          # Database connections and migrations
│   ├── gateway/           # API gateway, circuit breaker, health checks
│   ├── logging/           # Structured logging
│   ├── metrics/           # Prometheus metrics
│   ├── middleware/        # HTTP middleware (auth, rate limit, security)
│   ├── resilience/        # Retry, timeout, bulkhead patterns
│   └── testing/           # Test utilities, mocks, fixtures
├── services/              # Microservices
│   ├── transaction/       # Transaction management
│   ├── account/           # Account management
│   ├── payment/           # Payment processing
│   ├── notification/      # Notification delivery
│   └── ledger/            # Audit ledger
├── deploy/                # Deployment configurations
│   └── kubernetes/        # Kubernetes manifests
├── configs/               # Configuration files
├── docs/                  # Documentation
├── Dockerfile            # Multi-stage Docker build
├── docker-compose.yml    # Local development stack
├── Makefile              # Build and development commands
└── go.mod                # Go module definition
```

## Configuration

The application is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `ENV` | Environment (development/staging/production) | development |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | info |
| `LOG_FORMAT` | Log format (json/text) | json |
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PORT` | PostgreSQL port | 5432 |
| `DB_NAME` | Database name | amanah |
| `DB_USER` | Database user | amanah |
| `DB_PASSWORD` | Database password | - |
| `DB_SSL_MODE` | SSL mode (disable/require) | disable |
| `REDIS_HOST` | Redis host | localhost |
| `REDIS_PORT` | Redis port | 6379 |
| `REDIS_PASSWORD` | Redis password | - |
| `JWT_SECRET` | JWT signing secret (min 32 chars) | - |
| `API_KEY_SECRET` | API key encryption secret | - |

## API Endpoints

### Health Checks
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe (checks dependencies)

### Metrics
- `GET /metrics` - Prometheus metrics endpoint

### Authentication
- `POST /auth/login` - User login
- `POST /auth/refresh` - Refresh access token
- `POST /auth/logout` - Invalidate tokens

### Transactions
- `POST /api/v1/transactions` - Create transaction
- `GET /api/v1/transactions/:id` - Get transaction
- `GET /api/v1/transactions` - List transactions
- `POST /api/v1/transactions/:id/cancel` - Cancel transaction
- `POST /api/v1/transactions/:id/refund` - Refund transaction

### Accounts
- `POST /api/v1/accounts` - Create account
- `GET /api/v1/accounts/:id` - Get account
- `GET /api/v1/accounts/:id/balance` - Get balance
- `POST /api/v1/accounts/:id/deposit` - Deposit funds
- `POST /api/v1/accounts/:id/withdraw` - Withdraw funds

## Library Usage

### Authentication (libs/auth)

```go
import "amanah/libs/auth"

// JWT Token Management
jwtManager := auth.NewJWTManager(&auth.JWTConfig{
    SecretKey:          []byte("your-secret-key"),
    AccessTokenExpiry:  15 * time.Minute,
    RefreshTokenExpiry: 7 * 24 * time.Hour,
})

// Generate tokens
claims := &auth.Claims{UserID: "user123", Role: "merchant"}
accessToken, _ := jwtManager.GenerateAccessToken(claims)

// Validate token
claims, err := jwtManager.ValidateToken(accessToken)
```

### Rate Limiting (libs/middleware)

```go
import "amanah/libs/middleware"

// Token bucket rate limiter
limiter := middleware.NewRateLimiter(&middleware.RateLimiterConfig{
    RequestsPerSecond: 100,
    BurstSize:         200,
})

// Apply as middleware
http.Handle("/api/", limiter.Middleware(handler))
```

### Circuit Breaker (libs/gateway)

```go
import "amanah/libs/gateway"

cb := gateway.NewCircuitBreaker(&gateway.CircuitBreakerConfig{
    Name:            "payment-service",
    Threshold:       5,
    Timeout:         30 * time.Second,
    HalfOpenMaxReqs: 3,
})

err := cb.Execute(func() error {
    return callExternalService()
})
```

### Retry with Backoff (libs/resilience)

```go
import "amanah/libs/resilience"

config := &resilience.RetryConfig{
    MaxRetries:   3,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    Jitter:       0.1,
}

err := resilience.Retry(ctx, config, func() error {
    return performOperation()
})
```

### Structured Logging (libs/logging)

```go
import "amanah/libs/logging"

logger := logging.NewLogger(&logging.LoggerConfig{
    Level:       logging.LevelInfo,
    Format:      logging.FormatJSON,
    ServiceName: "payment-service",
})

logger.WithField("transaction_id", "txn_123").
    WithField("amount", 100.50).
    Info("Processing payment")
```

### Metrics (libs/metrics)

```go
import "amanah/libs/metrics"

registry := metrics.NewRegistry()

// Record request
registry.Requests.Inc(map[string]string{
    "method": "POST",
    "path":   "/transactions",
    "status": "200",
})

// Record latency
registry.RequestLatency.Observe(150.5, map[string]string{
    "method": "POST",
    "path":   "/transactions",
})
```

## Testing

### Run All Tests
```bash
make test
```

### Run with Coverage
```bash
make test-coverage
# Open coverage.html in browser
```

### Run Integration Tests
```bash
# Start dependencies first
docker-compose up -d postgres redis

# Run integration tests
make test-integration
```

### Using Test Utilities

```go
import t "amanah/libs/testing"

func TestMyService(tt *testing.T) {
    suite := t.NewIntegrationSuite(tt)
    defer suite.Teardown()

    // Use factory for test data
    user := suite.Factory.CreateUser(t.WithUserRole("admin"))
    account := suite.Factory.CreateAccount(user.ID, t.WithAccountBalance(5000))

    // Use mocks
    suite.MockDB.Insert("users", user.ID, user)

    // API testing with fluent interface
    suite.SetupServer(myHandler)
    suite.API().
        Post("/transactions").
        WithJSON(map[string]interface{}{"amount": 100}).
        WithAuth("token").
        Expect().
        Status(201).
        BodyContains("transaction_id")
}
```

## Deployment

### Kubernetes

```bash
# Create namespace
kubectl create namespace amanah

# Apply secrets (edit with your values first)
kubectl apply -f deploy/kubernetes/secrets.yaml

# Deploy
kubectl apply -f deploy/kubernetes/deployment.yaml

# Check status
kubectl get pods -n amanah
```

### Docker

```bash
# Build production image
docker build --target production -t amanah/gateway:latest .

# Run container
docker run -d \
  -p 8080:8080 \
  -e DB_HOST=your-db-host \
  -e REDIS_HOST=your-redis-host \
  -e JWT_SECRET=your-jwt-secret \
  amanah/gateway:latest
```

## Monitoring

### Prometheus Metrics

The gateway exposes metrics at `/metrics`:
- `amanah_requests_total` - Total HTTP requests
- `amanah_request_duration_seconds` - Request latency histogram
- `amanah_errors_total` - Error count
- `amanah_active_connections` - Current active connections

### Grafana Dashboards

Import dashboards from `configs/grafana/dashboards/` or access Grafana at `http://localhost:3000` when running with docker-compose.

### Health Checks

```bash
# Liveness (is the service running?)
curl http://localhost:8080/health/live

# Readiness (is the service ready to accept traffic?)
curl http://localhost:8080/health/ready
```

## Security Considerations

### Production Checklist

- [ ] Use strong, unique secrets for JWT and API keys (min 32 chars)
- [ ] Enable TLS/HTTPS in production
- [ ] Configure proper CORS origins (not *)
- [ ] Set appropriate rate limits
- [ ] Enable database SSL mode
- [ ] Review and adjust RBAC permissions
- [ ] Enable audit logging
- [ ] Set up log aggregation
- [ ] Configure alerting for circuit breaker trips
- [ ] Regular security scanning (gosec, trivy)

### PCI DSS Compliance Notes

This framework provides building blocks for PCI DSS compliance:
- Sensitive data masking in logs
- Audit trail through immutable ledger
- Access control via RBAC
- Input validation
- Secure communication (TLS ready)

Actual compliance requires additional measures specific to your deployment.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Run `make lint` before committing
- Write tests for new functionality
- Update documentation for API changes
- Follow existing code patterns

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/osama1998H/Amanah/issues)
- **Documentation**: [docs/](docs/)
