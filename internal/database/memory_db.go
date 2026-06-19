package database

import (
	"container/list"
	"context"
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MemoryDB struct {
	tables map[string][]map[string]interface{}
	mu     sync.RWMutex
}

func NewMemoryDB() *MemoryDB {
	db := &MemoryDB{
		tables: make(map[string][]map[string]interface{}),
	}
	db.initSampleData()
	return db
}

func (m *MemoryDB) initSampleData() {
	m.tables["users"] = []map[string]interface{}{
		{"id": int64(1), "username": "alice", "email": "alice@example.com", "age": int64(25), "created_at": "2023-06-15T10:30:00Z", "is_active": int64(1)},
		{"id": int64(2), "username": "bob", "email": "bob@example.com", "age": int64(30), "created_at": "2023-07-20T14:20:00Z", "is_active": int64(1)},
		{"id": int64(3), "username": "charlie", "email": "charlie@example.com", "age": int64(35), "created_at": "2023-08-10T09:15:00Z", "is_active": int64(0)},
		{"id": int64(4), "username": "david", "email": "david@example.com", "age": int64(28), "created_at": "2023-09-05T16:45:00Z", "is_active": int64(1)},
		{"id": int64(5), "username": "emma", "email": "emma@example.com", "age": int64(22), "created_at": "2023-10-12T11:00:00Z", "is_active": int64(1)},
		{"id": int64(6), "username": "frank", "email": "frank@example.com", "age": int64(40), "created_at": "2023-11-18T08:30:00Z", "is_active": int64(0)},
		{"id": int64(7), "username": "grace", "email": "grace@example.com", "age": int64(33), "created_at": "2023-12-01T13:25:00Z", "is_active": int64(1)},
		{"id": int64(8), "username": "henry", "email": "henry@example.com", "age": int64(45), "created_at": "2024-01-15T10:10:00Z", "is_active": int64(1)},
		{"id": int64(9), "username": "ivy", "email": "ivy@example.com", "age": int64(27), "created_at": "2024-02-20T15:55:00Z", "is_active": int64(1)},
		{"id": int64(10), "username": "jack", "email": "jack@example.com", "age": int64(38), "created_at": "2024-03-10T12:40:00Z", "is_active": int64(0)},
	}

	m.tables["products"] = []map[string]interface{}{
		{"id": int64(1), "name": "Laptop", "category": "Electronics", "price": 999.99, "stock": int64(50), "description": "High-performance laptop"},
		{"id": int64(2), "name": "Smartphone", "category": "Electronics", "price": 699.99, "stock": int64(120), "description": "Latest smartphone"},
		{"id": int64(3), "name": "Headphones", "category": "Electronics", "price": 199.99, "stock": int64(200), "description": "Noise-cancelling headphones"},
		{"id": int64(4), "name": "Novel", "category": "Books", "price": 29.99, "stock": int64(300), "description": "Bestselling novel"},
		{"id": int64(5), "name": "T-shirt", "category": "Clothing", "price": 19.99, "stock": int64(500), "description": "Cotton t-shirt"},
		{"id": int64(6), "name": "Coffee", "category": "Food", "price": 9.99, "stock": int64(1000), "description": "Premium coffee beans"},
		{"id": int64(7), "name": "Basketball", "category": "Sports", "price": 49.99, "stock": int64(150), "description": "Professional basketball"},
		{"id": int64(8), "name": "Lamp", "category": "Home", "price": 79.99, "stock": int64(80), "description": "LED desk lamp"},
		{"id": int64(9), "name": "Shampoo", "category": "Beauty", "price": 14.99, "stock": int64(400), "description": "Hydrating shampoo"},
		{"id": int64(10), "name": "Doll", "category": "Toys", "price": 39.99, "stock": int64(250), "description": "Collectible doll"},
	}

	m.tables["orders"] = []map[string]interface{}{}
	for i := 1; i <= 50; i++ {
		userID := int64(1 + (i-1)%10)
		productID := int64(1 + (i-1)%10)
		quantity := int64(1 + (i-1)%5)
		price := float64(10 + (i-1)*20)
		m.tables["orders"] = append(m.tables["orders"], map[string]interface{}{
			"id":          int64(i),
			"user_id":     userID,
			"product_id":  productID,
			"quantity":    quantity,
			"total_price": float64(quantity) * price,
			"order_date":  time.Now().AddDate(0, 0, -i).Format(time.RFC3339),
			"status":      []string{"pending", "shipped", "delivered", "cancelled"}[i%4],
		})
	}
}

type MemorySession struct {
	ID                string
	CreatedAt         time.Time
	LastAccessedAt    time.Time
	ExpiresAt         time.Time
	IdleExpiresAt     time.Time
	QueryCount        int64
	IdleTimeout       time.Duration
	MaxLifetime       time.Duration
	IsCustomTimeout   bool
	db                *MemoryDB
}

type SessionStats struct {
	TotalSessions       int       `json:"total_sessions"`
	MaxSessions         int       `json:"max_sessions"`
	IdleTimeout         string    `json:"idle_timeout"`
	MaxLifetime         string    `json:"max_lifetime"`
	TotalCreated        int64     `json:"total_created"`
	TotalClosed         int64     `json:"total_closed"`
	TotalExpired        int64     `json:"total_expired"`
	TotalEvicted        int64     `json:"total_evicted"`
	TotalQueries        int64     `json:"total_queries"`
	LastCleanup         time.Time `json:"last_cleanup"`
	CleanupCount        int64     `json:"cleanup_count"`
	Uptime              string    `json:"uptime"`
	StartTime           time.Time `json:"start_time"`
}

type lruEntry struct {
	sessionID string
	element   *list.Element
}

type MemorySessionManager struct {
	sessions     map[string]*MemorySession
	sessionIndex map[string]*lruEntry
	lruList      *list.List
	mu           sync.Mutex
	db           *MemoryDB

	idleTimeout   time.Duration
	maxLifetime   time.Duration
	maxSessions   int

	stats         SessionStats
	startTime     time.Time
}

type SessionConfig struct {
	IdleTimeout time.Duration `json:"idle_timeout"`
	MaxLifetime time.Duration `json:"max_lifetime"`
	MaxSessions int           `json:"max_sessions"`
}

const (
	DefaultIdleTimeout     = 5 * time.Minute
	DefaultMaxLifetime     = 30 * time.Minute
	DefaultMaxSessions     = 100
	MinIdleTimeout         = 1 * time.Second
	MaxIdleTimeout         = 2 * time.Hour
	MinMaxLifetime         = 10 * time.Second
	MaxMaxLifetime         = 24 * time.Hour
	MinMaxSessions         = 1
	MaxMaxSessions         = 10000
	MaxIdleLifetimeRatio   = 100
)

func NewMemorySessionManager(idleTimeout time.Duration) *MemorySessionManager {
	if idleTimeout <= 0 {
		idleTimeout = DefaultIdleTimeout
	}

	maxLifetime := idleTimeout * 6
	if maxLifetime < 10*time.Minute {
		maxLifetime = 10 * time.Minute
	}
	if maxLifetime > 2*time.Hour {
		maxLifetime = 2 * time.Hour
	}

	return &MemorySessionManager{
		sessions:     make(map[string]*MemorySession),
		sessionIndex: make(map[string]*lruEntry),
		lruList:      list.New(),
		db:           NewMemoryDB(),
		idleTimeout:  idleTimeout,
		maxLifetime:  maxLifetime,
		maxSessions:  DefaultMaxSessions,
		startTime:    time.Now(),
		stats: SessionStats{
			MaxSessions: DefaultMaxSessions,
			IdleTimeout: idleTimeout.String(),
			MaxLifetime: maxLifetime.String(),
			StartTime:   time.Now(),
		},
	}
}

func ValidateSessionConfig(idleTimeout, maxLifetime time.Duration, maxSessions int) error {
	if idleTimeout < MinIdleTimeout {
		return fmt.Errorf("idle_timeout too short: %v (minimum: %v)", idleTimeout, MinIdleTimeout)
	}
	if idleTimeout > MaxIdleTimeout {
		return fmt.Errorf("idle_timeout too long: %v (maximum: %v)", idleTimeout, MaxIdleTimeout)
	}
	if maxLifetime < MinMaxLifetime {
		return fmt.Errorf("max_lifetime too short: %v (minimum: %v)", maxLifetime, MinMaxLifetime)
	}
	if maxLifetime > MaxMaxLifetime {
		return fmt.Errorf("max_lifetime too long: %v (maximum: %v)", maxLifetime, MaxMaxLifetime)
	}
	if maxLifetime < idleTimeout {
		return fmt.Errorf("max_lifetime (%v) must be >= idle_timeout (%v)", maxLifetime, idleTimeout)
	}
	if maxLifetime > idleTimeout*time.Duration(MaxIdleLifetimeRatio) {
		return fmt.Errorf("max_lifetime (%v) too large compared to idle_timeout (%v, max ratio: %d)",
			maxLifetime, idleTimeout, MaxIdleLifetimeRatio)
	}
	if maxSessions < MinMaxSessions {
		return fmt.Errorf("max_sessions too small: %d (minimum: %d)", maxSessions, MinMaxSessions)
	}
	if maxSessions > MaxMaxSessions {
		return fmt.Errorf("max_sessions too large: %d (maximum: %d)", maxSessions, MaxMaxSessions)
	}
	return nil
}

func (sm *MemorySessionManager) SetMaxSessions(max int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if err := ValidateSessionConfig(sm.idleTimeout, sm.maxLifetime, max); err != nil {
		return err
	}
	sm.maxSessions = max
	sm.stats.MaxSessions = max
	return nil
}

func (sm *MemorySessionManager) SetDefaultTimeout(idleTimeout, maxLifetime time.Duration) error {
	if err := ValidateSessionConfig(idleTimeout, maxLifetime, sm.maxSessions); err != nil {
		return err
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.idleTimeout = idleTimeout
	sm.maxLifetime = maxLifetime
	sm.stats.IdleTimeout = idleTimeout.String()
	sm.stats.MaxLifetime = maxLifetime.String()
	log.Printf("[SessionManager] Default timeouts updated: idle=%v, max_lifetime=%v", idleTimeout, maxLifetime)
	return nil
}

func (sm *MemorySessionManager) GetConfig() SessionConfig {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return SessionConfig{
		IdleTimeout: sm.idleTimeout,
		MaxLifetime: sm.maxLifetime,
		MaxSessions: sm.maxSessions,
	}
}

func (sm *MemorySessionManager) CreateSession() (*MemorySession, error) {
	return sm.createSessionInternal(0, 0)
}

func (sm *MemorySessionManager) CreateSessionWithTimeout(idleTimeout, maxLifetime time.Duration) (*MemorySession, error) {
	return sm.createSessionInternal(idleTimeout, maxLifetime)
}

func (sm *MemorySessionManager) createSessionInternal(customIdleTimeout, customMaxLifetime time.Duration) (*MemorySession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.sessions) >= sm.maxSessions {
		if !sm.evictOldestSessionLocked() {
			return nil, fmt.Errorf("maximum session limit (%d) reached", sm.maxSessions)
		}
	}

	idleTimeout := sm.idleTimeout
	maxLifetime := sm.maxLifetime
	isCustom := false

	if customIdleTimeout > 0 || customMaxLifetime > 0 {
		if customIdleTimeout <= 0 {
			customIdleTimeout = sm.idleTimeout
		}
		if customMaxLifetime <= 0 {
			customMaxLifetime = customIdleTimeout * 6
			if customMaxLifetime < sm.maxLifetime {
				customMaxLifetime = MinMaxLifetime
			}
		}
		if err := ValidateSessionConfig(customIdleTimeout, customMaxLifetime, sm.maxSessions); err != nil {
			return nil, err
		}
		idleTimeout = customIdleTimeout
		maxLifetime = customMaxLifetime
		isCustom = true
	}

	sessionID := generateSessionID()
	now := time.Now()
	session := &MemorySession{
		ID:              sessionID,
		CreatedAt:       now,
		LastAccessedAt:  now,
		ExpiresAt:       now.Add(maxLifetime),
		IdleExpiresAt:   now.Add(idleTimeout),
		QueryCount:      0,
		IdleTimeout:     idleTimeout,
		MaxLifetime:     maxLifetime,
		IsCustomTimeout: isCustom,
		db:              sm.db,
	}

	elem := sm.lruList.PushFront(sessionID)
	sm.sessions[sessionID] = session
	sm.sessionIndex[sessionID] = &lruEntry{
		sessionID: sessionID,
		element:   elem,
	}

	sm.stats.TotalCreated++
	sm.stats.TotalSessions = len(sm.sessions)

	if isCustom {
		log.Printf("[SessionManager] Session created with custom timeouts: id=%s idle=%v max_lifetime=%v",
			sessionID, idleTimeout, maxLifetime)
	}

	return session, nil
}

func (sm *MemorySessionManager) evictOldestSessionLocked() bool {
	if sm.lruList.Len() == 0 {
		return false
	}

	backElem := sm.lruList.Back()
	if backElem == nil {
		return false
	}

	sessionID := backElem.Value.(string)
	session, exists := sm.sessions[sessionID]
	if !exists {
		sm.lruList.Remove(backElem)
		delete(sm.sessionIndex, sessionID)
		return false
	}

	log.Printf("[SessionManager] Evicting LRU session: %s (idle for %v, queries: %d)",
		sessionID, time.Since(session.LastAccessedAt), session.QueryCount)

	sm.lruList.Remove(backElem)
	delete(sm.sessions, sessionID)
	delete(sm.sessionIndex, sessionID)
	sm.stats.TotalEvicted++
	sm.stats.TotalSessions = len(sm.sessions)

	return true
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("mem-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (sm *MemorySessionManager) GetSession(sessionID string) (*MemorySession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	now := time.Now()

	if now.After(session.ExpiresAt) {
		log.Printf("[SessionManager] Session expired (max lifetime): %s (lifetime: %v)",
			sessionID, now.Sub(session.CreatedAt))
		sm.removeSessionLocked(sessionID)
		sm.stats.TotalExpired++
		return nil, fmt.Errorf("session expired (maximum lifetime reached)")
	}

	if now.After(session.IdleExpiresAt) {
		log.Printf("[SessionManager] Session expired (idle timeout): %s (idle: %v)",
			sessionID, now.Sub(session.LastAccessedAt))
		sm.removeSessionLocked(sessionID)
		sm.stats.TotalExpired++
		return nil, fmt.Errorf("session expired (idle timeout)")
	}

	session.LastAccessedAt = now
	session.IdleExpiresAt = now.Add(session.IdleTimeout)

	if entry, ok := sm.sessionIndex[sessionID]; ok {
		sm.lruList.MoveToFront(entry.element)
	}

	return session, nil
}

func (sm *MemorySessionManager) removeSessionLocked(sessionID string) {
	if entry, ok := sm.sessionIndex[sessionID]; ok {
		sm.lruList.Remove(entry.element)
		delete(sm.sessionIndex, sessionID)
	}
	delete(sm.sessions, sessionID)
	sm.stats.TotalSessions = len(sm.sessions)
}

func (sm *MemorySessionManager) CloseSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	sm.removeSessionLocked(sessionID)
	sm.stats.TotalClosed++
	return nil
}

func (sm *MemorySessionManager) CleanupExpiredSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	cleanedCount := 0

	for sessionID, session := range sm.sessions {
		if now.After(session.ExpiresAt) || now.After(session.IdleExpiresAt) {
			reason := "idle"
			if now.After(session.ExpiresAt) {
				reason = "max lifetime"
			}
			log.Printf("[SessionManager] Cleanup removing session: %s (reason: %s, idle: %v)",
				sessionID, reason, now.Sub(session.LastAccessedAt))
			sm.removeSessionLocked(sessionID)
			sm.stats.TotalExpired++
			cleanedCount++
		}
	}

	sm.stats.LastCleanup = now
	sm.stats.CleanupCount++
	sm.stats.Uptime = time.Since(sm.startTime).String()

	if cleanedCount > 0 {
		log.Printf("[SessionManager] Cleanup complete: removed %d expired sessions, remaining: %d",
			cleanedCount, len(sm.sessions))
	}

	return cleanedCount
}

