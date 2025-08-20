package sessionmanager_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/poy/adk-rnd/mcp/sqlite_mcp/pkg/sessionmanager"
)

func TestCreateDatabase(t *testing.T) {
	rootDir := t.TempDir()

	manager := sessionmanager.NewSessionManager(rootDir, 10*time.Minute)

	sessionID, err := manager.CreateDatabase()
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if _, err := manager.GetDB(sessionID); err != nil {
		t.Fatalf("Failed to get database: %v", err)
	}
}

func TestSessionExpiration(t *testing.T) {
	rootDir := t.TempDir()

	manager := sessionmanager.NewSessionManager(rootDir, 10*time.Millisecond)
	sessionID, err := manager.CreateDatabase()
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	_, err = manager.GetDB(sessionID)
	if err == nil {
		t.Error("Expected error due to session expiration, got none")
	}
}

func TestSessionRenewal(t *testing.T) {
	rootDir := t.TempDir()

	manager := sessionmanager.NewSessionManager(rootDir, 50*time.Millisecond)
	sessionID, err := manager.CreateDatabase()
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Renew before expiry
	time.Sleep(25 * time.Millisecond)
	_, err = manager.GetDB(sessionID)
	if err != nil {
		t.Fatalf("Expected session to still be valid, got error: %v", err)
	}

	time.Sleep(30 * time.Millisecond) // should still be valid due to renewal
	_, err = manager.GetDB(sessionID)
	if err != nil {
		t.Fatalf("Expected session to still be valid after renewal, got error: %v", err)
	}
}

func TestInvalidSession(t *testing.T) {
	rootDir := t.TempDir()

	manager := sessionmanager.NewSessionManager(rootDir, 1*time.Minute)
	_, err := manager.GetDB("not-a-real-session")
	if err == nil {
		t.Error("Expected error for invalid session, got none")
	}
}

func TestDatabaseFilePath(t *testing.T) {
	rootDir := t.TempDir()

	manager := sessionmanager.NewSessionManager(rootDir, 1*time.Minute)
	sessionID, err := manager.CreateDatabase()
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	expectedPath := filepath.Join(rootDir, sessionID+".db")
	if filepath.Dir(expectedPath) != rootDir {
		t.Errorf("Database not created in root dir. Got %s, expected prefix %s", expectedPath, rootDir)
	}
}
