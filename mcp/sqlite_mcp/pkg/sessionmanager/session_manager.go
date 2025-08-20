package sessionmanager

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SessionInfo struct {
	Path       string
	ExpiresAt  time.Time
	LastAccess time.Time
}

type SessionManager struct {
	sessions    map[string]*SessionInfo
	mu          sync.Mutex
	rootDir     string
	expiration  time.Duration
	cleanupFreq time.Duration
}

func NewSessionManager(rootDir string, expiration time.Duration) *SessionManager {
	mgr := &SessionManager{
		sessions:    make(map[string]*SessionInfo),
		rootDir:     rootDir,
		expiration:  expiration,
		cleanupFreq: 1 * time.Minute,
	}

	if err := os.MkdirAll(rootDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create root dir: %v", err))
	}

	go mgr.cleanupLoop()
	return mgr
}

func (m *SessionManager) CreateDatabase() (string, error) {
	sessionID := generateSessionID()
	dbPath := filepath.Join(m.rootDir, sessionID+".db")

	// Touch the DB to ensure it exists
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()

	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS metadata (created_at TEXT);"); err != nil {
		return "", err
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = &SessionInfo{
		Path:       dbPath,
		ExpiresAt:  now.Add(m.expiration),
		LastAccess: now,
	}

	return sessionID, nil
}

func (m *SessionManager) GetDB(sessionID string) (*sql.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.sessions[sessionID]
	if !ok {
		return nil, errors.New("invalid session")
	}

	now := time.Now()
	if now.After(info.ExpiresAt) {
		delete(m.sessions, sessionID)
		return nil, errors.New("session expired")
	}

	// Extend expiration
	info.LastAccess = now
	info.ExpiresAt = now.Add(m.expiration)

	db, err := sql.Open("sqlite3", info.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	return db, nil
}

func (m *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(m.cleanupFreq)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpired()
	}
}

func (m *SessionManager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, info := range m.sessions {
		if now.After(info.ExpiresAt) {
			os.Remove(info.Path)
			delete(m.sessions, id)
		}
	}
}

func generateSessionID() string {
	return fmt.Sprintf("%d%d%d", time.Now().UnixNano(), rand.Uint64(), rand.Uint64())
}
