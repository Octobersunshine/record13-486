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
	CreateSessionWithTimeout(idleTimeout, maxLifetime time.Duration) (*database.MemorySession, error)
	GetSession(sessionID string) (*database.MemorySession, error)
	CloseSession(sessionID string) error
	StartCleanupWorker(ctx context.Context)
	GetStats() database.SessionStats
	GetConfig() database.SessionConfig
	SetMaxSessions(max int) error
	SetDefaultTimeout(idleTimeout, maxLifetime time.Duration) error
}

type Handler struct {
	sessionManager MemorySessionManager
}

func NewMemoryHandler(sm *database.MemorySessionManager) *Handler {
	return &Handler{
		sessionManager: sm,
	}
}

type CreateSessionRequest struct {
	IdleTimeoutSeconds int `json:"idle_timeout_seconds,omitempty"`
	MaxLifetimeSeconds int `json:"max_lifetime_seconds,omitempty"`
	IdleTimeout        string `json:"idle_timeout,omitempty"`
	MaxLifetime        string `json:"max_lifetime,omitempty"`
}

type CreateSessionResponse struct {
	SessionID        string    `json:"session_id"`
	CreatedAt        time.Time `json:"created_at"`
	LastAccessedAt   time.Time `json:"last_accessed_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	IdleExpiresAt    time.Time `json:"idle_expires_at"`
	IdleTimeout      string    `json:"idle_timeout"`
	MaxLifetime      string    `json:"max_lifetime"`
	IsCustomTimeout  bool      `json:"is_custom_timeout"`
	Message          string    `json:"message"`
}

type UpdateConfigRequest struct {
	IdleTimeoutSeconds *int   `json:"idle_timeout_seconds,omitempty"`
	MaxLifetimeSeconds *int   `json:"max_lifetime_seconds,omitempty"`
	MaxSessions        *int   `json:"max_sessions,omitempty"`
	IdleTimeout        string `json:"idle_timeout,omitempty"`
	MaxLifetime        string `json:"max_lifetime,omitempty"`
}

type ConfigResponse struct {
	IdleTimeout     string `json:"idle_timeout"`
	MaxLifetime     string `json:"max_lifetime"`
	MaxSessions     int    `json:"max_sessions"`
	IdleTimeoutSec  int64  `json:"idle_timeout_seconds"`
	MaxLifetimeSec  int64  `json:"max_lifetime_seconds"`
}

type QueryRequest struct {
	SQL string `json:"sql"`
}

type QueryResponse struct {
	Columns      []string                 `json:"columns"`
	Rows         []map[string]interface{} `json:"rows"`
	Count        int                      `json:"count"`
	Time         float64                  `json:"execution_time_ms"`
	QueryCount   int64                    `json:"session_query_count"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

func parseDuration(str string, seconds int) (time.Duration, bool) {
	var d time.Duration
	hasValue := false

	if str != "" {
		parsed, err := time.ParseDuration(str)
		if err == nil {
			d = parsed
			hasValue = true
		}
	}
	if seconds > 0 && !hasValue {
		d = time.Duration(seconds) * time.Second
		hasValue = true
	}
	return d, hasValue
}

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "only POST method is accepted")
		return
	}

	var (
		session *database.MemorySession
		err     error
	)

	if r.ContentLength > 0 {
		var req CreateSessionRequest
		if json.NewDecoder(r.Body).Decode(&req) == nil {
			defer r.Body.Close()
			idleTimeout, hasIdle := parseDuration(req.IdleTimeout, req.IdleTimeoutSeconds)
			maxLifetime, hasMax := parseDuration(req.MaxLifetime, req.MaxLifetimeSeconds)

			if hasIdle || hasMax {
				session, err = h.sessionManager.CreateSessionWithTimeout(idleTimeout, maxLifetime)
				if err != nil {
					sendError(w, http.StatusBadRequest, "invalid timeout configuration", err.Error())
					return
				}
			}
		}
	}

	if session == nil {
		session, err = h.sessionManager.CreateSession()
		if err != nil {
			sendError(w, http.StatusInternalServerError, "failed to create session", err.Error())
			return
		}
	}

	response := CreateSessionResponse{
		SessionID:      session.ID,
		CreatedAt:      session.CreatedAt,
		LastAccessedAt: session.LastAccessedAt,
		ExpiresAt:      session.ExpiresAt,
		IdleExpiresAt:  session.IdleExpiresAt,
		IdleTimeout:    session.IdleTimeout.String(),
		MaxLifetime:    session.MaxLifetime.String(),
		IsCustomTimeout: session.IsCustomTimeout,
		Message:        "Read-only session created successfully. Use session_id in X-Session-Id header for queries.",
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
		Columns:     columns,
		Rows:        results,
		Count:       len(results),
		Time:        executionTime,
		QueryCount:  executor.QueryCount,
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

	stats := h.sessionManager.GetStats()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "healthy",
		"timestamp":       time.Now().Format(time.RFC3339),
		"service":         "readonly-db-api",
		"mode":            "memory",
		"active_sessions": stats.TotalSessions,
		"uptime":          stats.Uptime,
	})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "only GET method is accepted")
		return
	}

	stats := h.sessionManager.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_manager": stats,
	})
}

