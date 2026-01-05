# Amanah API Documentation

This document describes the REST API endpoints provided by the Amanah Payment Gateway.

## Base URL

```
Production: https://api.yourgateway.com
Development: http://localhost:8080
```

## Authentication

The API supports two authentication methods:

### JWT Bearer Token

Include the token in the Authorization header:
```
Authorization: Bearer <access_token>
```

### API Key

Include the API key in the header:
```
X-API-Key: <api_key>
```

## Common Headers

| Header | Description | Required |
|--------|-------------|----------|
| `Content-Type` | `application/json` for JSON bodies | Yes (POST/PUT) |
| `Authorization` | Bearer token for authenticated requests | Conditional |
| `X-API-Key` | API key for service-to-service calls | Conditional |
| `X-Request-ID` | Unique request identifier for tracing | Recommended |
| `X-Idempotency-Key` | Idempotency key for mutation operations | Recommended |

## Response Format

All responses follow this structure:

### Success Response
```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "request_id": "req_abc123",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

### Error Response
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters",
    "details": "Field 'amount' must be positive",
    "request_id": "req_abc123",
    "errors": [
      {
        "field": "amount",
        "message": "must be positive",
        "code": "INVALID_VALUE"
      }
    ]
  }
}
```

---

## Health Endpoints

### GET /health/live

Liveness probe - indicates if the service is running.

**Response**
```json
{
  "status": "ok",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### GET /health/ready

Readiness probe - indicates if the service is ready to accept traffic.

**Response**
```json
{
  "status": "ok",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "rabbitmq": "ok"
  }
}
```

---

## Authentication Endpoints

### POST /auth/login

Authenticate a user and receive access tokens.

**Request Body**
```json
{
  "email": "user@example.com",
  "password": "secure_password"
}
```

**Response**
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": {
      "id": "usr_abc123",
      "email": "user@example.com",
      "role": "merchant"
    }
  }
}
```

### POST /auth/refresh

Refresh an expired access token.

**Request Body**
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response**
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

### POST /auth/logout

Invalidate the current session.

**Headers**
```
Authorization: Bearer <access_token>
```

**Response**
```json
{
  "success": true,
  "data": {
    "message": "Successfully logged out"
  }
}
```

---

## Transaction Endpoints

### POST /api/v1/transactions

Create a new transaction.

**Headers**
```
Authorization: Bearer <access_token>
X-Idempotency-Key: unique-key-123
```

**Request Body**
```json
{
  "type": "transfer",
  "from_account_id": "acc_source123",
  "to_account_id": "acc_dest456",
  "amount": 100.50,
  "currency": "USD",
  "reference": "INV-2024-001",
  "metadata": {
    "description": "Invoice payment"
  }
}
```

