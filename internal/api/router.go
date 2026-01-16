package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"go.uber.org/zap"

	"marketpulse/internal/api/handlers"
	"marketpulse/internal/api/middleware"
	"marketpulse/internal/config"
	"marketpulse/internal/domain/service"
)

func NewRouter(cfg *config.Config, logger *zap.Logger, marketSvc *service.IntradayService) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recoverer)
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	authHandler := handlers.NewAuthHandler(cfg)
	r.Post("/login", authHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth([]byte(cfg.JWTSecret)))

		marketHandler := handlers.NewMarketHandler(marketSvc)
		r.Get("/market/intraday/{symbol}", marketHandler.Intraday)
	})

	staticDir := http.Dir("./web/static/")
	fs := http.FileServer(staticDir)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	})
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	return r
}
