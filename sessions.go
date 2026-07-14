package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Время жизни сессии. Ровно столько же ставится в MaxAge cookie.
const sessionTTL = 24 * time.Hour

// cleanupInterval — как часто вычищаются просроченные сессии из памяти
// (для DBSessions устаревшие записи просто перестают проходить проверку
// expires_at, отдельная чистка не обязательна, но тоже выполняется).
const cleanupInterval = 30 * time.Minute

func newSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SessionStore — хранилище сессий администратора. Реализации:
// MemSessions (в памяти, для файлового режима) и DBSessions (в Postgres,
// переживает перезапуск/передеплой процесса).
type SessionStore interface {
	Create(ctx context.Context, login string) (string, error)
	Get(ctx context.Context, token string) (string, bool)
	Drop(ctx context.Context, token string)
}

// ---------- in-memory реализация ----------

type memSessionEntry struct {
	login   string
	expires time.Time
}

type MemSessions struct {
	mu sync.Mutex
	m  map[string]memSessionEntry
}

func newMemSessions() *MemSessions {
	s := &MemSessions{m: map[string]memSessionEntry{}}
	go s.cleanupLoop()
	return s
}

func (s *MemSessions) Create(_ context.Context, login string) (string, error) {
	tok := newSessionToken()
	s.mu.Lock()
	s.m[tok] = memSessionEntry{login: login, expires: time.Now().Add(sessionTTL)}
	s.mu.Unlock()
	return tok, nil
}

func (s *MemSessions) Get(_ context.Context, tok string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[tok]
	if !ok {
		return "", false
	}
	if time.Now().After(e.expires) {
		delete(s.m, tok)
		return "", false
	}
	return e.login, true
}

func (s *MemSessions) Drop(_ context.Context, tok string) {
	s.mu.Lock()
	delete(s.m, tok)
	s.mu.Unlock()
}

func (s *MemSessions) cleanupLoop() {
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		s.mu.Lock()
		for tok, e := range s.m {
			if now.After(e.expires) {
				delete(s.m, tok)
			}
		}
		s.mu.Unlock()
	}
}