**Response** (201 Created)
```json
{
  "success": true,
  "data": {
    "id": "txn_abc123xyz",
    "type": "transfer",
    "status": "pending",
    "from_account_id": "acc_source123",
    "to_account_id": "acc_dest456",
    "amount": 100.50,
    "currency": "USD",
    "reference": "INV-2024-001",
    "idempotency_key": "unique-key-123",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

### GET /api/v1/transactions/:id

Get transaction details.

**Response**
```json
{
  "success": true,
  "data": {
    "id": "txn_abc123xyz",
    "type": "transfer",
    "status": "completed",
    "from_account_id": "acc_source123",
    "to_account_id": "acc_dest456",
    "amount": 100.50,
    "currency": "USD",
    "reference": "INV-2024-001",
    "created_at": "2024-01-15T10:30:00Z",
    "completed_at": "2024-01-15T10:30:05Z",
    "metadata": {
      "description": "Invoice payment"
    }
  }
}
```

### GET /api/v1/transactions

List transactions with filtering and pagination.

**Query Parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| `page` | integer | Page number (default: 1) |
| `limit` | integer | Items per page (default: 20, max: 100) |
| `status` | string | Filter by status (pending, completed, failed, cancelled) |
| `type` | string | Filter by type (transfer, deposit, withdrawal) |
| `from_date` | string | Filter by start date (ISO 8601) |
| `to_date` | string | Filter by end date (ISO 8601) |
| `account_id` | string | Filter by account ID |

**Response**
```json
{
  "success": true,
  "data": [
    {
      "id": "txn_abc123xyz",
      "type": "transfer",
      "status": "completed",
      "amount": 100.50,
      "currency": "USD",
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 150,
    "total_pages": 8
  }
}
```

### POST /api/v1/transactions/:id/cancel

Cancel a pending transaction.

**Response**
```json
{
  "success": true,
  "data": {
    "id": "txn_abc123xyz",
    "status": "cancelled",
    "cancelled_at": "2024-01-15T10:35:00Z"
  }
}
```

### POST /api/v1/transactions/:id/refund

Refund a completed transaction.

**Request Body**
```json
{
  "amount": 50.00,
  "reason": "Customer request"
}
```

**Response**
```json
{
  "success": true,
  "data": {
    "refund_id": "ref_xyz789",
    "original_transaction_id": "txn_abc123xyz",
    "amount": 50.00,
    "status": "pending",
    "created_at": "2024-01-15T10:40:00Z"
  }
}
```

---

## Account Endpoints

### POST /api/v1/accounts

Create a new account.

**Request Body**
```json
{
  "type": "checking",
  "currency": "USD",
  "owner_id": "usr_abc123",
  "metadata": {
    "nickname": "Primary Account"
  }
}
```

**Response** (201 Created)
```json
{
  "success": true,
  "data": {
    "id": "acc_new123",
    "type": "checking",
    "currency": "USD",
    "balance": 0.00,
    "status": "active",
    "owner_id": "usr_abc123",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

### GET /api/v1/accounts/:id

Get account details.

**Response**
```json
{
  "success": true,
  "data": {
    "id": "acc_abc123",
    "type": "checking",
    "currency": "USD",
    "balance": 1500.00,
    "available_balance": 1450.00,
    "pending_balance": 50.00,
    "status": "active",
    "owner_id": "usr_abc123",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

### GET /api/v1/accounts/:id/balance

Get account balance.

**Response**
```json
{
  "success": true,
  "data": {
    "account_id": "acc_abc123",
    "currency": "USD",
    "balance": 1500.00,
    "available_balance": 1450.00,
    "pending_balance": 50.00,
    "as_of": "2024-01-15T10:30:00Z"
  }
}
```

### POST /api/v1/accounts/:id/deposit

Deposit funds into an account.

**Request Body**
```json
{
  "amount": 500.00,
  "source": "bank_transfer",
  "reference": "DEP-001"
}
```

**Response**
```json
{
  "success": true,
  "data": {
    "transaction_id": "txn_dep123",
    "account_id": "acc_abc123",
    "amount": 500.00,
    "new_balance": 2000.00,
    "status": "completed"
  }
}
```

### POST /api/v1/accounts/:id/withdraw

Withdraw funds from an account.

**Request Body**
```json
{
  "amount": 200.00,
  "destination": "bank_account",
  "reference": "WTH-001"
}
```

**Response**
```json
{
  "success": true,
  "data": {
    "transaction_id": "txn_wth123",
    "account_id": "acc_abc123",
    "amount": 200.00,
    "new_balance": 1300.00,
    "status": "pending"
  }
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request format or parameters |
| `UNAUTHORIZED` | 401 | Missing or invalid authentication |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource already exists or state conflict |
| `UNPROCESSABLE_ENTITY` | 422 | Business logic validation failed |
| `TOO_MANY_REQUESTS` | 429 | Rate limit exceeded |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |
| `GATEWAY_TIMEOUT` | 504 | Upstream service timeout |
| `VALIDATION_ERROR` | 400 | Input validation failed |
| `INSUFFICIENT_FUNDS` | 422 | Not enough balance |
| `DUPLICATE_ENTRY` | 409 | Duplicate idempotency key |
| `EXPIRED` | 410 | Resource has expired |
| `INVALID_STATE` | 422 | Invalid state transition |

---

## Rate Limits

| Endpoint Type | Limit | Window |
|---------------|-------|--------|
| Authentication | 10 requests | 1 minute |
| Read Operations | 1000 requests | 1 minute |
| Write Operations | 100 requests | 1 minute |
| Bulk Operations | 10 requests | 1 minute |

Rate limit headers are included in all responses:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 950
X-RateLimit-Reset: 1705315800
```

---

## Pagination

List endpoints support pagination via query parameters:

- `page`: Page number (1-indexed)
- `limit`: Items per page (max 100)

Response includes pagination metadata:
```json
{
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 150,
    "total_pages": 8,
    "has_next": true,
    "has_prev": false
  }
}
```

---

## Webhooks

Configure webhooks to receive real-time notifications for events.

### Event Types

| Event | Description |
|-------|-------------|
| `transaction.created` | New transaction created |
| `transaction.completed` | Transaction completed successfully |
| `transaction.failed` | Transaction failed |
| `transaction.cancelled` | Transaction cancelled |
| `account.created` | New account created |
| `account.updated` | Account details updated |
| `refund.created` | Refund initiated |
| `refund.completed` | Refund completed |

### Webhook Payload

```json
{
  "id": "evt_abc123",
  "type": "transaction.completed",
  "created_at": "2024-01-15T10:30:00Z",
  "data": {
    "id": "txn_xyz789",
    "status": "completed",
    "amount": 100.50
  }
}
```

### Webhook Signature

Verify webhook authenticity using the signature header:
```
X-Webhook-Signature: sha256=...
```
