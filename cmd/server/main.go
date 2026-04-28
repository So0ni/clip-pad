package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/So0ni/clip-pad/internal/config"
	clipdb "github.com/So0ni/clip-pad/internal/db"
	appmiddleware "github.com/So0ni/clip-pad/internal/middleware"
	"github.com/So0ni/clip-pad/internal/paste"
	"github.com/So0ni/clip-pad/internal/ratelimit"
	"github.com/go-chi/chi/v5"
)

type appPageData struct {
	CurrentPage string
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := clipdb.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	if err := clipdb.Init(ctx, database); err != nil {
		log.Fatalf("initialize database: %v", err)
	}

	renderer, err := newTemplateRenderer(filepath.Join("web", "templates"))
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	service := paste.NewService(database, paste.Config{
		MaxPasteSize:         cfg.MaxPasteSize,
		MaxTotalContentBytes: cfg.MaxTotalContentBytes,
		IPHashSecret:         cfg.IPHashSecret,
		RateLimit: ratelimit.Config{
			PerIPPerMinute: cfg.RateLimitPerIPPerMinute,
			PerIPPerDay:    cfg.RateLimitPerIPPerDay,
			GlobalPerDay:   cfg.RateLimitGlobalPerDay,
		},
	})

	if err := service.CleanupExpiredPastes(ctx); err != nil {
		log.Fatalf("initial paste cleanup: %v", err)
	}
	if err := service.CleanupExpiredRateLimits(ctx); err != nil {
		log.Fatalf("initial rate-limit cleanup: %v", err)
	}

	cleanupCtx, cleanupStop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cleanupStop()
	go startCleanupLoop(cleanupCtx, service)

	router := chi.NewRouter()
	router.Use(appmiddleware.RequestLogger)
	router.Use(appmiddleware.NewRealIPResolver(cfg.TrustCloudflare, cfg.TrustProxyHeaders, cfg.TrustedProxyCIDRs).Middleware)

	pasteHandler := paste.NewHandler(service, renderer, cfg.MaxPasteSize)
	pasteHandler.Routes(router)

	fileServer := http.FileServer(http.Dir(filepath.Join("web", "static")))
	staticHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400, must-revalidate")
		fileServer.ServeHTTP(w, r)
	})
	router.Handle("/static/*", http.StripPrefix("/static/", staticHandler))
	router.Get("/", func(w http.ResponseWriter, req *http.Request) {
		if err := renderer.Render(w, http.StatusOK, "index.html", appPageData{CurrentPage: "paste-bin"}); err != nil {
			log.Printf("render index error: %v", err)
		}
	})
	router.Get("/notepad", func(w http.ResponseWriter, req *http.Request) {
		if err := renderer.Render(w, http.StatusOK, "notepad.html", appPageData{CurrentPage: "notepad"}); err != nil {
			log.Printf("render notepad error: %v", err)
		}
	})
	router.Get("/robots.txt", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("ClipPad listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	<-cleanupCtx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

func startCleanupLoop(ctx context.Context, service *paste.Service) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := service.CleanupExpiredPastes(context.Background()); err != nil {
				log.Printf("cleanup expired pastes error: %v", err)
			}
			if err := service.CleanupExpiredRateLimits(context.Background()); err != nil {
				log.Printf("cleanup expired rate limits error: %v", err)
			}
		}
	}
}
