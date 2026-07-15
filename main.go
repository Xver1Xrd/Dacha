// Бэкенд сайта СНТ «Михайловское».
// Только REST API. Статика раздаётся отдельным frontend-контейнером.
// Хранилище — PostgreSQL (DATABASE_URL) или data.json (если DATABASE_URL не задан).
// Авторизация — сессии в HttpOnly-cookie, пароли хешируются PBKDF2-HMAC-SHA256.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const dataFile = "data.json"

// ---------- модели ----------

type Worker struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Telegram string `json:"telegram,omitempty"`
	AddedAt  int64  `json:"addedAt"`
	Clients  int    `json:"clients"`
}

type WorkRec struct {
	Status  string   `json:"status"`
	Workers []string `json:"workers,omitempty"`
	Work    string   `json:"work,omitempty"`
	Date    string   `json:"date,omitempty"`
}

type Perms struct {
	Map     bool `json:"map"`
	Workers bool `json:"workers"`
	Admins  bool `json:"admins"`
	Reviews bool `json:"reviews"`
}

type Admin struct {
	Login   string `json:"login"`
	Salt    string `json:"salt"`
	Hash    string `json:"hash"`
	Primary bool   `json:"primary"`
	Perms   Perms  `json:"perms"`
}

type Review struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Text  string `json:"text"`
	Stars int    `json:"stars"`
	Color string `json:"color,omitempty"`
}

type Alley struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// ---------- хранилище (файловое, для локальной разработки) ----------

type DB struct {
	Alleys       []Alley            `json:"alleys"`
	Works        map[string]WorkRec `json:"works"`
	Workers      []Worker           `json:"workers"`
	Admins       []Admin            `json:"admins"`
	Reviews      []Review           `json:"reviews"`
	NextWorkerID int                `json:"nextWorkerId"`
	NextReviewID int                `json:"nextReviewId"`
}

type Store struct {
	mu sync.Mutex
	db DB
}

func (s *Store) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(dataFile)
	if err == nil {
		if json.Unmarshal(b, &s.db) == nil && len(s.db.Admins) > 0 {
			return
		}
	}
	s.db = seed()
	s.saveLocked()
	log.Println("создан data.json со стартовыми данными")
}

func (s *Store) saveLocked() {
	b, _ := json.MarshalIndent(s.db, "", "  ")
	tmp := dataFile + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		os.Rename(tmp, dataFile)
	}
}

func (s *Store) save() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveLocked()
}

// ---------- единый интерфейс хранилища ----------

type dataStore interface {
	GetAlleys(ctx context.Context) ([]Alley, error)
	GetWorks(ctx context.Context) (map[string]WorkRec, error)
	GetWorkers(ctx context.Context) ([]Worker, error)
	GetReviews(ctx context.Context) ([]Review, error)
	GetAdmins(ctx context.Context) ([]Admin, error)
	GetAdminByLogin(ctx context.Context, login string) (*Admin, error)
	AddAdmin(ctx context.Context, login, pass string, perms Perms) error
	RemoveAdmin(ctx context.Context, login string) (bool, error)
	SetAdminPassword(ctx context.Context, login, pass string) error
	HasPerm(ctx context.Context, login, perm string) (bool, error)
	AddWorker(ctx context.Context, name, phone, telegram string, clients int) (int, error)
	EditWorker(ctx context.Context, id int, name, phone, telegram string, clients int) error
	RemoveWorker(ctx context.Context, id int) error
	AddReview(ctx context.Context, name, text string, stars int, color string) (int, error)
	EditReview(ctx context.Context, id int, name, text string, stars int, color string) error
	RemoveReview(ctx context.Context, id int) (bool, error)
	MoveReview(ctx context.Context, id int, direction string) error
	SetWork(ctx context.Context, key string, rec *WorkRec) error
}

// ---------- имплементация dataStore для файлового Store ----------

func (s *Store) GetAlleys(_ context.Context) ([]Alley, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Alley, len(s.db.Alleys))
	copy(out, s.db.Alleys)
	return out, nil
}

