# HPN Router

```
 ██╗  ██╗██████╗ ███╗   ██╗    ██████╗  ██████╗ ██╗   ██╗████████╗███████╗██████╗ 
 ██║  ██║██╔══██╗████╗  ██║    ██╔══██╗██╔═══██╗██║   ██║╚══██╔══╝██╔════╝██╔══██╗
 ███████║██████╔╝██╔██╗ ██║    ██████╔╝██║   ██║██║   ██║   ██║   █████╗  ██████╔╝
 ██╔══██║██╔═══╝ ██║╚██╗██║    ██╔══██╗██║   ██║██║   ██║   ██║   ██╔══╝  ██╔══██╗
 ██║  ██║██║     ██║ ╚████║    ██║  ██║╚██████╔╝╚██████╔╝   ██║   ███████╗██║  ██║
 ╚═╝  ╚═╝╚═╝     ╚═╝  ╚═══╝    ╚═╝  ╚═╝ ╚═════╝  ╚═════╝    ╚═╝   ╚══════╝╚═╝  ╚═╝
```

<div align="center">

**Smart Load Balancer & Failover Proxy for Google Gemini API**

[![Go Report Card](https://goreportcard.com/badge/github.com/hpn/hpn-g-router?style=flat-square&label=Go%20Report%20Card&color=00ADD8)](https://goreportcard.com/report/github.com/hpn/hpn-g-router)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg?style=flat-square)](LICENSE)
[![Built with Golang](https://img.shields.io/badge/Built%20with-Golang-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org)
![Money Saved](https://img.shields.io/badge/Money%20Saved-♾️-gold?style=flat-square)

</div>

## Overview

**HPN Router** is a production-grade API gateway that manages multiple Google Gemini API keys with intelligent load balancing and automatic failover. It provides an OpenAI-compatible interface, allowing you to leverage Google's free tier without changing your existing codebase.

### Problem Statement

- **Rate Limits**: Google Gemini's free tier enforces a 15 RPM limit per API key
- **Error Handling**: `429 Too Many Requests` errors interrupt workflows
- **Vendor Lock-in**: Switching between AI providers requires significant code changes

### Solution

HPN Router acts as a transparent proxy between your application and Google Gemini, managing multiple API keys to create a unified, high-availability endpoint with automatic failover and load distribution.

```
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│              │         │              │         │              │
│  Your App    │────────▶│  HPN Router  │────────▶│   Gemini API │
│ (OpenAI SDK) │         │              │         │  (Free Tier) │
│              │         │              │         │              │
└──────────────┘         └──────────────┘         └──────────────┘
                                │
                         ┌──────┴──────┐
                         │  Key Pool   │
                         ├─────────────┤
                         │ Key 1 ─ 15R │
                         │ Key 2 ─ 15R │
                         │ Key 3 ─ 15R │
                         └─────────────┘
```

---

## Features

### Core Capabilities

| Feature | Description |
|---------|-------------|
| **Smart Rotation** | Round-robin scheduling distributes requests evenly across all API keys |
| **Immortal Mode** | Automatic failover - when one key hits rate limits, the next takes over instantly |
| **Cost Estimator** | Tracks equivalent OpenAI costs to demonstrate savings |
| **Flash Cache** | In-memory caching for duplicate requests with sub-millisecond response times |
| **Universal Adapter** | OpenAI-compatible API that transparently translates requests to Gemini format |
| **Security** | Automatic log redaction for API keys and sensitive headers |

### Architecture

```
internal/
├── adapter/        # Provider-specific API translation
│   └── gemini.go   # OpenAI → Gemini request/response mapping
├── config/         # Configuration management (Viper)
├── domain/         # Core business logic
│   ├── key_manager.go    # Thread-safe key rotation
│   └── models.go         # Domain entities
├── handler/        # HTTP request handling
│   └── chat.go     # Chat completion endpoint
└── middleware/     # Cross-cutting concerns
    ├── cache.go    # Flash cache implementation
    ├── cost.go     # Cost estimation
    └── logger.go   # Structured logging with redaction
```

---

## Installation

### Prerequisites

- Go 1.21 or higher
- Multiple Google Gemini API keys ([Get keys here](https://aistudio.google.com/app/apikey))

### Quick Start

```bash
# Clone the repository
git clone https://github.com/hpn/hpn-g-router.git
cd hpn-g-router

# Install dependencies
go mod download

# Configure API keys (see Configuration section)
cp configs/config.example.yaml configs/config.yaml
# Edit config.yaml with your API keys

# Run the server
go run cmd/server/main.go
```

### Build from Source

```bash
# Build binary
go build -o hpn-router cmd/server/main.go

# Run binary
./hpn-router
```

---

## Configuration

### File-based Configuration

Create `configs/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout_seconds: 30
  write_timeout_seconds: 30

key_pool:
  strategy: "round-robin"
  retry_count: 3
  cooldown_seconds: 60
  keys:
    - name: "gemini_key_1"
      key: "AIzaSyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
      provider: "google"
      enabled: true
    - name: "gemini_key_2"
      key: "AIzaSyYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY"
      provider: "google"
      enabled: true

logging:
  level: "info"      # debug | info | warn | error
  format: "json"     # json | text
```

### Environment Variables (Production)

For production deployments, use environment variables to avoid committing secrets:

```bash
# Comma-separated list of API keys
export HPN_API_KEYS="AIzaSyXXX,AIzaSyYYY,AIzaSyZZZ"

# Optional: Override server port
export HPN_PORT="8080"

# Optional: Override log level
export HPN_LOG_LEVEL="info"
```

> **Security Note**: The router automatically prioritizes environment variables over config files and redacts sensitive data from logs.

### Configuration Reference

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `server.host` | string | `0.0.0.0` | Server bind address |
| `server.port` | int | `8080` | HTTP port |
| `server.read_timeout_seconds` | int | `30` | Request read timeout |
| `server.write_timeout_seconds` | int | `30` | Response write timeout |
| `key_pool.strategy` | string | `round-robin` | Key selection strategy |
| `key_pool.retry_count` | int | `3` | Max retries per request |
| `key_pool.cooldown_seconds` | int | `60` | Failed key cooldown period |
| `logging.level` | string | `info` | Log verbosity |
| `logging.format` | string | `json` | Log output format |

---

## Usage

### cURL Example

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dummy-key" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain machine learning in one sentence."}
    ]
  }'
```

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="dummy-key"  # Any value works; router manages real keys
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "What is the capital of France?"}
    ]
)

print(response.choices[0].message.content)
```

### Node.js (OpenAI SDK)

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8080/v1',
  apiKey: 'dummy-key'
});

const completion = await client.chat.completions.create({
  model: 'gpt-4',
  messages: [
    { role: 'system', content: 'You are a helpful assistant.' },
    { role: 'user', content: 'Write a haiku about TypeScript.' }
  ]
});

console.log(completion.choices[0].message.content);
```

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "active_keys": 3,
  "total_requests": 1523,
  "total_saved": "$45.67"
}
```

---

## Advanced Features

### Flash Cache

The in-memory cache uses SHA-256 hashing of request bodies to identify duplicate requests:

```
Request Flow:
1. Hash incoming request body
2. Check cache (TTL: 5 minutes)
3. Cache HIT → Return immediately (~0ms latency)
4. Cache MISS → Forward to Gemini → Store response
```

**Log Output:**
```
{"level":"info","msg":"⚡ CACHE HIT","latency_ms":0.12}
{"level":"info","msg":"💸 CHA-CHING! You saved $0.0008 on this request. Total Saved: $45.67"}
```

### Cost Estimator

Calculates equivalent OpenAI costs using approximate tokenization:

**Pricing:**
- Input: $0.50 per 1M tokens
- Output: $1.50 per 1M tokens

**Token Estimation:** `tokens ≈ word_count × 1.3`

### Automatic Failover

When a key receives a `429` response:

1. Mark key as temporarily disabled (cooldown period)
2. Retrieve next available key from pool
3. Retry request with new key
4. Log failover event

**Example Log:**
```json
{
  "level":"warn",
  "msg":"Key rotation triggered",
  "reason":"429 Too Many Requests",
  "old_key":"gemini_key_1",
  "new_key":"gemini_key_2",
  "retry_attempt":1
}
```

---

## Testing

### Run Unit Tests

```bash
go test ./... -v
```

### Run with Race Detection

```bash
go test ./... -race
```

### E2E Tests

```bash
go test ./tests -v -run TestRouterE2E
```

**Test Coverage:**
- Happy path (single key, successful request)
- Failover logic (429 → retry with different key)
- Exhaustion scenario (all keys depleted)
- Concurrency (100 parallel requests, no race conditions)

---

## Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o hpn-router cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/hpn-router .

EXPOSE 8080
CMD ["./hpn-router"]
```

```bash
# Build image
docker build -t hpn-router:latest .

# Run container
docker run -d \
  -p 8080:8080 \
  -e HPN_API_KEYS="key1,key2,key3" \
  hpn-router:latest
```

### Systemd Service

```ini
[Unit]
Description=HPN Router Service
After=network.target

[Service]
Type=simple
User=hpn-router
WorkingDirectory=/opt/hpn-router
ExecStart=/opt/hpn-router/hpn-router
Restart=on-failure
Environment="HPN_API_KEYS=key1,key2,key3"

[Install]
WantedBy=multi-user.target
```

---

## Roadmap

- [x] Round-robin key rotation
- [x] Automatic failover (Immortal Mode)
- [x] Cost tracking and estimation
- [x] In-memory caching (Flash Cache)
- [x] Log redaction for security
- [x] OpenAI-compatible API adapter
- [ ] Web dashboard for monitoring
- [ ] Multi-provider support (Anthropic Claude, Mistral)
- [ ] Prometheus metrics export
- [ ] Redis-backed distributed cache
- [ ] Rate limiting per client
- [ ] Webhook notifications for key exhaustion

---

## Contributing

Contributions are welcome! Please follow these steps:

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feature/your-feature`
3. **Commit** your changes: `git commit -m 'Add new feature'`
4. **Push** to the branch: `git push origin feature/your-feature`
5. **Open** a Pull Request

### Development Guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go) conventions
- Add tests for new features
- Update documentation for API changes
- Run `go fmt` and `go vet` before committing

---

## License

This project is licensed under the **MIT License**. See [LICENSE](LICENSE) for details.

---

## Support

- **Issues**: [GitHub Issues](https://github.com/hpn/hpn-g-router/issues)
- **Discussions**: [GitHub Discussions](https://github.com/hpn/hpn-g-router/discussions)

---

<div align="center">

**Built with ❤️ by HPN Corporation**

*Because enterprise AI shouldn't cost enterprise money.*

⭐ **Star this repo if it saved you money!** ⭐

</div>

