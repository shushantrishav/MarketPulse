# MarketPulse: Real-Time Stock Analytics & RSI Tracking API

MarketPulse is a **Go microservice** that tracks intraday stock movements and generates **real-time RSI alerts**. It maintains per-symbol RSI state across requests and restarts, providing accurate technical analysis with oversold/overbought notifications for professional trading workflows.

---

## 🚀 Features

| Feature | Description |
|------|------------|
| **Stateful RSI Tracking** | Maintains per-symbol RSI state across requests/restarts using Wilder's Smoothing with SMA seeding. |
| **Hybrid State Storage** | Redis persistence + in-memory fallback with eviction (configurable max symbols). |
| **Cache Stampede Protection** | Singleflight pattern prevents thundering herd during fallback initialization. |
| **Incremental Data Fetch** | Fetches only candles newer than last update timestamp to minimize bandwidth. |
| **Smart Warmup Flow** | 3-phase RSI warmup: Processing (<14), Warming (14–50), Stable (≥50). |
| **Real-time Alerts** | Oversold/Overbought detection with configurable thresholds. |
| **Rate Limiting** | Upstream API protection via ticker-based request pacing. |
| **JWT Authentication** | Secure token-based access control for production environments. |
| **Middleware** | CORS, structured logging (Zap), panic recovery, and request tracing. |

---

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Frontend      │    │  MarketPulse     │    │  Upstream API   │
│  (Static SPA)   │◄──►│  (Go Microsvc)   │◄──►│ (AlphaVantage+) │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                       ┌──────────────────┐
                       │     Redis        │  ← Compact RSI State
                       │ (symbol:*.compact)|
                       └──────────────────┘
```

---

## 🧩 Core Design Patterns

- **Repository Pattern** – `StateRepository` abstracts Redis/memory storage logic
- **Circuit Breaker** – Redis health checks with transparent fallback to memory
- **Singleflight** – Prevents concurrent fallback initialization stampedes
- **Dependency Injection** – Clean service wiring in `main.go`
- **Context Propagation** – Full request tracing through middleware stack

---

## 📡 API Reference

### Authentication

**POST** `/login`

```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"demo"}'
```

**Response**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user_id": "demo_user_1"
}
```

---

### Market Data (Protected)

**GET** `/market/intraday/{symbol}`

```bash
curl "http://localhost:8080/market/intraday/IBM?tail=20&rsi_low=25&rsi_high=75" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

#### Query Parameters

| Param | Type | Default | Description |
|----|----|----|----|
| `tail` | int | nil | Max recent candles to return |
| `rsi_low` | float64 | 30.0 | Oversold threshold |
| `rsi_high` | float64 | 70.0 | Overbought threshold |

**Sample Response**
```json
{
  "symbol": "IBM",
  "rsi": 42.56,
  "change_pct": 1.25,
  "alert": "",
  "is_valid_rsi": true,
  "warmup_status": "stable",
  "seeded_candles": 200,
  "rsi_count": 127,
  "last_fetch": "2026-01-16T10:30:00Z",
  "candles": [
    {
      "ts": "2026-01-16T10:25:00Z",
      "o": 145.20,
      "h": 145.80,
      "l": 145.10,
      "c": 145.65,
      "v": 123456
    }
  ]
}
```

---

## 🧠 RSI Calculation Pipeline

```
1. LOAD STATE ─┐
                ├─ [Redis UP] ──→ symbol:IBM:compact ──┐
                └─ [Redis DOWN] ─→ Memory Fallback ────┤
                                                        │
2. WARMUP/UPDATE ──────────────────────────────────────┤
   - If Count=0: Seed from full history (SMA 14-period) │
   - Fetch incremental candles (> LastTs)               │
   - Wilder smoothing: (Prev*13 + Current)/14           │
                                                        │
3. ALERT → RESPONSE ───────────────────────────────────┘
```

---

## 🔥 Warmup States

| State | Count | Description |
|----|----|----|
| **processing** | < 14 | Initial SMA calculation period |
| **warming** | 14–49 | Wilder smoothing warmup; usable but lower confidence |
| **stable** | ≥ 50 | Production-ready accuracy |
| **insufficient** | – | No loss data detected (prevents division by zero) |

---

## 🛠️ Tech Stack

| Component | Technology | Purpose |
|----|----|----|
| Web Framework | Chi Router | Lightweight, composable routing |
| State Store | Redis + In-memory | Hybrid persistence + fallback |
| HTTP Client | Resty | Upstream API calls |
| Auth | JWT (HS256) | Token-based security |
| Logging | Zap | Structured logging |

---

## 🚀 Quick Start

### 1. Clone & Configure
```bash
git clone <repository> marketpulse
cd marketpulse
cp .env.example .env
```

### 2. Environment Variables
```env
# Required
REDIS_ADDR=redis:6379
UPSTREAM_URL=http://localhost:8000/query
JWT_SECRET=your-super-secret-key-min-32-chars
HTTP_PORT=:8080

# Optional
MAX_SYMBOLS_MEMORY=1000
JWT_EXPIRY=24h
LOG_LEVEL=info
```

### 3. Docker Compose (Recommended)
```bash
docker-compose up --build
```

---

## 📁 Project Structure

```
marketpulse/
├── cmd/
│   └── main.go
├── internal/
│   ├── api/
│   ├── domain/
│   └── infra/
└── pkg/
    └── rsi/
```

---

## 📈 Performance Characteristics

| Operation | p50 | p95 | Notes |
|----|----|----|----|
| Cold RSI (seed) | 180ms | 320ms | Full history fetch + SMA |
| Warm RSI | 45ms | 120ms | Incremental + cache hit |
| Redis Fallback | 2ms | 8ms | Pure memory path |
| Memory Pressure | 15ms | 45ms | LRU eviction overhead |

---

## 📄 License

This project is licensed under the **MIT License**.

