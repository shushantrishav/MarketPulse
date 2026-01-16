// Updated frontend - handles new warmup_status, seeded_candles, better UX
const API_BASE = "http://localhost:8080";
let token = localStorage.getItem("token") || "";
let pollTimer = null;
let chart = null;
const candleHistory = [];
const MAX_POINTS = 50;

function qs(id) {
  return document.getElementById(id);
}

function setAuthedUI() {
  qs("login").classList.add("hidden");
  qs("app").classList.remove("hidden");
}

function setLoggedOutUI() {
  qs("login").classList.remove("hidden");
  qs("app").classList.add("hidden");
  stopPolling();
}

function showError(msg) {
  const status = qs("status");
  if (status) status.textContent = msg;
}

async function apiFetch(path, opts = {}) {
  const headers = Object.assign({}, opts.headers || {});
  if (!headers["Content-Type"] && opts.body)
    headers["Content-Type"] = "application/json";
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${API_BASE}${path}`, { ...opts, headers });
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch (_) {
    data = null;
  }

  if (!res.ok) {
    const msg =
      (data && (data.error || data.message)) ||
      `HTTP ${res.status} ${res.statusText}`;
    throw new Error(msg);
  }
  return data;
}

async function login(username, password) {
  const data = await apiFetch("/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
  if (!data || !data.token) throw new Error("No token received");
  token = data.token;
  localStorage.setItem("token", token);
  setAuthedUI();
}

function logout() {
  token = "";
  localStorage.removeItem("token");
  setLoggedOutUI();
}

async function poll() {
  const symbol = (qs("symbol")?.value || "NVDA").trim();
  const rsiLow = parseFloat(qs("rsiLow")?.value || "30");
  const rsiHigh = parseFloat(qs("rsiHigh")?.value || "70");

  const data = await apiFetch(
    `/market/intraday/${encodeURIComponent(
      symbol
    )}?tail=1&rsi_low=${rsiLow}&rsi_high=${rsiHigh}`,
    { method: "GET" }
  );

  updateUI(data);
  updateCandleTable(data);
  updateChart(data);

  const raw = qs("raw");
  if (raw) raw.textContent = JSON.stringify(data, null, 2);
}

function updateUI(data) {
  // RSI Value & Gauge
  const rsiVal = Number(data.rsi || 0);
  qs("metricRsi").innerText = rsiVal.toFixed(2);
  const gauge = qs("rsiGauge");
  const circumference = 502.6;
  gauge.style.strokeDashoffset = circumference - (rsiVal / 100) * circumference;
  qs("rsiValueBig").innerText = Math.round(rsiVal);

  // NEW: Warmup Status (most important!)
  const warmupEl = qs("metricWarmup");
  const warmupStatus = data.warmup_status || "unknown";
  warmupEl.innerText = warmupStatus.toUpperCase();
  warmupEl.className = `text-xl font-black px-3 py-1 rounded-full ${
    warmupStatus === "stable" ? "bg-emerald-500/20 text-emerald-400 border border-emerald-500/50" :
    warmupStatus === "warming" ? "bg-amber-500/20 text-amber-400 border border-amber-500/50" :
    "bg-slate-500/20 text-slate-400 border border-slate-500/50"
  }`;

  // NEW: Seeding info
  const seedInfo = qs("seedInfo");
  if (data.seeded_candles > 0) {
    seedInfo.textContent = `Seeded: ${data.seeded_candles}`;
    seedInfo.classList.remove("hidden");
  } else {
    seedInfo.classList.add("hidden");
  }

  // Count with warmup progress
  qs("metricCount").innerText = `${data.rsi_count || 0}`;
  const progressEl = qs("warmupProgress");
  if (data.rsi_count >= 14) {
    const progress = Math.min((data.rsi_count / 50) * 100, 100);
    progressEl.style.width = `${progress}%`;
    progressEl.className = `h-2 bg-gradient-to-r ${
      progress < 100 ? "from-amber-400 to-yellow-400" : "from-emerald-400 to-emerald-600"
    } rounded-full transition-all duration-300`;
  }

  // Change %
  qs("metricChange").innerText = `${Number(data.change_pct || 0).toFixed(2)}%`;
  qs("metricChange").className = `text-2xl font-black font-mono-custom ${
    data.change_pct >= 0 ? "text-emerald-400" : "text-rose-400"
  }`;

  // Valid RSI
  qs("metricValid").innerText = data.is_valid_rsi ? "YES" : "NO";
  qs("metricValid").className = `text-2xl font-black font-mono-custom ${
    data.is_valid_rsi ? "text-emerald-400" : "text-amber-500"
  }`;

  // Alert
  const alertEl = qs("alert");
  if (data.alert) {
    alertEl.textContent = `${data.symbol.toUpperCase()}: ${data.alert}`;
    alertEl.classList.remove("hidden");
    alertEl.classList.add("animate-pulse");
  } else {
    alertEl.classList.add("hidden");
  }

  // Last fetch
  qs("lastFetch").innerText = `Last: ${
    data.last_fetch ? new Date(data.last_fetch).toLocaleTimeString() : "-"
  }`;

  // Threshold colors
  const lowT = parseFloat(qs("rsiLow").value);
  const highT = parseFloat(qs("rsiHigh").value);
  if (rsiVal >= highT) {
    gauge.style.stroke = "#ff1b1b";
    qs("rsiValueBig").style.color = "#ff1b1b";
    qs("rsiLabel").innerText = "OVERBOUGHT";
    qs("rsiLabel").className = "text-rose-400 font-bold";
  } else if (rsiVal <= lowT) {
    gauge.style.stroke = "#17a122";
    qs("rsiValueBig").style.color = "#17a122";
    qs("rsiLabel").innerText = "OVERSOLD";
    qs("rsiLabel").className = "text-emerald-400 font-bold";
  } else {
    gauge.style.stroke = "#3b82f6";
    qs("rsiValueBig").style.color = "#f8fafc";
    qs("rsiLabel").innerText = "NEUTRAL";
    qs("rsiLabel").className = "text-blue-400";
  }
}

function updateCandleTable(data) {
  if (!data.candles || data.candles.length === 0) return;

  data.candles.forEach((newCandle) => {
    if (!candleHistory.find((c) => c.ts === newCandle.ts)) {
      candleHistory.push(newCandle);
      if (candleHistory.length > 100) candleHistory.shift();
    }
  });

  const body = qs("candleBody");
  const displayData = [...candleHistory].reverse().slice(0, 20); // Last 20
  body.innerHTML = displayData
    .map(
      (c) => `
        <tr class="border-b border-slate-800/50 hover:bg-slate-800/30 transition-colors">
          <td class="p-3 text-xs text-slate-500">${new Date(c.ts).toLocaleTimeString()}</td>
          <td class="p-3 text-xs text-slate-300">${c.o.toFixed(2)}</td>
          <td class="p-3 text-xs text-emerald-400 font-bold">${c.h.toFixed(2)}</td>
          <td class="p-3 text-xs text-rose-400 font-bold">${c.l.toFixed(2)}</td>
          <td class="p-3 text-sm font-bold text-slate-100">${c.c.toFixed(2)}</td>
          <td class="p-3 text-xs text-right text-slate-500">${c.v.toLocaleString()}</td>
        </tr>
      `
    )
    .join("");

  qs("tableStatus").innerText = `${candleHistory.length} candles cached`;
}

function ensureChart() {
  if (chart) return chart;
  const ctx = qs("chart").getContext("2d");
  chart = new Chart(ctx, {
    type: "candlestick",
    data: {
      datasets: [
        {
          label: "Price",
          data: [],
          color: {
            up: "#17a122",
            down: "#ff1b1b",
            unchanged: "#333",
          },
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        x: {
          type: "time",
          time: { unit: "minute" },
          grid: { color: "rgba(255,255,255,0.05)" },
          ticks: { color: "#64748b", font: { size: 10 } },
        },
        y: {
          grid: { color: "rgba(255,255,255,0.05)" },
          ticks: { color: "#64748b", font: { size: 10 } },
        },
      },
      plugins: {
        legend: { display: false },
      },
    },
  });
  return chart;
}

function updateChart(data) {
  const c = ensureChart();

  // NEW: Show RSI line when valid
  const rsiValid = data.is_valid_rsi;
  if (!c.data.datasets[1]) {
    c.data.datasets.push({
      label: "RSI",
      type: "line",
      data: [],
      borderColor: rsiValid ? "#3b82f6" : "#64748b",
      borderWidth: 2,
      fill: false,
      yAxisID: "y1",
      tension: 0.1,
    });
  }
  c.options.scales.y1 = {
    type: "linear",
    display: rsiValid,
    position: "right",
    min: 0,
    max: 100,
    grid: { drawOnChartArea: false },
  };
  c.data.datasets[1].borderColor = rsiValid ? "#3b82f6" : "#64748b";

  // Candles
  const chartData = candleHistory.slice(-MAX_POINTS).map((candle) => ({
    x: new Date(candle.ts).getTime(),
    o: candle.o,
    h: candle.h,
    l: candle.l,
    c: candle.c,
  }));
  c.data.datasets[0].data = chartData;

  // RSI line data (when valid)
  if (rsiValid && candleHistory.length > 0) {
    c.data.datasets[1].data = candleHistory.slice(-MAX_POINTS).map((candle, idx) => ({
      x: new Date(candle.ts).getTime(),
      y: data.rsi,  // Current RSI value
    }));
  } else {
    c.data.datasets[1].data = [];
  }

  c.update("none");
}

function startPolling() {
  const interval = parseInt(qs("pollInterval")?.value || "2000", 10);
  const btn = qs("btnPoll");
  if (pollTimer) {
    stopPolling();
    btn.innerText = "▶️ Start";
    btn.className = "bg-blue-600 hover:bg-blue-500 text-white px-6 py-2.5 rounded-xl font-bold transition-all text-xs";
    return;
  }

  btn.innerText = "⏹️ Stop";
  btn.className = "bg-rose-600 hover:bg-rose-500 text-white px-6 py-2.5 rounded-xl font-bold transition-all text-xs";

  poll().catch((e) => showError(`Poll error: ${e.message}`));
  pollTimer = setInterval(() => {
    poll().catch((e) => showError(`Poll error: ${e.message}`));
  }, interval);
}

function stopPolling() {
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = null;
}

window.addEventListener("DOMContentLoaded", () => {
  if (token) {
    setAuthedUI();
    ensureChart();
    startPolling();
  } else {
    setLoggedOutUI();
  }

  qs("login-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      const u = qs("username").value;
      const p = qs("password").value;
      await login(u, p);
      startPolling();
    } catch (err) {
      showError(`Login failed: ${err.message}`);
    }
  });
});