func (s *Store) GetWorks(_ context.Context) (map[string]WorkRec, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]WorkRec{}
	for k, v := range s.db.Works {
		out[k] = v
	}
	return out, nil
}

func (s *Store) GetWorkers(_ context.Context) ([]Worker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Worker, len(s.db.Workers))
	copy(out, s.db.Workers)
	return out, nil
}

func (s *Store) GetReviews(_ context.Context) ([]Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Review, len(s.db.Reviews))
	copy(out, s.db.Reviews)
	return out, nil
}

func (s *Store) GetAdmins(_ context.Context) ([]Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Admin, len(s.db.Admins))
	copy(out, s.db.Admins)
	return out, nil
}

func (s *Store) GetAdminByLogin(_ context.Context, login string) (*Admin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.Admins {
		if s.db.Admins[i].Login == login {
			a := s.db.Admins[i]
			return &a, nil
		}
	}
	return nil, nil
}

func (s *Store) AddAdmin(_ context.Context, login, pass string, perms Perms) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.db.Admins {
		if a.Login == login {
			return errDuplicateLogin
		}
	}
	salt, hash := hashPassword(pass)
	s.db.Admins = append(s.db.Admins, Admin{Login: login, Salt: salt, Hash: hash, Perms: perms})
	s.saveLocked()
	return nil
}

func (s *Store) SetAdminPassword(_ context.Context, login, pass string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.Admins {
		if s.db.Admins[i].Login == login {
			salt, hash := hashPassword(pass)
			s.db.Admins[i].Salt = salt
			s.db.Admins[i].Hash = hash
			s.saveLocked()
			return nil
		}
	}
	return pgx.ErrNoRows
}

func (s *Store) RemoveAdmin(_ context.Context, login string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.db.Admins[:0]
	removed := false
	for _, a := range s.db.Admins {
		if a.Login == login && !a.Primary {
			removed = true
			continue
		}
		out = append(out, a)
	}
	s.db.Admins = out
	if removed {
		s.saveLocked()
	}
	return removed, nil
}

func (s *Store) HasPerm(_ context.Context, login, perm string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.db.Admins {
		if a.Login == login {
			if a.Primary {
				return true, nil
			}
			switch perm {
			case "map":
				return a.Perms.Map, nil
			case "workers":
				return a.Perms.Workers, nil
			case "admins":
				return a.Perms.Admins, nil
			case "reviews":
				return a.Perms.Reviews, nil
			}
			return false, nil
		}
	}
	return false, nil
}

func (s *Store) AddWorker(_ context.Context, name, phone, telegram string, clients int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.db.NextWorkerID
	s.db.NextWorkerID++
	s.db.Workers = append(s.db.Workers, Worker{
		ID: id, Name: name, Phone: phone, Telegram: telegram, AddedAt: time.Now().UnixMilli(), Clients: clients,
	})
	s.saveLocked()
	return id, nil
}

func (s *Store) EditWorker(_ context.Context, id int, name, phone, telegram string, clients int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.Workers {
		if s.db.Workers[i].ID == id {
			s.db.Workers[i].Name = name
			s.db.Workers[i].Phone = phone
			s.db.Workers[i].Telegram = telegram
			s.db.Workers[i].Clients = clients
			s.saveLocked()
			return nil
		}
	}
	return pgx.ErrNoRows
}

func (s *Store) RemoveWorker(_ context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.db.Workers[:0]
	for _, w := range s.db.Workers {
		if w.ID == id {
			continue
		}
		out = append(out, w)
	}
	s.db.Workers = out
	s.saveLocked()
	return nil
}

func (s *Store) AddReview(_ context.Context, name, text string, stars int, color string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.db.NextReviewID
	s.db.NextReviewID++
	s.db.Reviews = append(s.db.Reviews, Review{ID: id, Name: name, Text: text, Stars: stars, Color: color})
	s.saveLocked()
	return id, nil
}

