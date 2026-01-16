package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"marketpulse/internal/config"
)

const (
	tailCandles = 200
	timeout     = 8 * time.Second
)

type Candle struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
}

// AlphaVantage-like candle payload (strings)
type avCandle struct {
	Open   string `json:"1. open"`
	High   string `json:"2. high"`
	Low    string `json:"3. low"`
	Close  string `json:"4. close"`
	Volume string `json:"5. volume"`
}

// Generic AV-style response
type avResponse struct {
	MetaData map[string]any              `json:"Meta Data"`
	Series   map[string]map[string]avCandle `json:"-"`
	Error    string                      `json:"error,omitempty"`
	Note     string                      `json:"Note,omitempty"`
}

func (r *avResponse) UnmarshalJSON(b []byte) error {
	// First decode normally
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if v, ok := raw["Meta Data"]; ok {
		_ = json.Unmarshal(v, &r.MetaData)
	}
	if v, ok := raw["error"]; ok {
		_ = json.Unmarshal(v, &r.Error)
	}
	if v, ok := raw["Note"]; ok {
		_ = json.Unmarshal(v, &r.Note)
	}

	// Find the "Time Series (...)" key dynamically (e.g., "Time Series (5min)")
	for k, v := range raw {
		if strings.HasPrefix(k, "Time Series (") {
			r.Series = make(map[string]map[string]avCandle, 1)
			var m map[string]avCandle
			if err := json.Unmarshal(v, &m); err != nil {
				return err
			}
			r.Series[k] = m
			break
		}
	}
	return nil
}

type Client struct {
	client  *resty.Client
	baseURL string
	limiter *time.Ticker
}

func NewClient(cfg *config.Config) *Client {
	c := resty.New()
	c.SetTimeout(timeout)
	c.SetRetryCount(0)

	return &Client{
		client:  c,
		baseURL: strings.TrimRight(cfg.UpstreamURL, "/"),
		limiter: time.NewTicker(250 * time.Millisecond),
	}
}

func (f *Client) FetchIntraday(ctx context.Context, symbol string, since time.Time) ([]Candle, error) {
	if f == nil || f.client == nil {
		return nil, fmt.Errorf("feed client not initialized")
	}

	<-f.limiter.C

	u := fmt.Sprintf(
		"%s/query?function=TIME_SERIES_INTRADAY&symbol=%s&interval=5min&apikey=demo&tail=%d",
		f.baseURL,
		url.QueryEscape(symbol),
		tailCandles,
	)

	resp, err := f.client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		Get(u)
	if err != nil {
		return nil, fmt.Errorf("feed request: %w", err)
	}
	if resp.StatusCode() >= 400 {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode(), resp.String())
	}

	var data avResponse
	if err := json.Unmarshal(resp.Body(), &data); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}
	if data.Error != "" {
		return nil, fmt.Errorf("upstream error: %s", data.Error)
	}
	if data.Note != "" {
		return nil, fmt.Errorf("upstream note: %s", data.Note)
	}
	if len(data.Series) == 0 {
		return []Candle{}, nil
	}

	// Get the only series map
	var tsMap map[string]avCandle
	for _, m := range data.Series {
		tsMap = m
		break
	}

	out := make([]Candle, 0, len(tsMap))
	for tsStr, c := range tsMap {
		// Your timestamps are like: "2026-01-15 08:46:23" and timezone is UTC
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", tsStr, time.UTC)
		if err != nil {
			continue
		}
		if !since.IsZero() && !ts.After(since) {
			continue
		}

		open, err1 := strconv.ParseFloat(c.Open, 64)
		high, err2 := strconv.ParseFloat(c.High, 64)
		low, err3 := strconv.ParseFloat(c.Low, 64)
		closep, err4 := strconv.ParseFloat(c.Close, 64)
		vol, err5 := strconv.ParseInt(c.Volume, 10, 64)

		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
			continue
		}

		out = append(out, Candle{
			Timestamp: ts,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closep,
			Volume:    vol,
		})
	}

	// Newest first (matches your API output ordering expectation)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})

	return out, nil
}
