package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"readonly-db-api/internal/database"
	"readonly-db-api/internal/handler"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	idleTimeout := flag.Duration("idle-timeout", 5*time.Minute, "Session idle timeout (session expires after this duration of inactivity)")
	maxSessions := flag.Int("max-sessions", 100, "Maximum number of concurrent sessions (oldest LRU sessions will be evicted when limit is reached)")
	flag.Parse()

	log.Println("Using in-memory database mode")
	sessionManager := database.NewMemorySessionManager(*idleTimeout)
	if err := sessionManager.SetMaxSessions(*maxSessions); err != nil {
		log.Fatalf("Invalid max-sessions config: %v", err)
	}
	h := handler.NewMemoryHandler(sessionManager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionManager.StartCleanupWorker(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/config", h.GetConfig)
	mux.HandleFunc("/config/update", h.UpdateConfig)
	mux.HandleFunc("/stats", h.GetStats)
	mux.HandleFunc("/session/create", h.CreateSession)
	mux.HandleFunc("/session/info", h.GetSessionInfo)
	mux.HandleFunc("/session/close", h.CloseSession)
	mux.HandleFunc("/query", h.ExecuteQuery)

	server := &http.Server{
		Addr:              *addr,
		Handler:           withLogging(mux),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Starting read-only database API server on %s", *addr)
		log.Printf("Session idle timeout: %v", *idleTimeout)
		log.Printf("Maximum concurrent sessions: %d", *maxSessions)
		log.Println()
		log.Println("Available endpoints:")
		log.Println("  GET  /health            - Health check (includes active sessions count)")
		log.Println("  GET  /config            - Get current default timeout configuration")
		log.Println("  POST /config/update     - Update default timeout config (idle_timeout, max_lifetime, max_sessions)")
		log.Println("  GET  /stats             - Session manager statistics")
		log.Println("  POST /session/create    - Create a read-only session (optional: custom idle_timeout, max_lifetime)")
		log.Println("  GET  /session/info      - Get current session info (requires X-Session-Id header)")
		log.Println("  POST /session/close     - Close an existing session")
		log.Println("  POST /query             - Execute a SELECT query (requires X-Session-Id header)")
		log.Println()
		log.Println("Session management features:")
		log.Println("  - Idle timeout: sessions automatically expire after idle period")
		log.Println("  - Max lifetime: sessions have a maximum absolute lifetime")
		log.Println("  - Custom timeout: create sessions with per-session custom timeout")
		log.Println("  - Runtime config: update default timeouts dynamically via API")
		log.Println("  - LRU eviction: oldest sessions evicted when max limit reached")
		log.Println("  - Background cleanup: expired sessions cleaned every 10 seconds")
		log.Println("  - Safe limits: all timeouts validated with min/max bounds")
		log.Println()
		log.Println("Available tables: users, products, orders")
		log.Println()
		log.Println("Sample queries:")
		log.Println("  SELECT * FROM users")
		log.Println("  SELECT id, username, email FROM users WHERE age > 30")
		log.Println("  SELECT * FROM users WHERE age > 25 AND is_active = 1")
		log.Println("  SELECT name, category, price FROM products WHERE price > 100 LIMIT 5")
		log.Println("  SELECT * FROM orders WHERE status = 'delivered'")
		log.Println()
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	cancel()
	log.Println("Server gracefully stopped")
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Started %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)

		log.Printf("Completed %s %s in %v with status %d",
			r.Method, r.URL.Path, time.Since(start), rw.status)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