func (s *Store) EditReview(_ context.Context, id int, name, text string, stars int, color string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.db.Reviews {
		if s.db.Reviews[i].ID == id {
			s.db.Reviews[i].Name = name
			s.db.Reviews[i].Text = text
			s.db.Reviews[i].Stars = stars
			s.db.Reviews[i].Color = color
			s.saveLocked()
			return nil
		}
	}
	return pgx.ErrNoRows
}

func (s *Store) RemoveReview(_ context.Context, id int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.db.Reviews[:0]
	removed := false
	for _, r := range s.db.Reviews {
		if r.ID == id {
			removed = true
			continue
		}
		out = append(out, r)
	}
	s.db.Reviews = out
	if removed {
		s.saveLocked()
	}
	return removed, nil
}

func (s *Store) MoveReview(_ context.Context, id int, direction string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, r := range s.db.Reviews {
		if r.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return pgx.ErrNoRows
	}
	swapWith := idx - 1
	if direction == "down" {
		swapWith = idx + 1
	}
	if swapWith < 0 || swapWith >= len(s.db.Reviews) {
		return nil
	}
	s.db.Reviews[idx], s.db.Reviews[swapWith] = s.db.Reviews[swapWith], s.db.Reviews[idx]
	s.saveLocked()
	return nil
}

func (s *Store) SetWork(_ context.Context, key string, rec *WorkRec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db.Works == nil {
		s.db.Works = map[string]WorkRec{}
	}
	if rec == nil {
		delete(s.db.Works, key)
	} else {
		s.db.Works[key] = *rec
	}
	s.saveLocked()
	return nil
}

var errDuplicateLogin = errors.New("такой логин уже есть")

// ---------- пароли (PBKDF2-HMAC-SHA256) ----------

func pbkdf2(pass, salt []byte, iter, keyLen int) []byte {
	hashLen := sha256.Size
	blocks := (keyLen + hashLen - 1) / hashLen
	var dk []byte
	buf := make([]byte, 4)
	for b := 1; b <= blocks; b++ {
		prf := hmac.New(sha256.New, pass)
		prf.Write(salt)
		buf[0], buf[1], buf[2], buf[3] = byte(b>>24), byte(b>>16), byte(b>>8), byte(b)
		prf.Write(buf)
		u := prf.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)
		for n := 2; n <= iter; n++ {
			prf = hmac.New(sha256.New, pass)
			prf.Write(u)
			u = prf.Sum(nil)
			for i := range t {
				t[i] ^= u[i]
			}
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}

const pbkdfIter = 100000

func hashPassword(pass string) (saltHex, hashHex string) {
	salt := make([]byte, 16)
	rand.Read(salt)
	h := pbkdf2([]byte(pass), salt, pbkdfIter, 32)
	return hex.EncodeToString(salt), hex.EncodeToString(h)
}

func verifyPassword(pass, saltHex, hashHex string) bool {
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}
	got := pbkdf2([]byte(pass), salt, pbkdfIter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ---------- сервер ----------

const cookieName = "snt_session"

type Server struct {
	store dataStore
	sess  SessionStore
	limit *LoginLimiter
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (srv *Server) currentLogin(r *http.Request) (string, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", false
	}
	return srv.sess.Get(r.Context(), c.Value)
}

func (srv *Server) auth(h func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		login, ok := srv.currentLogin(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "не авторизован"})
			return
		}
		h(w, r, login)
	}
}

func (srv *Server) requirePerm(perm string, h func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return srv.auth(func(w http.ResponseWriter, r *http.Request, login string) {
		ok, err := srv.store.HasPerm(r.Context(), login, perm)
		if err != nil || !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "недостаточно прав"})
			return
		}
		h(w, r, login)
	})
}

