// Бэкенд сайта СНТ «Михайловское».
// Раздаёт статику (index.html, admin.html, css, js) и REST API.
// Хранилище — JSON-файл data.json. Авторизация — сессии в HttpOnly-cookie,
// пароли хешируются PBKDF2-HMAC-SHA256. Без внешних зависимостей.
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed index.html admin.html css js
var staticFS embed.FS

const dataFile = "data.json"

// ---------- модели ----------

type Worker struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	AddedAt int64  `json:"addedAt"`
}

type WorkRec struct {
	Status  string   `json:"status"`
	Workers []string `json:"workers,omitempty"`
	Work    string   `json:"work,omitempty"`
	Date    string   `json:"date,omitempty"`
}

type Admin struct {
	Login   string `json:"login"`
	Salt    string `json:"salt"`
	Hash    string `json:"hash"`
	Primary bool   `json:"primary"`
}

type Alley struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type DB struct {
	Alleys       []Alley            `json:"alleys"`
	Works        map[string]WorkRec `json:"works"`
	Workers      []Worker           `json:"workers"`
	Admins       []Admin            `json:"admins"`
	NextWorkerID int                `json:"nextWorkerId"`
}

// ---------- хранилище ----------

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
	log.Println("создан data.json со стартовыми данными (админ: admin / admin)")
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

// ---------- сессии ----------

type Sessions struct {
	mu sync.Mutex
	m  map[string]string // token -> login
}

func newSessions() *Sessions { return &Sessions{m: map[string]string{}} }

func (s *Sessions) create(login string) string {
	tok := make([]byte, 32)
	rand.Read(tok)
	t := hex.EncodeToString(tok)
	s.mu.Lock()
	s.m[t] = login
	s.mu.Unlock()
	return t
}
func (s *Sessions) get(tok string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.m[tok]
	return l, ok
}
func (s *Sessions) drop(tok string) {
	s.mu.Lock()
	delete(s.m, tok)
	s.mu.Unlock()
}

const cookieName = "snt_session"

// ---------- сервер ----------

type Server struct {
	store *Store
	sess  *Sessions
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
	return srv.sess.get(c.Value)
}

// auth — обёртка, требующая входа
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

// GET /api/data — публичные данные для карты
func (srv *Server) handleData(w http.ResponseWriter, r *http.Request) {
	srv.store.mu.Lock()
	defer srv.store.mu.Unlock()
	writeJSON(w, 200, map[string]any{
		"alleys":  srv.store.db.Alleys,
		"works":   srv.store.db.Works,
		"workers": srv.store.db.Workers,
	})
}

// POST /api/login
func (srv *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct{ Login, Pass string }
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "плохой запрос"})
		return
	}
	srv.store.mu.Lock()
	var found *Admin
	for i := range srv.store.db.Admins {
		if srv.store.db.Admins[i].Login == in.Login {
			found = &srv.store.db.Admins[i]
			break
		}
	}
	srv.store.mu.Unlock()
	if found == nil || !verifyPassword(in.Pass, found.Salt, found.Hash) {
		writeJSON(w, 401, map[string]string{"error": "неверный логин или пароль"})
		return
	}
	tok := srv.sess.create(found.Login)
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: tok, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, 200, map[string]string{"login": found.Login})
}

// POST /api/logout
func (srv *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		srv.sess.drop(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
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
	srv.store.mu.Lock()
	defer srv.store.mu.Unlock()
	out := make([]map[string]any, 0, len(srv.store.db.Admins))
	for _, a := range srv.store.db.Admins {
		out = append(out, map[string]any{"login": a.Login, "primary": a.Primary})
	}
	writeJSON(w, 200, out)
}

// POST /api/admins
func (srv *Server) handleAdminsAdd(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct{ Login, Pass string }
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Login == "" || in.Pass == "" {
		writeJSON(w, 400, map[string]string{"error": "укажите логин и пароль"})
		return
	}
	srv.store.mu.Lock()
	for _, a := range srv.store.db.Admins {
		if a.Login == in.Login {
			srv.store.mu.Unlock()
			writeJSON(w, 409, map[string]string{"error": "такой логин уже есть"})
			return
		}
	}
	salt, hash := hashPassword(in.Pass)
	srv.store.db.Admins = append(srv.store.db.Admins, Admin{Login: in.Login, Salt: salt, Hash: hash})
	srv.store.saveLocked()
	srv.store.mu.Unlock()
	writeJSON(w, 201, map[string]string{"login": in.Login})
}

// DELETE /api/admins/{login}
func (srv *Server) handleAdminsDel(w http.ResponseWriter, r *http.Request, _ string) {
	login := r.PathValue("login")
	srv.store.mu.Lock()
	out := srv.store.db.Admins[:0]
	removed := false
	for _, a := range srv.store.db.Admins {
		if a.Login == login && !a.Primary {
			removed = true
			continue
		}
		out = append(out, a)
	}
	srv.store.db.Admins = out
	if removed {
		srv.store.saveLocked()
	}
	srv.store.mu.Unlock()
	writeJSON(w, 200, map[string]bool{"removed": removed})
}

// POST /api/workers
func (srv *Server) handleWorkersAdd(w http.ResponseWriter, r *http.Request, _ string) {
	var in struct{ Name, Phone string }
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" {
		writeJSON(w, 400, map[string]string{"error": "укажите имя"})
		return
	}
	srv.store.mu.Lock()
	id := srv.store.db.NextWorkerID
	srv.store.db.NextWorkerID++
	srv.store.db.Workers = append(srv.store.db.Workers, Worker{
		ID: id, Name: strings.TrimSpace(in.Name), Phone: strings.TrimSpace(in.Phone), AddedAt: time.Now().UnixMilli(),
	})
	srv.store.saveLocked()
	srv.store.mu.Unlock()
	writeJSON(w, 201, map[string]int{"id": id})
}

