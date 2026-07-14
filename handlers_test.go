package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	store := newTestStore(t)
	return &Server{store: store, sess: newMemSessions(), limit: newLoginLimiter()}
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = strings.NewReader(string(b))
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func extractCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestLoginWrongPasswordThenRateLimited(t *testing.T) {
	srv := newTestServer(t)
	h := buildHandler(srv)

	var last *httptest.ResponseRecorder
	for i := 0; i < loginMaxAttempts; i++ {
		last = doJSON(t, h, "POST", "/api/login", map[string]string{"login": seedAdminLogin, "pass": "wrong"}, nil, "")
		if last.Code != http.StatusUnauthorized {
			t.Fatalf("попытка %d: код = %d, want 401", i+1, last.Code)
		}
	}
	rec := doJSON(t, h, "POST", "/api/login", map[string]string{"login": seedAdminLogin, "pass": "wrong"}, nil, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("после %d неудачных попыток ожидали 429, получили %d", loginMaxAttempts, rec.Code)
	}
}

func TestLoginSuccessAndCSRFFlow(t *testing.T) {
	srv := newTestServer(t)
	h := buildHandler(srv)

	rec := doJSON(t, h, "POST", "/api/login", map[string]string{"login": seedAdminLogin, "pass": seedAdminPassword}, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("логин с верным паролем должен вернуть 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	sessionCookie := extractCookie(rec, cookieName)
	csrfCookie := extractCookie(rec, csrfCookieName)
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("после логина должны быть выставлены обе куки: сессия и csrf")
	}
	cookies := []*http.Cookie{sessionCookie, csrfCookie}

	// Без CSRF-токена в заголовке мутирующий запрос должен быть отклонён.
	rec = doJSON(t, h, "POST", "/api/workers", map[string]string{"name": "Иван"}, cookies, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /api/workers без X-CSRF-Token должен вернуть 403, получили %d", rec.Code)
	}

	// С правильным CSRF-токеном запрос должен пройти.
	rec = doJSON(t, h, "POST", "/api/workers", map[string]string{"name": "Иван"}, cookies, csrfCookie.Value)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/workers с верным X-CSRF-Token должен вернуть 201, получили %d: %s", rec.Code, rec.Body.String())
	}

	// GET-запросы CSRF не требуют.
	rec = doJSON(t, h, "GET", "/api/session", nil, cookies, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/session должен вернуть 200, получили %d", rec.Code)
	}
}

func TestWorkersRejectOverlongName(t *testing.T) {
	srv := newTestServer(t)
	h := buildHandler(srv)

	rec := doJSON(t, h, "POST", "/api/login", map[string]string{"login": seedAdminLogin, "pass": seedAdminPassword}, nil, "")
	cookies := []*http.Cookie{extractCookie(rec, cookieName), extractCookie(rec, csrfCookieName)}
	csrf := cookies[1].Value

	longName := strings.Repeat("a", maxNameLen+1)
	rec = doJSON(t, h, "POST", "/api/workers", map[string]string{"name": longName}, cookies, csrf)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("слишком длинное имя должно быть отклонено с 400, получили %d", rec.Code)
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	srv := newTestServer(t)
	h := buildHandler(srv)

	rec := doJSON(t, h, "GET", "/api/data", nil, nil, "")
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("ожидали заголовок Content-Security-Policy")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("ожидали X-Frame-Options: DENY")
	}
}