func (h *Handler) GetSessionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "only GET method is accepted")
		return
	}

	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sendError(w, http.StatusBadRequest, "missing session id", "X-Session-Id header is required")
		return
	}

	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		sendError(w, http.StatusUnauthorized, "invalid session", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id":                  session.ID,
		"created_at":                  session.CreatedAt,
		"last_accessed_at":            session.LastAccessedAt,
		"expires_at":                  session.ExpiresAt,
		"idle_expires_at":             session.IdleExpiresAt,
		"query_count":                 session.QueryCount,
		"idle_timeout":                session.IdleTimeout.String(),
		"max_lifetime":                session.MaxLifetime.String(),
		"is_custom_timeout":           session.IsCustomTimeout,
		"idle_time_seconds":           time.Since(session.LastAccessedAt).Seconds(),
		"remaining_idle_seconds":      time.Until(session.IdleExpiresAt).Seconds(),
		"remaining_lifetime_seconds":  time.Until(session.ExpiresAt).Seconds(),
	})
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "only GET method is accepted")
		return
	}

	config := h.sessionManager.GetConfig()

	response := ConfigResponse{
		IdleTimeout:    config.IdleTimeout.String(),
		MaxLifetime:    config.MaxLifetime.String(),
		MaxSessions:    config.MaxSessions,
		IdleTimeoutSec: int64(config.IdleTimeout.Seconds()),
		MaxLifetimeSec: int64(config.MaxLifetime.Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "method not allowed", "only PUT or POST method is accepted")
		return
	}

	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	currentConfig := h.sessionManager.GetConfig()
	idleTimeout := currentConfig.IdleTimeout
	maxLifetime := currentConfig.MaxLifetime
	maxSessions := currentConfig.MaxSessions

	updated := false

	if d, ok := parseDuration(req.IdleTimeout, derefInt(req.IdleTimeoutSeconds)); ok {
		idleTimeout = d
		updated = true
	}
	if d, ok := parseDuration(req.MaxLifetime, derefInt(req.MaxLifetimeSeconds)); ok {
		maxLifetime = d
		updated = true
	}
	if req.MaxSessions != nil && *req.MaxSessions > 0 {
		maxSessions = *req.MaxSessions
		updated = true
	}

	if !updated {
		sendError(w, http.StatusBadRequest, "no valid configuration fields provided",
			"provide at least one of: idle_timeout, max_lifetime, max_sessions")
		return
	}

	if err := database.ValidateSessionConfig(idleTimeout, maxLifetime, maxSessions); err != nil {
		sendError(w, http.StatusBadRequest, "invalid configuration", err.Error())
		return
	}

	if err := h.sessionManager.SetDefaultTimeout(idleTimeout, maxLifetime); err != nil {
		sendError(w, http.StatusInternalServerError, "failed to update timeout config", err.Error())
		return
	}

	if err := h.sessionManager.SetMaxSessions(maxSessions); err != nil {
		sendError(w, http.StatusInternalServerError, "failed to update max sessions", err.Error())
		return
	}

	config := h.sessionManager.GetConfig()
	response := ConfigResponse{
		IdleTimeout:    config.IdleTimeout.String(),
		MaxLifetime:    config.MaxLifetime.String(),
		MaxSessions:    config.MaxSessions,
		IdleTimeoutSec: int64(config.IdleTimeout.Seconds()),
		MaxLifetimeSec: int64(config.MaxLifetime.Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Configuration updated successfully",
		"config":  response,
	})
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
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