// DELETE /api/workers/{id}
func (srv *Server) handleWorkersDel(w http.ResponseWriter, r *http.Request, _ string) {
	id := r.PathValue("id")
	srv.store.mu.Lock()
	out := srv.store.db.Workers[:0]
	for _, wk := range srv.store.db.Workers {
		if itoa(wk.ID) == id {
			continue
		}
		out = append(out, wk)
	}
	srv.store.db.Workers = out
	srv.store.saveLocked()
	srv.store.mu.Unlock()
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
	srv.store.mu.Lock()
	if srv.store.db.Works == nil {
		srv.store.db.Works = map[string]WorkRec{}
	}
	if in.Rec == nil {
		delete(srv.store.db.Works, in.Key)
	} else {
		srv.store.db.Works[in.Key] = *in.Rec
	}
	srv.store.saveLocked()
	srv.store.mu.Unlock()
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func itoa(i int) string {
	return strings.TrimSpace(jsonNum(i))
}
func jsonNum(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	store := &Store{}
	store.load()
	srv := &Server{store: store, sess: newSessions()}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/data", srv.handleData)
	mux.HandleFunc("POST /api/login", srv.handleLogin)
	mux.HandleFunc("POST /api/logout", srv.handleLogout)
	mux.HandleFunc("GET /api/session", srv.handleSession)
	mux.HandleFunc("GET /api/admins", srv.auth(srv.handleAdminsList))
	mux.HandleFunc("POST /api/admins", srv.auth(srv.handleAdminsAdd))
	mux.HandleFunc("DELETE /api/admins/{login}", srv.auth(srv.handleAdminsDel))
	mux.HandleFunc("POST /api/workers", srv.auth(srv.handleWorkersAdd))
	mux.HandleFunc("DELETE /api/workers/{id}", srv.auth(srv.handleWorkersDel))
	mux.HandleFunc("POST /api/works", srv.auth(srv.handleWorks))

	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	log.Printf("сервер запущен: http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// ---------- стартовые данные ----------

func seed() DB {
	salt, hash := hashPassword("admin")
	return DB{
		Alleys: []Alley{
			{"Дачная аллея", 13}, {"Западная улица", 18}, {"Набережная улица", 26},
			{"Восточная улица", 22}, {"Лучевая аллея", 22}, {"Зелёная аллея", 24},
			{"Ключевая аллея", 26}, {"Луговая аллея", 28}, {"Цветочная аллея", 30},
			{"Полевая аллея", 32}, {"Родниковая аллея", 30}, {"Лесная аллея", 28},
			{"Тенистая аллея", 24}, {"Озёрная аллея", 20}, {"Южная аллея", 23},
		},
		Works: map[string]WorkRec{
			"Лучевая аллея 7":   {Status: "done", Workers: []string{"Ярослав", "Роман"}, Work: "Покос травы и уборка мусора", Date: "июнь 2026"},
			"Южная аллея 5":     {Status: "done", Workers: []string{"Ярослав"}, Work: "Покос травы", Date: "июнь 2026"},
			"Цветочная аллея 9": {Status: "done", Workers: []string{"Денис", "Роман"}, Work: "Чистка канав, покос травы", Date: "май 2026"},
			"Луговая аллея 14":  {Status: "progress", Workers: []string{"Денис"}, Work: "Перекопка земли под грядки"},
			"Полевая аллея 21":  {Status: "progress", Workers: []string{"Ярослав", "Роман", "Денис"}, Work: "Уборка участка и перекопка щебня"},
			"Зелёная аллея 3":   {Status: "planned", Workers: []string{"Ярослав"}, Work: "Спил мелких деревьев и поросли"},
			"Ключевая аллея 2":  {Status: "planned", Workers: []string{"Роман"}, Work: "Покос травы"},
		},
		Workers: []Worker{
			{ID: 1, Name: "Ярослав", Phone: "+7 967 592 58 71", AddedAt: 0},
			{ID: 2, Name: "Роман", Phone: "+7 981 204 11 78", AddedAt: 0},
			{ID: 3, Name: "Денис", Phone: "+7 950 029 03 98", AddedAt: 0},
		},
		Admins:       []Admin{{Login: "admin", Salt: salt, Hash: hash, Primary: true}},
		NextWorkerID: 4,
	}
}
