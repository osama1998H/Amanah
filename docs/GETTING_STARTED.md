# Getting Started with Amanah Payment Gateway

This guide will walk you through setting up and running Amanah for the first time.

## Prerequisites

Before you begin, ensure you have the following installed:

- **Go 1.23+** - [Download Go](https://golang.org/dl/)
- **Docker** - [Install Docker](https://docs.docker.com/get-docker/)
- **Docker Compose** - Usually included with Docker Desktop
- **Make** (optional) - For convenient build commands
- **Git** - For cloning the repository

## Step 1: Clone the Repository

```bash
git clone https://github.com/osama1998H/Amanah.git
cd Amanah
```

## Step 2: Install Dependencies

```bash
# Download Go modules
go mod download

# Or using Make
make deps
```

## Step 3: Start Infrastructure

The gateway requires PostgreSQL and Redis. Start them using Docker Compose:

```bash
# Start only the required services
docker-compose up -d postgres redis

# Verify they're running
docker-compose ps
```

## Step 4: Configure Environment

Create a `.env` file or export environment variables:

```bash
# Database
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=amanah
export DB_USER=amanah
export DB_PASSWORD=amanah_secret

# Redis
export REDIS_HOST=localhost
export REDIS_PORT=6379

# Security (use your own secrets in production!)
export JWT_SECRET=your-jwt-secret-key-minimum-32-characters
export API_KEY_SECRET=your-api-key-secret

# Logging
export LOG_LEVEL=debug
export LOG_FORMAT=json
```

## Step 5: Run the Gateway

```bash
# Run directly
go run ./cmd/gateway

# Or build and run
go build -o bin/gateway ./cmd/gateway
./bin/gateway
```

You should see output like:
```
{"level":"info","time":"...","message":"Starting Amanah Gateway","port":8080}
{"level":"info","time":"...","message":"Database connected"}
{"level":"info","time":"...","message":"Redis connected"}
{"level":"info","time":"...","message":"Server listening on :8080"}
```

## Step 6: Verify Installation

Test the health endpoints:

```bash
# Liveness check
curl http://localhost:8080/health/live
# Response: {"status":"ok"}

# Readiness check
curl http://localhost:8080/health/ready
# Response: {"status":"ok","checks":{"database":"ok","redis":"ok"}}
```

## Step 7: Run Tests

```bash
# Run all tests
go test ./...

# With verbose output
go test -v ./...

# With coverage
go test -cover ./...
```

## Next Steps

### Explore the API

Check out the [API Documentation](API.md) for available endpoints.

### Configure for Production

Review the [Production Deployment Guide](DEPLOYMENT.md) for production setup.

### Customize the Gateway

See the [Library Usage Guide](LIBRARIES.md) for how to use individual components.

## Troubleshooting

### Database Connection Failed

```
Error: failed to connect to database
```

**Solution**: Ensure PostgreSQL is running and the credentials are correct:
```bash
docker-compose ps postgres
docker-compose logs postgres
```

### Redis Connection Failed

```
Error: failed to connect to Redis
```

**Solution**: Verify Redis is running:
```bash
docker-compose ps redis
docker-compose logs redis
```

### Port Already in Use

```
Error: listen tcp :8080: bind: address already in use
```

**Solution**: Either stop the other process or use a different port:
```bash
# Find what's using port 8080
lsof -i :8080

# Or use a different port
export SERVER_PORT=8081
```

### Go Module Issues

```
Error: cannot find package
```

**Solution**: Clean and re-download modules:
```bash
go clean -modcache
go mod download
```

## Development Workflow

### Making Changes

1. Make your code changes
2. Run linting: `go vet ./...`
3. Run tests: `go test ./...`
4. Build: `go build ./cmd/gateway`

### Hot Reload (Development)

Use [Air](https://github.com/cosmtrek/air) for hot reloading during development:

```bash
# Install Air
go install github.com/cosmtrek/air@latest

# Run with hot reload
air
```

### Using Make Commands

```bash
make deps          # Download dependencies
make fmt           # Format code
make lint          # Run linters
make test          # Run tests
make build         # Build binaries
make docker-build  # Build Docker image
make up            # Start all services
make down          # Stop all services
make logs          # View logs
make clean         # Clean build artifacts
```

## Full Stack Development

To run the complete stack including monitoring:

```bash
# Start everything
docker-compose up -d

# Services available:
# - Gateway:    http://localhost:8080
# - Prometheus: http://localhost:9091
# - Grafana:    http://localhost:3000 (admin/admin)
# - Jaeger:     http://localhost:16686
# - RabbitMQ:   http://localhost:15672 (amanah/amanah_secret)
```

## Getting Help

- Check the [FAQ](FAQ.md)
- Review [GitHub Issues](https://github.com/osama1998H/Amanah/issues)
- Read the code documentation with `go doc`
