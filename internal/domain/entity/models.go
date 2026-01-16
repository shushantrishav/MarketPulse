package entity

import (
	"fmt"
	"time"

	"marketpulse/internal/infra/feed"
)

type HTTPError struct {
	StatusCode int
	Msg        string
}

func (e HTTPError) Error() string { return e.Msg }

func ErrInternal(msg string) error {
	return HTTPError{StatusCode: 500, Msg: fmt.Sprintf("internal: %s", msg)}
}

type IntradayRequest struct {
	Symbol  string   `json:"symbol" validate:"required"`
	Tail    *int     `json:"tail,omitempty"`
	RSILow  *float64 `json:"rsi_low,omitempty"`
	RSIHigh *float64 `json:"rsi_high,omitempty"`
}

type IntradayResponse struct {
	Symbol     string  		`json:"symbol"`
	Candles    []Candle 	`json:"candles,omitempty"`
	RSI        float64 		`json:"rsi"`
	ChangePct  float64 		`json:"change_pct"` 
	Alert      string  		`json:"alert,omitempty"`
	IsValidRSI bool    		`json:"is_valid_rsi"`
	LastFetch  time.Time 	`json:"last_fetch"`
	WarmupStatus string 	`json:"warmup_status"`
    SeededCandles int    	`json:"seeded_candles"`
	RSICount   int     		`json:"rsi_count"`
}

type Candle struct {
	Timestamp time.Time `json:"ts"`
	Open      float64   `json:"o"`
	High      float64   `json:"h"`
	Low       float64   `json:"l"`
	Close     float64   `json:"c"`
	Volume    int64     `json:"v"`
}

func CandleFromFeed(f *feed.Candle) *Candle {
	return &Candle{
		Timestamp: f.Timestamp,
		Open:      f.Open,
		High:      f.High,
		Low:       f.Low,
		Close:     f.Close,
		Volume:    f.Volume,
	}
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}