func (sm *MemorySessionManager) GetStats() SessionStats {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.stats.TotalSessions = len(sm.sessions)
	sm.stats.Uptime = time.Since(sm.startTime).String()
	return sm.stats
}

func (sm *MemorySessionManager) StartCleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		log.Printf("[SessionManager] Cleanup worker started (interval: 10s, idle timeout: %v, max lifetime: %v, max sessions: %d)",
			sm.idleTimeout, sm.maxLifetime, sm.maxSessions)

		for {
			select {
			case <-ticker.C:
				sm.CleanupExpiredSessions()
			case <-ctx.Done():
				sm.mu.Lock()
				defer sm.mu.Unlock()
				log.Printf("[SessionManager] Cleanup worker shutting down, closing %d remaining sessions", len(sm.sessions))
				for id := range sm.sessions {
					sm.removeSessionLocked(id)
				}
				sm.stats.TotalSessions = 0
				return
			}
		}
	}()
}

func (s *MemorySession) RecordQuery() {
	s.QueryCount++
}

func (s *MemorySession) ExecuteQuery(ctx context.Context, query string) ([]map[string]interface{}, error) {
	s.RecordQuery()

	s.db.mu.RLock()
	defer s.db.mu.RUnlock()

	query = strings.TrimSpace(query)

	if strings.HasPrefix(strings.ToUpper(query), "WITH") {
		return nil, fmt.Errorf("WITH clauses not supported in memory mode")
	}

	tableName, columns, whereClause, err := parseSelectQuery(query)
	if err != nil {
		return nil, err
	}

	table, exists := s.db.tables[tableName]
	if !exists {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	var results []map[string]interface{}
	for _, row := range table {
		if whereClause != nil {
			if !evaluateWhere(row, whereClause) {
				continue
			}
		}

		resultRow := make(map[string]interface{})
		if len(columns) == 1 && columns[0] == "*" {
			for k, v := range row {
				resultRow[k] = convertMemoryValue(v)
			}
		} else {
			for _, col := range columns {
				if val, exists := row[col]; exists {
					resultRow[col] = convertMemoryValue(val)
				} else {
					return nil, fmt.Errorf("column not found: %s", col)
				}
			}
		}
		results = append(results, resultRow)
	}

	return results, nil
}

func parseSelectQuery(query string) (string, []string, *WhereCondition, error) {
	upperQuery := strings.ToUpper(query)

	if !strings.HasPrefix(upperQuery, "SELECT ") {
		return "", nil, nil, fmt.Errorf("only SELECT queries are supported")
	}

	fromIdx := strings.Index(upperQuery, " FROM ")
	if fromIdx == -1 {
		return "", nil, nil, fmt.Errorf("invalid SELECT query: missing FROM clause")
	}

	selectPart := strings.TrimSpace(query[7:fromIdx])
	columns := parseColumns(selectPart)

	afterFrom := strings.TrimSpace(query[fromIdx+6:])

	var tableName string
	var whereClause *WhereCondition

	whereIdx := strings.Index(strings.ToUpper(afterFrom), " WHERE ")
	if whereIdx != -1 {
		tableName = strings.TrimSpace(afterFrom[:whereIdx])
		wherePart := strings.TrimSpace(afterFrom[whereIdx+7:])

		limitIdx := strings.Index(strings.ToUpper(wherePart), " LIMIT ")
		if limitIdx != -1 {
			wherePart = strings.TrimSpace(wherePart[:limitIdx])
		}

		orderByIdx := strings.Index(strings.ToUpper(wherePart), " ORDER BY ")
		if orderByIdx != -1 {
			wherePart = strings.TrimSpace(wherePart[:orderByIdx])
		}

		whereClause = parseWhereClause(wherePart)
	} else {
		limitIdx := strings.Index(strings.ToUpper(afterFrom), " LIMIT ")
		if limitIdx != -1 {
			tableName = strings.TrimSpace(afterFrom[:limitIdx])
		} else {
			orderByIdx := strings.Index(strings.ToUpper(afterFrom), " ORDER BY ")
			if orderByIdx != -1 {
				tableName = strings.TrimSpace(afterFrom[:orderByIdx])
			} else {
				tableName = strings.TrimSpace(afterFrom)
			}
		}
	}

	return tableName, columns, whereClause, nil
}

func parseColumns(selectPart string) []string {
	if selectPart == "*" {
		return []string{"*"}
	}

	var columns []string
	var current strings.Builder
	inParens := 0

	for _, ch := range selectPart {
		switch ch {
		case '(':
			inParens++
			current.WriteRune(ch)
		case ')':
			inParens--
			current.WriteRune(ch)
		case ',':
			if inParens == 0 {
				col := strings.TrimSpace(current.String())
				if aliasIdx := strings.LastIndex(strings.ToUpper(col), " AS "); aliasIdx != -1 {
					col = strings.TrimSpace(col[aliasIdx+4:])
				}
				columns = append(columns, col)
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		col := strings.TrimSpace(current.String())
		if aliasIdx := strings.LastIndex(strings.ToUpper(col), " AS "); aliasIdx != -1 {
			col = strings.TrimSpace(col[aliasIdx+4:])
		}
		columns = append(columns, col)
	}

	return columns
}

type WhereCondition struct {
	Column   string
	Operator string
	Value    interface{}
	And      *WhereCondition
	Or       *WhereCondition
}

func parseWhereClause(wherePart string) *WhereCondition {
	upperPart := strings.ToUpper(wherePart)

	if andIdx := strings.Index(upperPart, " AND "); andIdx != -1 {
		left := parseWhereClause(strings.TrimSpace(wherePart[:andIdx]))
		right := parseWhereClause(strings.TrimSpace(wherePart[andIdx+5:]))
		if left != nil {
			left.And = right
			return left
		}
	}

	if orIdx := strings.Index(upperPart, " OR "); orIdx != -1 {
		left := parseWhereClause(strings.TrimSpace(wherePart[:orIdx]))
		right := parseWhereClause(strings.TrimSpace(wherePart[orIdx+4:]))
		if left != nil {
			left.Or = right
			return left
		}
	}

	operators := []string{"=", "!=", ">=", "<=", ">", "<"}
	for _, op := range operators {
		if idx := strings.Index(upperPart, " "+op+" "); idx != -1 {
			column := strings.TrimSpace(wherePart[:idx])
			valueStr := strings.TrimSpace(wherePart[idx+len(op)+2:])
			value := parseValue(valueStr)
			return &WhereCondition{
				Column:   column,
				Operator: op,
				Value:    value,
			}
		}
	}

	return nil
}

func parseValue(valueStr string) interface{} {
	valueStr = strings.TrimSpace(valueStr)
	valueStr = strings.Trim(valueStr, "'\"")

	if i, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return f
	}
	if strings.ToLower(valueStr) == "true" {
		return int64(1)
	}
	if strings.ToLower(valueStr) == "false" {
		return int64(0)
	}

	return valueStr
}

func evaluateWhere(row map[string]interface{}, cond *WhereCondition) bool {
	if cond == nil {
		return true
	}

	result := evaluateSingleCondition(row, cond)

	if cond.And != nil {
		return result && evaluateWhere(row, cond.And)
	}

	if cond.Or != nil {
		return result || evaluateWhere(row, cond.Or)
	}

	return result
}

func evaluateSingleCondition(row map[string]interface{}, cond *WhereCondition) bool {
	rowVal, exists := row[cond.Column]
	if !exists {
		return false
	}

	rowNum, rowIsNum := toFloat(rowVal)
	condNum, condIsNum := toFloat(cond.Value)

	if rowIsNum && condIsNum {
		switch cond.Operator {
		case "=":
			return rowNum == condNum
		case "!=":
			return rowNum != condNum
		case ">":
			return rowNum > condNum
		case "<":
			return rowNum < condNum
		case ">=":
			return rowNum >= condNum
		case "<=":
			return rowNum <= condNum
		}
	}

	rowStr := fmt.Sprintf("%v", rowVal)
	condStr := fmt.Sprintf("%v", cond.Value)

	switch cond.Operator {
	case "=":
		return rowStr == condStr
	case "!=":
		return rowStr != condStr
	}

	return false
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int64:
		return float64(val), true
	case float64:
		return val, true
	case int:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func convertMemoryValue(val interface{}) interface{} {
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
	case driver.Value:
		return v
	default:
		return v
	}
}
