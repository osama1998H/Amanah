# System Architecture

This document provides an overview of the Amanah microservices architecture.

## Overview

Amanah is composed of loosely coupled services that communicate over HTTP and message queues. Each service owns its data and exposes a small API surface.

### Core Services

- **Transaction Service** – Handles payment lifecycle operations such as initiating, checking status, and refunds.
- **Account Service** – Manages user and merchant accounts, including KYC and payment instrument linking.
- **Notification Service** – Delivers events through email, SMS, push notifications, or webhooks.
- **Ledger Service** – Records immutable entries for financial transactions.

### Service Interactions

1. A client creates a transaction via the Transaction Service.
2. The Transaction Service verifies account details with the Account Service.
3. Successful transactions are recorded in the Ledger Service.
4. The Notification Service is notified asynchronously to inform the user of updates.

```
Client -> Transaction Service -> Account Service
                      |-> Ledger Service
                      |-> Notification Service (async)
```

Message queues can be introduced between services to decouple high‑traffic operations such as notifications or ledger writes. Services publish events that consumers process independently, allowing the platform to scale horizontally.