// GET /api/data — публичные данные
func (srv *Server) handleData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alleys, err := srv.store.GetAlleys(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	works, err := srv.store.GetWorks(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	workers, err := srv.store.GetWorkers(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	reviews, err := srv.store.GetReviews(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"alleys": alleys, "works": works, "workers": workers, "reviews": reviews,
	})
}

// POST /api/login
func (srv *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login string `json:"login"`
		Pass  string `json:"pass"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil ||
		len(in.Login) > maxLoginLen || len(in.Pass) > maxPassLen {
		writeJSON(w, 400, map[string]string{"error": "плохой запрос"})
		return
	}

	key := loginLimiterKey(r, in.Login)
	if blocked, retryAfter := srv.limit.Blocked(key); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "слишком много попыток, попробуйте позже"})
		return
	}

	found, err := srv.store.GetAdminByLogin(r.Context(), in.Login)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	if found == nil || !verifyPassword(in.Pass, found.Salt, found.Hash) {
		srv.limit.RecordFailure(key)
		writeJSON(w, 401, map[string]string{"error": "неверный логин или пароль"})
		return
	}
	srv.limit.RecordSuccess(key)

	tok, err := srv.sess.Create(r.Context(), found.Login)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	secure := isSecureRequest(r)
	maxAge := int(sessionTTL.Seconds())
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: tok, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookieName, Value: newSessionToken(), Path: "/", MaxAge: maxAge,
		HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, 200, map[string]string{"login": found.Login})
}

// POST /api/logout
func (srv *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		srv.sess.Drop(r.Context(), c.Value)
	}
	secure := isSecureRequest(r)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure})
	http.SetCookie(w, &http.Cookie{Name: csrfCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: false, Secure: secure})
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// GET /api/session
func (srv *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	login, ok := srv.currentLogin(r)
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "нет сессии"})
		return
	}
	writeJSON(w, 200, map[string]string{"login": login})
}

// GET /api/admins
func (srv *Server) handleAdminsList(w http.ResponseWriter, r *http.Request, _ string) {
	admins, err := srv.store.GetAdmins(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	out := make([]map[string]any, 0, len(admins))
	for _, a := range admins {
		out = append(out, map[string]any{"login": a.Login, "primary": a.Primary, "perms": a.Perms})
	}
	writeJSON(w, 200, out)
}

// POST /api/admins
func (srv *Server) handleAdminsAdd(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct {
		Login string `json:"login"`
		Pass  string `json:"pass"`
		Perms Perms  `json:"perms"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Login == "" || in.Pass == "" ||
		len(in.Login) > maxLoginLen || len(in.Pass) > maxPassLen {
		writeJSON(w, 400, map[string]string{"error": "укажите логин и пароль"})
		return
	}
	if len(in.Pass) < minPassLen {
		writeJSON(w, 400, map[string]string{"error": "пароль должен быть не короче 6 символов"})
		return
	}
	err := srv.store.AddAdmin(r.Context(), in.Login, in.Pass, in.Perms)
	if err != nil {
		if errors.Is(err, errDuplicateLogin) {
			writeJSON(w, 409, map[string]string{"error": "такой логин уже есть"})
		} else {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeJSON(w, 409, map[string]string{"error": "такой логин уже есть"})
			} else {
				writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
			}
		}
		return
	}
	writeJSON(w, 201, map[string]string{"login": in.Login})
}

// DELETE /api/admins/{login}
func (srv *Server) handleAdminsDel(w http.ResponseWriter, r *http.Request, _ string) {
	login := r.PathValue("login")
	removed, err := srv.store.RemoveAdmin(r.Context(), login)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, map[string]bool{"removed": removed})
}

