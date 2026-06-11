package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gh2lark/config"
	"gh2lark/lark"
	"gh2lark/transformer"
	"gh2lark/webhook"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	client := lark.NewClient(cfg.LarkWebhookURL)
	h := &handler{cfg: cfg, client: client}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /webhook", h.webhook)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("gh2lark starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

type handler struct {
	cfg    *config.Config
	client *lark.Client
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"ok"}`)
}

func (h *handler) webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		http.Error(w, "missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	// Read body (capped at MaxPayloadSize)
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxPayloadSize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("failed to read body", "error", err)
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Validate signature
	sigHeader := r.Header.Get("X-Hub-Signature-256")
	if !webhook.ValidateSignature(h.cfg.GitHubWebhookSecret, body, sigHeader) {
		slog.Warn("signature validation failed")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	deliveryID := r.Header.Get("X-GitHub-Delivery")
	slog.Info("webhook received", "event", eventType, "delivery", deliveryID, "size", len(body))

	// Transform
	card, err := transformer.Transform(eventType, body)
	if err != nil {
		slog.Error("transform failed", "event", eventType, "error", err)
		http.Error(w, "transform error", http.StatusInternalServerError)
		return
	}

	// Send to Lark
	if err := h.client.SendCard(card); err != nil {
		slog.Error("lark send failed", "event", eventType, "delivery", deliveryID, "error", err)
		// Still return 200 to GitHub — the event was received and processed,
		// Lark delivery failure is a separate concern.
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Ensure packages are referenced (defensive compile-time check).
var _ = webhook.ValidateSignature
var _ = transformer.Transform
