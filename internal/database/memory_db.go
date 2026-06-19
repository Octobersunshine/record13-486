package database

import (
	"context"
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
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
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
	db        *MemoryDB
}

type MemorySessionManager struct {
	sessions map[string]*MemorySession
	mu       sync.RWMutex
	db       *MemoryDB
	timeout  time.Duration
}

func NewMemorySessionManager(timeout time.Duration) *MemorySessionManager {
	return &MemorySessionManager{
		sessions: make(map[string]*MemorySession),
		db:       NewMemoryDB(),
		timeout:  timeout,
	}
}

func (sm *MemorySessionManager) CreateSession() (*MemorySession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID := generateSessionID()
	now := time.Now()
	session := &MemorySession{
		ID:        sessionID,
		CreatedAt: now,
		ExpiresAt: now.Add(sm.timeout),
		db:        sm.db,
	}

	sm.sessions[sessionID] = session
	return session, nil
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("mem-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (sm *MemorySessionManager) GetSession(sessionID string) (*MemorySession, error) {
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
		sm.mu.Unlock()
		sm.mu.RLock()
		return nil, fmt.Errorf("session expired")
	}

	session.ExpiresAt = time.Now().Add(sm.timeout)
	return session, nil
}

func (sm *MemorySessionManager) CloseSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	delete(sm.sessions, sessionID)
	return nil
}

func (sm *MemorySessionManager) CleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			delete(sm.sessions, id)
		}
	}
}

func (sm *MemorySessionManager) StartCleanupWorker(ctx context.Context) {
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
				sm.sessions = make(map[string]*MemorySession)
				return
			}
		}
	}()
}

func (s *MemorySession) ExecuteQuery(ctx context.Context, query string) ([]map[string]interface{}, error) {
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
		whereClause = parseWhereClause(wherePart)
	} else {
		tableName = strings.TrimSpace(afterFrom)
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
