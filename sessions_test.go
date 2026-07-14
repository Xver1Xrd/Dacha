package main

import (
	"context"
	"testing"
	"time"
)

func TestMemSessionsCreateGetDrop(t *testing.T) {
	s := &MemSessions{m: map[string]memSessionEntry{}}
	ctx := context.Background()

	tok, err := s.Create(ctx, "admin")
	if err != nil {
		t.Fatalf("Create вернул ошибку: %v", err)
	}
	login, ok := s.Get(ctx, tok)
	if !ok || login != "admin" {
		t.Fatalf("Get(%q) = (%q, %v), want (admin, true)", tok, login, ok)
	}

	s.Drop(ctx, tok)
	if _, ok := s.Get(ctx, tok); ok {
		t.Fatal("сессия должна исчезнуть после Drop")
	}
}

func TestMemSessionsExpiry(t *testing.T) {
	s := &MemSessions{m: map[string]memSessionEntry{
		"expired": {login: "admin", expires: time.Now().Add(-time.Second)},
	}}
	if _, ok := s.Get(context.Background(), "expired"); ok {
		t.Fatal("просроченная сессия не должна отдаваться")
	}
	if _, ok := s.m["expired"]; ok {
		t.Fatal("просроченная сессия должна вычищаться при Get")
	}
}

func TestMemSessionsUnknownToken(t *testing.T) {
	s := &MemSessions{m: map[string]memSessionEntry{}}
	if _, ok := s.Get(context.Background(), "no-such-token"); ok {
		t.Fatal("несуществующий токен не должен находиться")
	}
}
