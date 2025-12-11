# hpn-g-router

A high-performance API Key Router implementing Clean Architecture in Go.

## Project Structure

```
hpn-g-router/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── configs/
│   └── config.yaml              # Configuration file
├── internal/
│   ├── config/
│   │   ├── config.go            # Configuration struct & Singleton
│   │   ├── loader.go            # Viper-based config loading
│   │   └── errors.go            # Custom error types
│   ├── domain/
│   │   ├── provider.go          # Provider entity
│   │   └── keypool.go           # KeyPool & APIKey entities
│   ├── handler/                 # HTTP handlers (Clean Architecture)
│   ├── usecase/                 # Business logic layer
│   └── repository/              # Data access layer
├── pkg/                         # Public reusable packages
├── .env.example                 # Environment variables template
├── go.mod                       # Go module definition
└── README.md                    # This file
```

## Architecture

This project follows **Clean Architecture** principles:

```
┌─────────────────────────────────────────────────────────────┐
│                        Handlers                              │
│              (HTTP/gRPC request handling)                   │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                        UseCase                               │
│              (Business logic & orchestration)               │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   Repository / Domain                        │
│              (Data access & core entities)                  │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Environment Variables

Set environment variables with the `HPN_ROUTER_` prefix:

```bash
HPN_ROUTER_SERVER_PORT=8080
HPN_ROUTER_KEY_POOL_STRATEGY=round-robin
HPN_ROUTER_API_KEY_OPENAI_0=sk-your-key-here
```

### Config File

Place `config.yaml` in the `configs/` directory. See `configs/config.yaml` for all options.

## Rotation Strategies

- **round-robin**: Cycles through keys sequentially
- **random**: Selects a random key from the pool
- **weighted**: Selects keys based on their weight
- **least-used**: Selects the key with the fewest recent uses

## Getting Started

```bash
# Install dependencies
go mod tidy

# Run the server
go run cmd/server/main.go
```

## License

MIT
