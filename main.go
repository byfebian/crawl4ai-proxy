package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var httpClient *http.Client
var globalMetrics = NewMetrics()

func initHTTPClient() {
	httpClient = &http.Client{
		Timeout: time.Duration(CRAWL_TOTAL_TIMEOUT_SECONDS) * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: time.Duration(CRAWL_CONNECT_TIMEOUT_SECONDS) * time.Second,
			}).DialContext,
		},
	}
}

var cache *ResponseCache

func setupLogger() {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if strings.ToLower(LOG_FORMAT) == "json" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, opts)))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))
	}
}

func main() {
	ReadEnvironment()
	setupLogger()
	initHTTPClient()

	if CACHE_ENABLED {
		cache = NewResponseCache(CACHE_MAX_ENTRIES, time.Duration(CACHE_TTL_SECONDS)*time.Second, globalMetrics)
	}

	rl := NewRateLimiter(RATE_LIMIT_RPS, RATE_LIMIT_BURST)

	mux := http.NewServeMux()
	mux.HandleFunc("/crawl", CrawlEndpoint)
	mux.HandleFunc("/md", MdEndpoint)
	mux.HandleFunc("/screenshot", ScreenshotEndpoint)
	mux.HandleFunc("/execute_js", ExecuteJsEndpoint)
	mux.HandleFunc("/health", HealthEndpoint)
	mux.HandleFunc("/metrics", MetricsEndpoint)

	var handler http.Handler = mux
	handler = RequestIDMiddleware(handler)
	handler = MetricsMiddleware(globalMetrics, handler)
	handler = LoggingMiddleware(handler)
	handler = RateLimitMiddleware(rl, globalMetrics, handler)

	listenAddress := fmt.Sprintf("%s:%d", LISTEN_IP, LISTEN_PORT)

	srv := &http.Server{
		Addr:         listenAddress,
		Handler:      handler,
		ReadTimeout:  time.Duration(SERVER_READ_TIMEOUT_SECONDS) * time.Second,
		WriteTimeout: time.Duration(SERVER_WRITE_TIMEOUT_SECONDS) * time.Second,
		IdleTimeout:  time.Duration(SERVER_IDLE_TIMEOUT_SECONDS) * time.Second,
	}

	slog.Info("starting server",
		"address", listenAddress,
		"total_timeout", CRAWL_TOTAL_TIMEOUT_SECONDS,
		"connect_timeout", CRAWL_CONNECT_TIMEOUT_SECONDS,
		"consent_popups", REMOVE_CONSENT_POPUPS,
		"shadow_dom", FLATTEN_SHADOW_DOM,
		"overlay_removal", REMOVE_OVERLAY,
		"ads", AVOID_ADS,
		"css", AVOID_CSS,
		"stealth", ENABLE_STEALTH,
		"retries", MAX_RETRIES,
		"pruning", PRUNING_THRESHOLD,
		"content_filter", CONTENT_FILTER_TYPE,
		"content_source", CONTENT_SOURCE,
		"deep_crawl", DEEP_CRAWL,
		"downstream_auth", CRAWL4AI_AUTH_TOKEN != "",
		"cache_enabled", CACHE_ENABLED,
		"rate_limit_rps", RATE_LIMIT_RPS,
		"upstream_retries", UPSTREAM_RETRIES,
	)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down server...")

	if cache != nil {
		cache.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
