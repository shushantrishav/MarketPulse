package handlers

import (
	"net/http"

	"github.com/go-chi/render"

	"marketpulse/internal/api/middleware"
	"marketpulse/internal/config"
	"marketpulse/internal/domain/entity"
)

type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	req := entity.LoginRequest{}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid JSON"})
		return
	}

	// Demo user
	if req.Username != "demo" || req.Password != "demo" {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "invalid credentials"})
		return
	}

	token, _ := middleware.GenerateJWT(h.cfg.JWTSecret, "demo_user_1", h.cfg.JWTExpiry)
	render.JSON(w, r, entity.LoginResponse{
		Token:  token,
		UserID: "demo_user_1",
	})
}
