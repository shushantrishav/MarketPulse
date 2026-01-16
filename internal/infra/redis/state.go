package redis

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"golang.org/x/sync/singleflight"
	"marketpulse/pkg/rsi"
)

// StateRouter manages the Redis client and an in-memory fallback.
type StateRouter struct {
	cli    *Client
	memMu  sync.RWMutex
	memory map[string]*rsi.CompactRSI
	sf     singleflight.Group
	maxSym int
}

// NewStateRouter initializes a new StateRouter.
func NewStateRouter(cli *Client, maxSymbols int) *StateRouter {
	return &StateRouter{
		cli:    cli,
		memory: make(map[string]*rsi.CompactRSI),
		maxSym: maxSymbols,
	}
}

// GetOrUpdate retrieves state from Redis or falls back to memory.
// Auto-saves after reading if Redis is available.
func (s *StateRouter) GetOrUpdate(ctx context.Context, symbol string) (*rsi.CompactRSI, error) {
	if !s.cli.IsDown() {
		if state, err := s.redisGet(ctx, symbol); err == nil && state != nil {
			// Auto-save: ensure Redis has the latest copy
			if err := s.redisSave(ctx, symbol, state); err != nil {
				fmt.Printf("redis save failed: %v\n", err)
			}
			return state, nil
		}
	}

	// Use singleflight to prevent cache stampede on fallback
	resIface, err, _ := s.sf.Do(symbol, func() (interface{}, error) {
		return s.memoryGetOrInit(ctx, symbol)
	})
	if err != nil {
		return nil, err
	}

	st := resIface.(*rsi.CompactRSI)
	if st == nil {
		return nil, fmt.Errorf("fallback returned nil state")
	}
	return st, nil
}
// Save persists RSI state to Redis (no-op if down) and memory.
func (s *StateRouter) Save(ctx context.Context, symbol string, st *rsi.CompactRSI) error {
	if !s.cli.IsDown() {
		if err := s.redisSave(ctx, symbol, st); err != nil {
			return fmt.Errorf("redis save: %w", err)
		}
	}
	// Sync to memory (double-write for consistency)
	s.memMu.Lock()
	s.memory[symbol] = st
	s.memMu.Unlock()
	if len(s.memory) > s.maxSym {
		s.memMu.Lock()
		for k := range s.memory {
			if k != symbol {
				delete(s.memory, k)
				break
			}
		}
		s.memMu.Unlock()
	}
	return nil
}

// redisGet fetches RSI state from Redis.
func (s *StateRouter) redisGet(ctx context.Context, symbol string) (*rsi.CompactRSI, error) {
	data, err := s.cli.RDB().HGetAll(ctx, fmt.Sprintf("symbol:%s:compact", symbol)).Result()
	if err != nil || len(data) == 0 {
		return nil, err
	}

	state := &rsi.CompactRSI{}
	if ag, ok := data["avg_gain"]; ok {
		state.AvgGain, _ = strconv.ParseFloat(ag, 64)
	}
	if al, ok := data["avg_loss"]; ok {
		state.AvgLoss, _ = strconv.ParseFloat(al, 64)
	}
	if cnt, ok := data["rsi_count"]; ok {
		state.Count, _ = strconv.Atoi(cnt)
	}
	if tsData, ok := data["last_ts"]; ok {
		if err := state.LastTs.UnmarshalJSON([]byte(tsData)); err == nil {
			// success
		}
	}
	if lc, ok := data["last_close"]; ok {
		state.LastClose, _ = strconv.ParseFloat(lc, 64)
	}
	if r, ok := data["rsi"]; ok {
		state.RSI, _ = strconv.ParseFloat(r, 64)
	}
	return state, nil
}

// redisSave writes updated state back to Redis.
func (s *StateRouter) redisSave(ctx context.Context, symbol string, st *rsi.CompactRSI) error {
	if s.cli.IsDown() || st == nil {
		return nil
	}
	key := fmt.Sprintf("symbol:%s:compact", symbol)
	data := map[string]interface{}{
		"avg_gain":   fmt.Sprintf("%.8f", st.AvgGain),
		"avg_loss":   fmt.Sprintf("%.8f", st.AvgLoss),
		"rsi_count":  st.Count,
		"last_close": fmt.Sprintf("%.4f", st.LastClose),
		"rsi":        fmt.Sprintf("%.8f", st.RSI),
	}
	if !st.LastTs.IsZero() {
		if b, err := st.LastTs.MarshalJSON(); err == nil {
			data["last_ts"] = string(b)
		}
	}
	return s.cli.RDB().HSet(ctx, key, data).Err()
}

// memoryGetOrInit manages the in-memory cache fallback.
func (s *StateRouter) memoryGetOrInit(ctx context.Context, symbol string) (*rsi.CompactRSI, error) {
	s.memMu.RLock()
	if st, ok := s.memory[symbol]; ok {
		s.memMu.RUnlock()
		return st, nil
	}
	s.memMu.RUnlock()

	st := &rsi.CompactRSI{}
	s.memMu.Lock()
	defer s.memMu.Unlock()

	if existing, ok := s.memory[symbol]; ok {
		return existing, nil
	}
	s.memory[symbol] = st

	// LRU eviction
	if len(s.memory) > s.maxSym {
		for k := range s.memory {
			if k != symbol {
				delete(s.memory, k)
				break
			}
		}
	}
	return st, nil
}
