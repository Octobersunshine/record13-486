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
	timeout := flag.Duration("timeout", 30*time.Minute, "Session timeout duration")
	flag.Parse()

	log.Println("Using in-memory database mode")
	sessionManager := database.NewMemorySessionManager(*timeout)
	h := handler.NewMemoryHandler(sessionManager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessionManager.StartCleanupWorker(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/session/create", h.CreateSession)
	mux.HandleFunc("/query", h.ExecuteQuery)
	mux.HandleFunc("/session/close", h.CloseSession)

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
		log.Printf("Session timeout: %v", *timeout)
		log.Println()
		log.Println("Available endpoints:")
		log.Println("  GET  /health          - Health check")
		log.Println("  POST /session/create  - Create a new read-only session")
		log.Println("  POST /query           - Execute a SELECT query (requires X-Session-Id header)")
		log.Println("  POST /session/close   - Close an existing session")
		log.Println()
		log.Println("Available tables: users, products, orders")
		log.Println()
		log.Println("Sample queries:")
		log.Println("  SELECT * FROM users")
		log.Println("  SELECT id, username, email FROM users WHERE age > 30")
		log.Println("  SELECT category, COUNT(*) as count FROM products GROUP BY category")
		log.Println("  SELECT * FROM orders WHERE status = 'delivered' LIMIT 10")
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
