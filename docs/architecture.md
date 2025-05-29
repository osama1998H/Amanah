# Architecture Overview

Amanah is organized around a microservices design. Each service focuses on a specific domain and runs independently.

## Core Services
- **Transaction Service**: Manages the lifecycle of financial transactions.
- **Notification Service**: Handles delivery of emails, SMS, and other alerts.
- **Support Services**: Components such as Redis are used for caching and message brokering.

Services communicate via HTTP and message queues. They can be started together locally using Docker Compose.
