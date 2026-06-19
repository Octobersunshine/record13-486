package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"readonly-db-api/internal/database"
	"readonly-db-api/internal/sqlparser"
)

type QueryExecutor interface {
	ExecuteQuery(ctx context.Context, query string) ([]map[string]interface{}, error)
}

type MemorySessionManager interface {
	CreateSession() (*database.MemorySession, error)
	GetSession(sessionID string) (*database.MemorySession, error)
	CloseSession(sessionID string) error
	StartCleanupWorker(ctx context.Context)
}

type Handler struct {
	sessionManager MemorySessionManager
}

func NewMemoryHandler(sm *database.MemorySessionManager) *Handler {
	return &Handler{
		sessionManager: sm,
	}
}

type CreateSessionResponse struct {
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Message   string    `json:"message"`
}

type QueryRequest struct {
	SQL string `json:"sql"`
}

type QueryResponse struct {
	Columns []string               `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                    `json:"count"`
	Time    float64                `json:"execution_time_ms"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "only POST method is accepted")
		return
	}

	session, err := h.sessionManager.CreateSession()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "failed to create session", err.Error())
		return
	}

	response := CreateSessionResponse{
		SessionID: session.ID,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
		Message:   "Read-only session created successfully. Use session_id in X-Session-Id header for queries.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Session-Id", session.ID)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) ExecuteQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "only POST method is accepted")
		return
	}

	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sendError(w, http.StatusBadRequest, "missing session id", "X-Session-Id header is required")
		return
	}

	executor, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		sendError(w, http.StatusUnauthorized, "invalid session", err.Error())
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	if err := sqlparser.IsReadOnlyQuery(req.SQL); err != nil {
		sendError(w, http.StatusForbidden, "query not allowed", err.Error())
		return
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	results, err := executor.ExecuteQuery(ctx, req.SQL)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "query execution failed", err.Error())
		return
	}

	executionTime := float64(time.Since(startTime).Microseconds()) / 1000.0

	var columns []string
	if len(results) > 0 {
		columns = make([]string, 0, len(results[0]))
		for col := range results[0] {
			columns = append(columns, col)
		}
	}

	response := QueryResponse{
		Columns: columns,
		Rows:    results,
		Count:   len(results),
		Time:    executionTime,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) CloseSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "only POST method is accepted")
		return
	}

	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sendError(w, http.StatusBadRequest, "missing session id", "X-Session-Id header is required")
		return
	}

	if err := h.sessionManager.CloseSession(sessionID); err != nil {
		sendError(w, http.StatusNotFound, "session not found", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Session closed successfully",
		"status":  "ok",
	})
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "readonly-db-api",
		"mode":      "memory",
	})
}

func sendError(w http.ResponseWriter, code int, err string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   err,
		Code:    code,
		Message: message,
	})
}
