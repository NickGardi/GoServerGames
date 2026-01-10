package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	SessionDuration = 1 * time.Hour
)

type Session struct {
	ID         string
	PlayerName string
	RoomCode   string
	CreatedAt  time.Time
}

type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewSessionStore() *SessionStore {
	ss := &SessionStore{
		sessions: make(map[string]*Session),
	}
	// Cleanup expired sessions periodically
	go ss.cleanupExpired()
	return ss
}

func (ss *SessionStore) CreateSession(playerName, roomCode string) (*Session, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:         sessionID,
		PlayerName: playerName,
		RoomCode:   roomCode,
		CreatedAt:  time.Now(),
	}

	ss.sessions[sessionID] = session
	return session, nil
}

func (ss *SessionStore) GetSession(sessionID string) (*Session, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	session, exists := ss.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Since(session.CreatedAt) > SessionDuration {
		return nil, false
	}

	return session, true
}

func (ss *SessionStore) DeleteSession(sessionID string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.sessions, sessionID)
}

func (ss *SessionStore) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ss.mu.Lock()
		now := time.Now()
		for id, session := range ss.sessions {
			if now.Sub(session.CreatedAt) > SessionDuration {
				delete(ss.sessions, id)
			}
		}
		ss.mu.Unlock()
	}
}

func generateSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GetPassword() (string, error) {
	password := os.Getenv("GAME_PASSWORD")
	if password == "" {
		return "", fmt.Errorf("GAME_PASSWORD environment variable not set")
	}
	return password, nil
}

func Authenticate(username, password string) (bool, error) {
	expectedPassword, err := GetPassword()
	if err != nil {
		return false, err
	}

	if password != expectedPassword {
		return false, nil
	}

	if username == "" {
		return false, fmt.Errorf("username cannot be empty")
	}

	return true, nil
}

