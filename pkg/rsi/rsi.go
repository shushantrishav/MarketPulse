// rsi.go - Updated with SMA seeding for first 14 + Wilder smoothing after
package rsi

import (
    "math"
    "sort"
    "time"

    "marketpulse/internal/infra/feed"
)

type CompactRSI struct {
    AvgGain   float64  `json:"avg_gain"`
    AvgLoss   float64  `json:"avg_loss"`
    Count     int      `json:"rsi_count"`
    LastTs    time.Time `json:"last_ts"`
    LastClose float64  `json:"last_close"`
    PrevClose float64  `json:"prev_close"`
    RSI       float64  `json:"rsi"`
    ChangePct float64  `json:"change_pct"`
}

// WarmupState indicates RSI quality
type WarmupState string

const (
    Processing   WarmupState = "processing"  // Count < 14
    Warming      WarmupState = "warming"     // 14 <= Count < 50
    Stable       WarmupState = "stable"      // Count >= 50
    Insufficient WarmupState = "insufficient" // No loss data
)

func (s *CompactRSI) IsValid() bool {
    return s.Count >= 14 && s.AvgLoss > 0
}

func (s *CompactRSI) WarmupStatus() WarmupState {
    if !s.IsValid() {
        return Processing
    }
    if s.AvgLoss == 0 {
        return Insufficient
    }
    if s.Count >= 50 {
        return Stable
    }
    return Warming
}

func (s *CompactRSI) UpdateIncremental(candle feed.Candle) bool {
    s.PrevClose = s.LastClose
    change := candle.Close - s.LastClose
    gain := math.Max(change, 0)
    loss := math.Max(-change, 0)

    if s.Count == 0 {
        s.AvgGain = gain
        s.AvgLoss = loss
    } else {
        s.AvgGain = (s.AvgGain*13 + gain) / 14
        s.AvgLoss = (s.AvgLoss*13 + loss) / 14
    }
    s.Count++
    s.LastClose = candle.Close
    s.LastTs = candle.Timestamp

    if s.AvgLoss > 0 {
        rs := s.AvgGain / s.AvgLoss
        s.RSI = 100 - (100/(1+rs))
    } else {
        s.RSI = 100
    }
    if s.PrevClose != 0 {
        s.ChangePct = (candle.Close - s.PrevClose) / s.PrevClose * 100
    }
    return s.Count >= 14
}

// SeedFromHistory computes SMA seed over first 14, then Wilder smoothing
func (s *CompactRSI) SeedFromHistory(candles []feed.Candle) {
    if len(candles) == 0 {
        return
    }

    // Reverse to chronological (oldest first) - feed returns newest first
    sort.Slice(candles, func(i, j int) bool {
        return candles[i].Timestamp.Before(candles[j].Timestamp)
    })

    s.Count = 0
    s.AvgGain, s.AvgLoss = 0, 0
    s.LastClose = candles[0].Close
    s.LastTs = candles[0].Timestamp

    // First 14: simple average (SMA) for seed
    if len(candles) >= 14 {
        gains, losses := make([]float64, 14), make([]float64, 14)
        for i := 1; i <= 14 && i < len(candles); i++ {
            change := candles[i].Close - candles[i-1].Close
            gains[i-1] = math.Max(change, 0)
            losses[i-1] = math.Max(-change, 0)
        }
        s.AvgGain = sum(gains) / 14
        s.AvgLoss = sum(losses) / 14
        s.Count = 14
        s.LastClose = candles[14].Close
        s.LastTs = candles[14].Timestamp
    } else {
        // Less than 14: incremental only
        for i := 1; i < len(candles); i++ {
            s.UpdateIncremental(candles[i])
        }
        return
    }

    // Remaining candles: Wilder smoothing
    for i := 15; i < len(candles); i++ {
        s.UpdateIncremental(candles[i])
    }
}

func sum(floats []float64) float64 {
    total := 0.0
    for _, f := range floats {
        total += f
    }
    return total
}

func CheckAlert(rsi float64, low, high float64) string {
    if rsi <= low {
        return "OVERSOLD"
    }
    if rsi >= high {
        return "OVERBOUGHT"
    }
    return ""
}
