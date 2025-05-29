# Shared Libraries

Reusable libraries and utilities for all services.

## Auth

`auth` provides an in-memory token manager:

```go
m := auth.NewManager()
token, err := m.GenerateToken("user1")
if err != nil {
    // handle error
}
if id, ok := m.ValidateToken(token); ok {
    fmt.Println("authenticated", id)
}
```

## Logging

`logging` is a thin wrapper over the standard `log` package:

```go
logging.Info("service started on %s", addr)
logging.Error("operation failed: %v", err)
```

## Metrics

`metrics` offers simple counters:

```go
c := metrics.NewCounter()
c.Inc("requests")
fmt.Println(c.Get("requests"))
```