// POST /api/admins/{login}/password  {pass}
func (srv *Server) handleAdminsPassword(w http.ResponseWriter, r *http.Request, _ string) {
	login := r.PathValue("login")
	var in struct {
		Pass string `json:"pass"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Pass) < minPassLen || len(in.Pass) > maxPassLen {
		writeJSON(w, 400, map[string]string{"error": "пароль должен быть не короче 6 символов"})
		return
	}
	err := srv.store.SetAdminPassword(r.Context(), login, in.Pass)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "не найден"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// POST /api/workers
func (srv *Server) handleWorkersAdd(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct {
		Name, Phone, Telegram string
		Clients               int
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" || in.Clients < 0 ||
		!withinLimits(in.Name, maxNameLen) || !withinLimits(in.Phone, maxPhoneLen) || !withinLimits(in.Telegram, maxTelegramLen) {
		writeJSON(w, 400, map[string]string{"error": "укажите имя"})
		return
	}
	id, err := srv.store.AddWorker(r.Context(),
		strings.TrimSpace(in.Name), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Telegram), in.Clients)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 201, map[string]int{"id": id})
}

// DELETE /api/workers/{id}
func (srv *Server) handleWorkersDel(w http.ResponseWriter, r *http.Request, _ string) {
	id := parseInt(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "неверный id"})
		return
	}
	err := srv.store.RemoveWorker(r.Context(), id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// PUT /api/workers/{id}
func (srv *Server) handleWorkersEdit(w http.ResponseWriter, r *http.Request, _ string) {
	id := parseInt(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "неверный id"})
		return
	}
	var in struct {
		Name, Phone, Telegram string
		Clients               int
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" || in.Clients < 0 ||
		!withinLimits(in.Name, maxNameLen) || !withinLimits(in.Phone, maxPhoneLen) || !withinLimits(in.Telegram, maxTelegramLen) {
		writeJSON(w, 400, map[string]string{"error": "укажите имя"})
		return
	}
	err := srv.store.EditWorker(r.Context(), id,
		strings.TrimSpace(in.Name), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Telegram), in.Clients)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "не найден"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// GET /api/reviews
func (srv *Server) handleReviewsList(w http.ResponseWriter, r *http.Request) {
	reviews, err := srv.store.GetReviews(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, reviews)
}

// POST /api/reviews
func (srv *Server) handleReviewsAdd(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct {
		Name  string `json:"name"`
		Text  string `json:"text"`
		Stars int    `json:"stars"`
		Color string `json:"color,omitempty"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Text) == "" ||
		!withinLimits(in.Name, maxNameLen) || !withinLimits(in.Text, maxTextLen) || !withinLimits(in.Color, maxColorLen) {
		writeJSON(w, 400, map[string]string{"error": "укажите имя и текст"})
		return
	}
	if in.Stars < 1 || in.Stars > 5 {
		in.Stars = 5
	}
	id, err := srv.store.AddReview(r.Context(),
		strings.TrimSpace(in.Name), strings.TrimSpace(in.Text), in.Stars, strings.TrimSpace(in.Color))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 201, map[string]int{"id": id})
}

// DELETE /api/reviews/{id}
func (srv *Server) handleReviewsDel(w http.ResponseWriter, r *http.Request, _ string) {
	id := parseInt(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "неверный id"})
		return
	}
	removed, err := srv.store.RemoveReview(r.Context(), id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, map[string]bool{"removed": removed})
}

