import asyncio
import random
import time
from datetime import datetime, timedelta, timezone
from fastapi import FastAPI, Query
from fastapi.responses import JSONResponse

app = FastAPI(title="Ultra Low Latency Mock Alpha Vantage API")

# ─────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────
INTERVALS = {
    "1min": timedelta(minutes=1),
    "5min": timedelta(minutes=5),
    "15min": timedelta(minutes=15),
    "30min": timedelta(minutes=30),
    "60min": timedelta(minutes=60),
}

# ─────────────────────────────────────────────
# Global State (lock-free reads)
# ─────────────────────────────────────────────
SYMBOLS: set[str] = {"IBM", "AAPL", "MSFT"}

PRICE_STATE: dict[str, float] = {
    "IBM": 300.0,
    "AAPL": 180.0,
    "MSFT": 320.0,
}

# symbol -> interval -> response
CACHE: dict[str, dict[str, dict]] = {}

# ─────────────────────────────────────────────
# Candle Generation
# ─────────────────────────────────────────────
def generate_candle(price: float) -> dict:
    # Symmetric drift around price: true random walk for realistic RSI
    change_pct = random.uniform(-2.0, +2.0)  # ±2% per candle (realistic 1min/5min volatility)
    new_close = round(price * (1 + change_pct/100), 4)
    
    # OHLC within tight bounds of close (realistic)
    o = round(price + random.uniform(-0.1, 0.1), 4)  # Open near prior close
    h = round(max(o, new_close) + abs(random.uniform(0, 0.2)), 4)
    l = round(min(o, new_close) - abs(random.uniform(0, 0.2)), 4)
    c = new_close
    
    return {
        "1. open": f"{o:.4f}",
        "2. high": f"{h:.4f}",
        "3. low": f"{l:.4f}",
        "4. close": f"{c:.4f}",
        "5. volume": str(random.randint(100, 5000)),
    }



def build_response(symbol: str, interval: str, candles: dict) -> dict:
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
    return {
        "Meta Data": {
            "1. Information": f"Intraday ({interval}) open, high, low, close prices and volume",
            "2. Symbol": symbol,
            "3. Last Refreshed": now,
            "4. Interval": interval,
            "5. Output Size": "Compact",
            "6. Time Zone": "UTC",
        },
        f"Time Series ({interval})": candles,
    }

# ─────────────────────────────────────────────
# Background Price Engine
# ─────────────────────────────────────────────
async def price_engine():
    global CACHE

    while True:
        now = datetime.now(timezone.utc)
        new_cache: dict[str, dict[str, dict]] = {}

        for symbol in SYMBOLS:
            PRICE_STATE[symbol] += random.uniform(-1.0, 1.0)
            last_price = PRICE_STATE[symbol]

            new_cache[symbol] = {}

            for interval, delta in INTERVALS.items():
                candles = {}
                price = last_price

                for i in range(20):
                    ts = now - i * delta
                    ts_str = ts.strftime("%Y-%m-%d %H:%M:%S")
                    candle = generate_candle(price)
                    price = float(candle["4. close"])
                    candles[ts_str] = candle

                # newest → oldest
                candles = dict(sorted(candles.items(), reverse=True))
                new_cache[symbol][interval] = build_response(
                    symbol, interval, candles
                )

        # 🔥 Atomic pointer swap (lock-free for readers)
        CACHE = new_cache

        await asyncio.sleep(1)


@app.on_event("startup")
async def startup():
    asyncio.create_task(price_engine())

# ─────────────────────────────────────────────
# Server-side latency measurement (GROUND TRUTH)
# ─────────────────────────────────────────────
@app.middleware("http")
async def server_timing_middleware(request, call_next):
    start_ns = time.perf_counter_ns()
    response = await call_next(request)
    response.headers["X-Server-Time-ns"] = str(
        time.perf_counter_ns() - start_ns
    )
    return response

# ─────────────────────────────────────────────
# Ultra-fast API Endpoint
# ─────────────────────────────────────────────
@app.get("/query")
async def intraday(
    function: str = Query(...),
    symbol: str = Query(...),
    interval: str = Query(...),
    apikey: str = Query(...),
    tail: int | None = Query(None, ge=1),  # 👈 NEW
):
    if function != "TIME_SERIES_INTRADAY":
        return JSONResponse(status_code=400, content={"error": "Invalid function"})

    if apikey != "demo":
        return JSONResponse(status_code=403, content={"error": "Invalid API key"})

    if interval not in INTERVALS:
        return JSONResponse(status_code=400, content={"error": "Invalid interval"})

    # 🔥 Lock-free auto-registration
    if symbol not in PRICE_STATE:
        SYMBOLS.add(symbol)
        PRICE_STATE[symbol] = random.uniform(50, 500)

    symbol_cache = CACHE.get(symbol)
    if not symbol_cache:
        return JSONResponse(
            content={
                "Meta Data": {
                    "Symbol": symbol,
                    "Note": "Warming up symbol"
                },
                f"Time Series ({interval})": {}
            }
        )

    # 🔥 O(1) hot path + optional tail
    response = symbol_cache[interval]

    if tail is not None:
        ts_key = f"Time Series ({interval})"
        candles = response[ts_key]

        limited = dict(list(candles.items())[:tail])

        return JSONResponse(
            content={
                "Meta Data": response["Meta Data"],
                ts_key: limited,
            }
        )

    return JSONResponse(content=response)
