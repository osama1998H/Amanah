# Contributing to Amanah

Thank you for considering contributing to Amanah! This document outlines how to set up the project locally, the coding style we follow, and the process for submitting pull requests.

## Running the Project

The easiest way to run the project is via Docker Compose.

1. Ensure you have **Docker** and **Docker Compose** installed.
2. Clone the repository and navigate to its root directory.
3. Build and start the services:
   ```bash
   docker-compose up --build
   ```
4. The application will be available on `http://localhost:8080`.

Alternatively, if you prefer running the Go services directly, install Go 1.23 or later and run:
```bash
go run ./...
```

## Coding Style

- Format all Go code using `gofmt` before committing:
  ```bash
  gofmt -w <files>
  ```
- Run `go vet ./...` to catch potential issues.
- Keep functions small and focused, and write clear comments for exported functions.
- Follow idiomatic Go naming conventions.

## Submitting Pull Requests

1. Fork the repository and create a new branch for your change.
2. Ensure all tests pass (`go test ./...`) and the project builds.
3. Commit your changes with clear commit messages.
4. Open a pull request against the `main` branch describing your changes and any related issues.
5. One of the maintainers will review your PR. Please respond to feedback and update your branch as needed.

We appreciate your contributions!
