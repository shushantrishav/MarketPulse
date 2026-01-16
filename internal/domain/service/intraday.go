package service

import (
    "context"
    "fmt"
    "time"

    "marketpulse/internal/domain/entity"
    "marketpulse/internal/infra/feed"
    "marketpulse/pkg/rsi"
)

type StateRepository interface {
    GetOrUpdate(ctx context.Context, symbol string) (*rsi.CompactRSI, error)
    Save(ctx context.Context, symbol string, st *rsi.CompactRSI) error
}

type IntradayService struct {
    stateRepo StateRepository
    feedCli   *feed.Client
}

func NewIntradayService(repo StateRepository, feedCli *feed.Client) *IntradayService {
    return &IntradayService{stateRepo: repo, feedCli: feedCli}
}

func (s *IntradayService) GetIntraday(ctx context.Context, req entity.IntradayRequest) (*entity.IntradayResponse, error) {
    if s == nil {
        return nil, fmt.Errorf("intraday service is nil")
    }
    if s.stateRepo == nil {
        return nil, fmt.Errorf("stateRepo is nil")
    }
    if s.feedCli == nil {
        return nil, fmt.Errorf("feedCli is nil")
    }
    if req.Symbol == "" {
        return nil, fmt.Errorf("symbol required")
    }

    state, err := s.stateRepo.GetOrUpdate(ctx, req.Symbol)
    if err != nil {
        return nil, err
    }
    if state == nil {
        state = &rsi.CompactRSI{}
    }

    var candles []entity.Candle
    seededCandles := 0
    changePct := 0.0

    // SEEDING: If uninitialized (Count == 0), fetch full history & warm up
    if state.Count == 0 {
        allCandles, err := s.feedCli.FetchIntraday(ctx, req.Symbol, time.Time{})
        if err == nil && len(allCandles) > 0 {
            state.SeedFromHistory(allCandles)
            s.stateRepo.Save(ctx, req.Symbol, state)
            candles = makeRecentCandles(allCandles, req.Tail)
            seededCandles = len(allCandles)
        }
    }

    // INCREMENTAL: Always fetch new candles (safe even after seeding)
    newCandles, err := s.feedCli.FetchIntraday(ctx, req.Symbol, state.LastTs)
    if err != nil {
        fmt.Printf("incremental fetch failed for %s: %v\n", req.Symbol, err)
    } else {
        prevClose := state.LastClose
        for _, c := range newCandles {
            // Dedupe: only process newer candles
            if !state.LastTs.IsZero() && !c.Timestamp.After(state.LastTs) {
                continue
            }

            // Compute change% before update
            if prevClose != 0 {
                changePct = ((c.Close - prevClose) / prevClose) * 100
            }

            state.UpdateIncremental(c)
            candles = append(candles, *entity.CandleFromFeed(&c))
            prevClose = c.Close
        }
    }

    // No new candles? Return last known (for fast polling)
    if len(candles) == 0 && state.Count > 0 {
        candles = append(candles, lastKnownCandle(state))
    }

    // Always persist
    if err := s.stateRepo.Save(ctx, req.Symbol, state); err != nil {
        fmt.Printf("state save failed for %s: %v\n", req.Symbol, err)
    }

    // Tail trim
    if req.Tail != nil && *req.Tail > 0 && len(candles) > *req.Tail {
        candles = candles[:*req.Tail]
    }

    low, high := 30.0, 70.0
    if req.RSILow != nil {
        low = *req.RSILow
    }
    if req.RSIHigh != nil {
        high = *req.RSIHigh
    }

    alert := rsi.CheckAlert(state.RSI, low, high)

    return &entity.IntradayResponse{
        Symbol:       req.Symbol,
        Candles:      candles,
        RSI:          state.RSI,
        ChangePct:    changePct,
        Alert:        alert,
        IsValidRSI:   state.IsValid(),
        WarmupStatus: string(state.WarmupStatus()),
        SeededCandles: seededCandles,
        LastFetch:    time.Now(),
        RSICount:     state.Count,
    }, nil
}

// Helpers
func makeRecentCandles(all []feed.Candle, tail *int) []entity.Candle {
    n := len(all)
    if tail != nil && *tail > 0 && *tail < n {
        n = *tail
    }
    out := make([]entity.Candle, 0, n)
    for i := len(all) - n; i < len(all); i++ {
        out = append(out, *entity.CandleFromFeed(&all[i]))
    }
    return out
}

func lastKnownCandle(state *rsi.CompactRSI) entity.Candle {
    return entity.Candle{
        Timestamp: state.LastTs,
        Open:      state.LastClose,
        High:      state.LastClose,
        Low:       state.LastClose,
        Close:     state.LastClose,
        Volume:    0,
    }
}
