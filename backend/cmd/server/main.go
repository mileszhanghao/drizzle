// Command server runs the SocialAI API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mileszhanghao/socialai/backend/internal/ai"
	"github.com/mileszhanghao/socialai/backend/internal/auth"
	"github.com/mileszhanghao/socialai/backend/internal/httpapi"
	"github.com/mileszhanghao/socialai/backend/internal/media"
	"github.com/mileszhanghao/socialai/backend/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	secret := os.Getenv("JWT_SECRET")
	tokens, err := auth.NewTokens(secret, 24*time.Hour)
	if err != nil {
		return errors.New("set JWT_SECRET to a random string of at least 32 characters (e.g. `openssl rand -hex 32`)")
	}

	var st store.Store
	if url := os.Getenv("DATABASE_URL"); url != "" {
		pg, err := store.OpenPostgres(ctx, url)
		if err != nil {
			return err
		}
		defer pg.Close()
		st = pg
	} else {
		log.Warn("DATABASE_URL not set: using in-memory storage (data is lost on restart)")
		st = store.NewMemory()
	}

	mediaStore, err := media.NewLocal(envOr("MEDIA_DIR", "./data/media"))
	if err != nil {
		return err
	}

	srv := &httpapi.Server{Store: st, Media: mediaStore, Tokens: tokens, Log: log}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		srv.Images = ai.NewOpenAIImages(key, envOr("OPENAI_IMAGE_MODEL", "gpt-image-1"))
		srv.ImageQuota, _ = strconv.Atoi(envOr("IMAGE_QUOTA_PER_HOUR", "10"))
	} else {
		log.Warn("OPENAI_API_KEY not set: AI image generation disabled")
	}

	handler := srv.Routes()
	if dir := os.Getenv("WEB_DIST_DIR"); dir != "" {
		handler = withSPA(handler, dir)
	}

	httpServer := &http.Server{
		Addr:              ":" + envOr("PORT", "8080"),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", httpServer.Addr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

// withSPA serves the built React app for every path the API doesn't own.
func withSPA(api http.Handler, dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.Handle("/media/", api)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(dir + r.URL.Path); r.URL.Path != "/" && err != nil {
			http.ServeFile(w, r, dir+"/index.html")
			return
		}
		files.ServeHTTP(w, r)
	})
	return mux
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
