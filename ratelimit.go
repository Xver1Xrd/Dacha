package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Простой лимитер попыток входа: ключ — IP+логин, чтобы одна учётка не
// блокировала всех пользователей за NAT и наоборот.
const (
	loginMaxAttempts   = 5
	loginAttemptWindow = 15 * time.Minute
	loginBlockDuration = 15 * time.Minute
)

type loginAttempt struct {
	count        int
	windowStart  time.Time
	blockedUntil time.Time
}

type LoginLimiter struct {
	mu sync.Mutex
	m  map[string]*loginAttempt
}

func newLoginLimiter() *LoginLimiter {
	l := &LoginLimiter{m: map[string]*loginAttempt{}}
	go l.cleanupLoop()
	return l
}

// Blocked сообщает, заблокирован ли сейчас ключ, и через сколько секунд снимется блокировка.
func (l *LoginLimiter) Blocked(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.m[key]
	if !ok {
		return false, 0
	}
	if now := time.Now(); now.Before(a.blockedUntil) {
		return true, int(a.blockedUntil.Sub(now).Seconds()) + 1
	}
	return false, 0
}

func (l *LoginLimiter) RecordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	a, ok := l.m[key]
	if !ok || now.Sub(a.windowStart) > loginAttemptWindow {
		a = &loginAttempt{windowStart: now}
		l.m[key] = a
	}
	a.count++
	if a.count >= loginMaxAttempts {
		a.blockedUntil = now.Add(loginBlockDuration)
		a.count = 0
		a.windowStart = now
	}
}

func (l *LoginLimiter) RecordSuccess(key string) {
	l.mu.Lock()
	delete(l.m, key)
	l.mu.Unlock()
}

func (l *LoginLimiter) cleanupLoop() {
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		l.mu.Lock()
		for k, a := range l.m {
			if now.After(a.blockedUntil) && now.Sub(a.windowStart) > loginAttemptWindow {
				delete(l.m, k)
			}
		}
		l.mu.Unlock()
	}
}

// clientIP возвращает адрес клиента без порта. Заголовкам прокси (X-Forwarded-For)
// намеренно не доверяем по умолчанию — их может подделать сам клиент, если сайт
// не стоит за настроенным реверс-прокси.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func loginLimiterKey(r *http.Request, login string) string {
	return clientIP(r) + "|" + strings.ToLower(strings.TrimSpace(login))
}
