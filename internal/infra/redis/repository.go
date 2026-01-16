package redis

import (
	"context"
	"marketpulse/pkg/rsi"
)

// StateRepository defines the behavior for managing RSI state.
// It uses the domain model rsi.CompactRSI from the pkg/rsi directory.
type StateRepository interface {
	GetOrUpdate(ctx context.Context, symbol string) (*rsi.CompactRSI, error)
	Save(ctx context.Context, symbol string, st *rsi.CompactRSI) error
}

// NewStateRepository returns an implementation of StateRepository.
// It returns a StateRouter which handles the transition between 
// Redis storage and the memory fallback.
func NewStateRepository(cli *Client, maxSymbols int) StateRepository {
	return NewStateRouter(cli, maxSymbols)
}