// PUT /api/reviews/{id}
func (srv *Server) handleReviewsEdit(w http.ResponseWriter, r *http.Request, _ string) {
	id := parseInt(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "неверный id"})
		return
	}
	var in struct {
		Name  string `json:"name"`
		Text  string `json:"text"`
		Stars int    `json:"stars"`
		Color string `json:"color,omitempty"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Text) == "" ||
		!withinLimits(in.Name, maxNameLen) || !withinLimits(in.Text, maxTextLen) || !withinLimits(in.Color, maxColorLen) {
		writeJSON(w, 400, map[string]string{"error": "укажите имя и текст"})
		return
	}
	if in.Stars < 1 || in.Stars > 5 {
		in.Stars = 5
	}
	err := srv.store.EditReview(r.Context(), id,
		strings.TrimSpace(in.Name), strings.TrimSpace(in.Text), in.Stars, strings.TrimSpace(in.Color))
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "не найден"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// POST /api/reviews/{id}/move  {direction: "up"|"down"}
func (srv *Server) handleReviewsMove(w http.ResponseWriter, r *http.Request, _ string) {
	id := parseInt(r.PathValue("id"))
	if id == 0 {
		writeJSON(w, 400, map[string]string{"error": "неверный id"})
		return
	}
	var in struct {
		Direction string `json:"direction"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || (in.Direction != "up" && in.Direction != "down") {
		writeJSON(w, 400, map[string]string{"error": "укажите direction: up или down"})
		return
	}
	err := srv.store.MoveReview(r.Context(), id, in.Direction)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "не найден"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

// POST /api/works  {key, rec}  (rec == null -> удалить)
func (srv *Server) handleWorks(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct {
		Key string   `json:"key"`
		Rec *WorkRec `json:"rec"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Key == "" {
		writeJSON(w, 400, map[string]string{"error": "плохой запрос"})
		return
	}
	err := srv.store.SetWork(r.Context(), in.Key, in.Rec)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "ошибка сервера"})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	var store dataStore
	var sess SessionStore
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		db, err := NewDatabase(context.Background(), dbURL)
		if err != nil {
			log.Fatalf("ошибка подключения к БД: %v", err)
		}
		defer db.Close()
		store = db
		sess = newDBSessions(db.pool)
		log.Println("режим: PostgreSQL (сессии переживают перезапуск)")
	} else {
		s := &Store{}
		s.load()
		store = s
		sess = newMemSessions()
		log.Println("режим: data.json (локальная разработка, сессии в памяти)")
	}

	srv := &Server{store: store, sess: sess, limit: newLoginLimiter()}
	handler := buildHandler(srv)

	log.Printf("сервер запущен: http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

// buildHandler собирает роутинг и цепочку middleware. Вынесено отдельно от
// main(), чтобы то же самое можно было поднять в тестах через httptest.
func buildHandler(srv *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/data", srv.handleData)
	mux.HandleFunc("POST /api/login", srv.handleLogin)
	mux.HandleFunc("POST /api/logout", srv.handleLogout)
	mux.HandleFunc("GET /api/session", srv.handleSession)
	mux.HandleFunc("GET /api/admins", srv.requirePerm("admins", srv.handleAdminsList))
	mux.HandleFunc("POST /api/admins", srv.requirePerm("admins", srv.handleAdminsAdd))
	mux.HandleFunc("DELETE /api/admins/{login}", srv.requirePerm("admins", srv.handleAdminsDel))
	mux.HandleFunc("POST /api/admins/{login}/password", srv.requirePerm("admins", srv.handleAdminsPassword))
	mux.HandleFunc("POST /api/workers", srv.requirePerm("workers", srv.handleWorkersAdd))
	mux.HandleFunc("PUT /api/workers/{id}", srv.requirePerm("workers", srv.handleWorkersEdit))
	mux.HandleFunc("DELETE /api/workers/{id}", srv.requirePerm("workers", srv.handleWorkersDel))
	mux.HandleFunc("POST /api/works", srv.requirePerm("map", srv.handleWorks))
	mux.HandleFunc("GET /api/reviews", srv.handleReviewsList)
	mux.HandleFunc("POST /api/reviews", srv.requirePerm("reviews", srv.handleReviewsAdd))
	mux.HandleFunc("DELETE /api/reviews/{id}", srv.requirePerm("reviews", srv.handleReviewsDel))
	mux.HandleFunc("PUT /api/reviews/{id}", srv.requirePerm("reviews", srv.handleReviewsEdit))
	mux.HandleFunc("POST /api/reviews/{id}/move", srv.requirePerm("reviews", srv.handleReviewsMove))

	return securityHeaders(limitBody(srv.csrfMiddleware(mux)))
}

// ---------- стартовые данные (для data.json) ----------

func seed() DB {
	salt, hash := hashPassword(seedAdminPassword)
	alleys := make([]Alley, len(seedAlleys))
	copy(alleys, seedAlleys)
	works := make(map[string]WorkRec, len(seedWorks))
	for k, v := range seedWorks {
		works[k] = v
	}
	workers := make([]Worker, len(seedWorkers))
	copy(workers, seedWorkers)
	reviews := make([]Review, len(seedReviews))
	copy(reviews, seedReviews)

	return DB{
		Alleys:  alleys,
		Works:   works,
		Workers: workers,
		Admins: []Admin{{
			Login: seedAdminLogin, Salt: salt, Hash: hash, Primary: true,
			Perms: Perms{Map: true, Workers: true, Admins: true, Reviews: true},
		}},
		Reviews:      reviews,
		NextWorkerID: len(workers) + 1,
		NextReviewID: len(reviews) + 1,
	}
}
