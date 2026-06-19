//go:build sqlite

package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type ReadOnlySession struct {
	ID        string
	DB        *sql.DB
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionManager struct {
	sessions map[string]*ReadOnlySession
	mu       sync.RWMutex
	dbPath   string
	timeout  time.Duration
}

func NewSessionManager(dbPath string, timeout time.Duration) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*ReadOnlySession),
		dbPath:   dbPath,
		timeout:  timeout,
	}
}

func (sm *SessionManager) CreateSession() (*ReadOnlySession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	db, err := sql.Open("sqlite", sm.dbPath+"?_pragma=query_only(true)&_pragma=read_uncommitted(false)")
	if err != nil {
		return nil, fmt.Errorf("failed to open read-only database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(sm.timeout)

	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	sessionID := generateSessionID()
	now := time.Now()
	session := &ReadOnlySession{
		ID:        sessionID,
		DB:        db,
		CreatedAt: now,
		ExpiresAt: now.Add(sm.timeout),
	}

	sm.sessions[sessionID] = session
	return session, nil
}

func (sm *SessionManager) GetSession(sessionID string) (*ReadOnlySession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	if time.Now().After(session.ExpiresAt) {
		sm.mu.RUnlock()
		sm.mu.Lock()
		delete(sm.sessions, sessionID)
		session.DB.Close()
		sm.mu.Unlock()
		sm.mu.RLock()
		return nil, fmt.Errorf("session expired")
	}

	session.ExpiresAt = time.Now().Add(sm.timeout)
	return session, nil
}

func (sm *SessionManager) CloseSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	delete(sm.sessions, sessionID)
	return session.DB.Close()
}

func (sm *SessionManager) CleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			session.DB.Close()
			delete(sm.sessions, id)
		}
	}
}

func (sm *SessionManager) StartCleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sm.CleanupExpiredSessions()
			case <-ctx.Done():
				sm.mu.Lock()
				defer sm.mu.Unlock()
				for _, session := range sm.sessions {
					session.DB.Close()
				}
				sm.sessions = make(map[string]*ReadOnlySession)
				return
			}
		}
	}()
}

func (s *ReadOnlySession) ExecuteQuery(ctx context.Context, query string) ([]map[string]interface{}, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{
		ReadOnly: true,
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to begin read-only transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			row[col] = convertValue(val, columnTypes[i])
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}

func convertValue(val interface{}, ct *sql.ColumnType) interface{} {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case []byte:
		return string(v)
	case int64:
		return v
	case float64:
		return v
	case bool:
		return v
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return v
	}
}
