# Setup and Installation

This guide walks you through setting up Amanah for local development.

## Prerequisites

- Go 1.23 or later
- Docker and Docker Compose

## Steps

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd Amanah
   ```
2. Build and start the services with Docker Compose:
   ```bash
   docker-compose up --build
   ```
   Services will be available on `http://localhost:8080` by default.
3. Alternatively, run the Go services directly:
   ```bash
   go run ./...
   ```

Configuration files for different environments are located in the `configs/` directory. Update environment variables as needed before running the services.

