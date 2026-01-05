# API Overview

This page provides sample API definitions used by Amanah services. APIs are exposed over HTTP with JSON payloads, and may also be defined using gRPC.

## OpenAPI Example (Transaction Service)

```yaml
openapi: 3.0.0
info:
  title: Transaction Service API
  version: 1.0.0
paths:
  /transactions:
    post:
      summary: Create a new transaction
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/NewTransaction'
      responses:
        '201':
          description: Transaction created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Transaction'
  /transactions/{id}:
    get:
      summary: Get transaction by ID
      parameters:
        - in: path
          name: id
          schema:
            type: string
          required: true
      responses:
        '200':
          description: Transaction status
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Transaction'
components:
  schemas:
    NewTransaction:
      type: object
      properties:
        amount:
          type: number
        currency:
          type: string
        sourceAccount:
          type: string
        destinationAccount:
          type: string
    Transaction:
      allOf:
        - $ref: '#/components/schemas/NewTransaction'
        - type: object
          properties:
            id:
              type: string
            status:
              type: string
```

## gRPC Example (Account Service)

```protobuf
syntax = "proto3";
package account;

service AccountService {
  rpc GetAccount(GetAccountRequest) returns (Account) {}
  rpc CreateAccount(CreateAccountRequest) returns (Account) {}
}

message GetAccountRequest {
  string id = 1;
}

message CreateAccountRequest {
  string name = 1;
  string email = 2;
}

message Account {
  string id = 1;
  string name = 2;
  string email = 3;
}
```

These samples serve as starting points for defining and documenting APIs across Amanah services. Real implementations should refine schemas, validation rules, and authentication requirements.

