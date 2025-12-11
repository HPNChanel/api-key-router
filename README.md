<![CDATA[```
 ██╗  ██╗██████╗ ███╗   ██╗    ██████╗  ██████╗ ██╗   ██╗████████╗███████╗██████╗ 
 ██║  ██║██╔══██╗████╗  ██║    ██╔══██╗██╔═══██╗██║   ██║╚══██╔══╝██╔════╝██╔══██╗
 ███████║██████╔╝██╔██╗ ██║    ██████╔╝██║   ██║██║   ██║   ██║   █████╗  ██████╔╝
 ██╔══██║██╔═══╝ ██║╚██╗██║    ██╔══██╗██║   ██║██║   ██║   ██║   ██╔══╝  ██╔══██╗
 ██║  ██║██║     ██║ ╚████║    ██║  ██║╚██████╔╝╚██████╔╝   ██║   ███████╗██║  ██║
 ╚═╝  ╚═╝╚═╝     ╚═╝  ╚═══╝    ╚═╝  ╚═╝ ╚═════╝  ╚═════╝    ╚═╝   ╚══════╝╚═╝  ╚═╝
```

<h3 align="center">✨ The Infinite Gateway - Turn Free Tier into Enterprise Power ✨</h3>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/hpn/hpn-g-router"><img src="https://goreportcard.com/badge/github.com/hpn/hpn-g-router?style=flat-square&label=Go%20Report%20Card&color=00ADD8" alt="Go Report Card: A+"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-green.svg?style=flat-square" alt="License: MIT"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Built%20with-Golang-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Built with: Golang"></a>
  <img src="https://img.shields.io/badge/Money%20Saved-♾️-gold?style=flat-square" alt="Money Saved: ♾️">
</p>

---

## 🤔 The Problem

You've been there. We've all been there.

> **"Tired of Google Gemini's 15 RPM limit?"**
>
> **"Hate getting `429 Too Many Requests` errors right in the middle of your workflow?"**
>
> **"Don't want to rewrite your entire OpenAI-compatible codebase just to switch providers?"**

The Free Tier is generous... but **15 requests per minute** isn't enough when you're building something real.

---

## 💡 The Solution

**HPN Router** is your smart **Load Balancer & Failover Proxy** that sits between your application and Google's Gemini API.

It manages **multiple free API keys** to create a **continuous, unbreakable stream** of AI power. When one key hits its limit, the next one takes over. Instantly. Seamlessly.

```
┌──────────────────┐         ┌──────────────────┐         ┌──────────────────┐
│                  │         │                  │         │                  │
│   Your App       │ ──────► │   HPN Router     │ ──────► │  Google Gemini   │
│   (OpenAI SDK)   │         │   (The Magic)    │         │  (Free Tier)     │
│                  │         │                  │         │                  │
└──────────────────┘         └──────────────────┘         └──────────────────┘
                                     │
                              🔄 Key 1 → 429?
                              🔄 Key 2 → 429?
                              🔄 Key 3 → ✅ Success!
```

---

## ✨ Key Features

| Feature | Description |
|---------|-------------|
| 🔄 **Smart Rotation** | Round-robin key scheduling distributes load evenly across all your API keys |
| 🛡️ **Immortal Mode** | Auto-failover mechanism - if Key 1 fails with 429, Key 2 takes over instantly |
| 💸 **Cost Estimator** | Tracks how much $$$ you would have paid OpenAI (spoiler: it's a lot) |
| ⚡ **Flash Cache** | In-memory caching for identical requests - get instant `0ms` responses |
| 🔌 **Universal Adapter** | Speak "OpenAI" to your app → Get "Gemini" under the hood |

---

## 🚀 Quick Start

Get up and running in **30 seconds**.

### 1. Clone & Install

```bash
git clone https://github.com/hpn/hpn-g-router.git
cd hpn-g-router
go mod download
```

### 2. Configure Your Keys

Create `configs/config.yaml`:

```yaml
# 🔐 HPN Router Configuration
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
      key: "AIzaSy...your-first-key..."
      provider: "google"
      enabled: true

    - name: "gemini_key_2"
      key: "AIzaSy...your-second-key..."
      provider: "google"
      enabled: true

    - name: "gemini_key_3"
      key: "AIzaSy...your-third-key..."
      provider: "google"
      enabled: true

logging:
  level: "info"
  format: "json"
```

> **💡 Pro Tip:** For production, use the `HPN_API_KEYS` environment variable instead of a config file:
> ```bash
> export HPN_API_KEYS="AIzaSy...key1,AIzaSy...key2,AIzaSy...key3"
> ```

### 3. Run the Server

```bash
go run cmd/server/main.go
```

That's it! 🎉 Your infinite gateway is now running at `http://localhost:8080`.

---

## 📖 Usage Examples

### Using cURL

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer any-fake-key-works" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello, world!"}
    ]
  }'
```

### Using Python (OpenAI SDK)

The magic? **Just change the `base_url`**. Your existing code works as-is.

```python
from openai import OpenAI

# Point to HPN Router instead of OpenAI
client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="any-key-works-here"  # Router handles the real keys
)

response = client.chat.completions.create(
    model="gpt-4",  # Router translates this to Gemini
    messages=[
        {"role": "user", "content": "Explain quantum computing in simple terms."}
    ]
)

print(response.choices[0].message.content)
```

### Using Node.js (OpenAI SDK)

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8080/v1',
  apiKey: 'any-key-works-here',
});

const response = await client.chat.completions.create({
  model: 'gpt-4',
  messages: [{ role: 'user', content: 'Write a haiku about coding.' }],
});

console.log(response.choices[0].message.content);
```

---

## ⚙️ Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `server.host` | string | `0.0.0.0` | Server bind address |
| `server.port` | int | `8080` | Server port |
| `server.read_timeout_seconds` | int | `30` | HTTP read timeout |
| `server.write_timeout_seconds` | int | `30` | HTTP write timeout |
| `key_pool.strategy` | string | `round-robin` | Key rotation strategy |
| `key_pool.retry_count` | int | `3` | Number of retry attempts per request |
| `key_pool.cooldown_seconds` | int | `60` | Cooldown period for failed keys |
| `logging.level` | string | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `logging.format` | string | `json` | Log format (`json`, `text`) |

---

## 🗺️ Roadmap

- [x] 🔄 Round-Robin Key Rotation
- [x] 🛡️ Auto-Failover (Immortal Mode)
- [x] 💸 Cost Tracker
- [x] ⚡ Flash Cache
- [x] 🔐 Log Redaction (Security)
- [ ] 📊 Dashboard UI *(Coming Soon)*
- [ ] 🌐 Multi-Provider Support (Anthropic, Mistral)
- [ ] 📈 Prometheus Metrics Export

---

## 🤝 Contributing

Contributions are welcome! Feel free to:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Built with ❤️ by <strong>HPN Corporation</strong>
</p>

<p align="center">
  ⭐ <strong>Star this repo if it saved you money!</strong> ⭐
</p>

<p align="center">
  <sub>Because enterprise AI shouldn't cost enterprise money.</sub>
</p>
]]>
