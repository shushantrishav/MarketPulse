package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"marketpulse/internal/domain/entity"
	"marketpulse/internal/domain/service"
)

type MarketHandler struct {
	intradaySvc *service.IntradayService
}

func NewMarketHandler(svc *service.IntradayService) *MarketHandler {
	return &MarketHandler{intradaySvc: svc}
}

func (h *MarketHandler) Intraday(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.intradaySvc == nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "intraday service not wired"})
		return
	}

	req := entity.IntradayRequest{
		Symbol: chi.URLParam(r, "symbol"),
	}

	if tailStr := r.URL.Query().Get("tail"); tailStr != "" {
		if tail, err := strconv.Atoi(tailStr); err == nil {
			req.Tail = &tail
		}
	}

	if lowStr := r.URL.Query().Get("rsi_low"); lowStr != "" {
		if low, err := strconv.ParseFloat(lowStr, 64); err == nil {
			req.RSILow = &low
		}
	}

	if highStr := r.URL.Query().Get("rsi_high"); highStr != "" {
		if high, err := strconv.ParseFloat(highStr, 64); err == nil {
			req.RSIHigh = &high
		}
	}
    
	resp, err := h.intradaySvc.GetIntraday(r.Context(), req)
	if err != nil {
		status := http.StatusBadGateway
		if he, ok := err.(entity.HTTPError); ok {
			status = he.StatusCode
		}
		render.Status(r, status)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	// Optional tail trimming at API layer
	if req.Tail != nil && *req.Tail > 0 && len(resp.Candles) > *req.Tail {
		resp.Candles = resp.Candles[:*req.Tail]
	}

	render.JSON(w, r, resp)
}
