package database

import "time"

type SessionInfo struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
}
