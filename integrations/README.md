# Integrations

This directory contains connectors to external payment providers.

## Adding a New Provider

1. Create a subdirectory under `integrations/` named after the provider.
2. Implement a `client.go` file that defines a `Client` struct and a `NewClient` constructor.
3. Add methods for common operations such as `Charge` or `Refund`.
4. Document configuration and usage details in a `README.md` inside the provider directory.
