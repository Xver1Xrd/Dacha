package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	pool *pgxpool.Pool
}

func NewDatabase(ctx context.Context, url string) (*Database, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	db := &Database{pool: pool}
	if err := db.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return db, nil
}

func (db *Database) Close() { db.pool.Close() }

func (db *Database) migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS alleys (
		label TEXT PRIMARY KEY,
		count INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS works (
		key TEXT PRIMARY KEY,
		status TEXT NOT NULL DEFAULT 'none',
		workers TEXT[] NOT NULL DEFAULT '{}',
		work_desc TEXT NOT NULL DEFAULT '',
		date TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS workers (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		phone TEXT NOT NULL DEFAULT '',
		telegram TEXT NOT NULL DEFAULT '',
		added_at BIGINT NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS reviews (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		text TEXT NOT NULL,
		stars INTEGER NOT NULL DEFAULT 5,
		color TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE IF NOT EXISTS admins (
		login TEXT PRIMARY KEY,
		salt TEXT NOT NULL,
		hash TEXT NOT NULL,
		is_primary BOOLEAN NOT NULL DEFAULT false,
		perms TEXT NOT NULL DEFAULT '{}'
	);
	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		login TEXT NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL
	);`
	if _, err := db.pool.Exec(ctx, schema); err != nil {
		return err
	}
	var count int
	db.pool.QueryRow(ctx, "SELECT COUNT(*) FROM admins").Scan(&count)
	if count == 0 {
		db.seed(ctx)
		log.Println("БД инициализирована стартовыми данными")
	} else {
		log.Printf("БД подключена, %d администратор(ов)", count)
	}
	return nil
}

func (db *Database) seed(ctx context.Context) {
	salt, hash := hashPassword(seedAdminPassword)
	perms, _ := json.Marshal(Perms{Map: true, Workers: true, Admins: true, Reviews: true})
	now := time.Now().UnixMilli()

	for _, a := range seedAlleys {
		db.pool.Exec(ctx, "INSERT INTO alleys(label,count) VALUES($1,$2) ON CONFLICT DO NOTHING", a.Label, a.Count)
	}

	for k, w := range seedWorks {
		db.pool.Exec(ctx, "INSERT INTO works(key,status,workers,work_desc,date) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING",
			k, w.Status, w.Workers, w.Work, w.Date)
	}

	for _, w := range seedWorkers {
		db.pool.Exec(ctx, "INSERT INTO workers(name,phone,telegram,added_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING",
			w.Name, w.Phone, w.Telegram, now)
	}

	db.pool.Exec(ctx, "INSERT INTO admins(login,salt,hash,is_primary,perms) VALUES($1,$2,$3,true,$4) ON CONFLICT DO NOTHING",
		seedAdminLogin, salt, hash, string(perms))

	for _, r := range seedReviews {
		db.pool.Exec(ctx, "INSERT INTO reviews(name,text,stars,color) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING",
			r.Name, r.Text, r.Stars, r.Color)
	}
}

// --- аллеи ---

func (db *Database) GetAlleys(ctx context.Context) ([]Alley, error) {
	rows, err := db.pool.Query(ctx, "SELECT label,count FROM alleys ORDER BY label")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alley
	for rows.Next() {
		var a Alley
		if err := rows.Scan(&a.Label, &a.Count); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// --- работы ---

func (db *Database) GetWorks(ctx context.Context) (map[string]WorkRec, error) {
	rows, err := db.pool.Query(ctx, "SELECT key,status,workers,work_desc,date FROM works")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]WorkRec{}
	for rows.Next() {
		var k string
		var w WorkRec
		if err := rows.Scan(&k, &w.Status, &w.Workers, &w.Work, &w.Date); err != nil {
			return nil, err
		}
		out[k] = w
	}
	return out, nil
}

func (db *Database) SetWork(ctx context.Context, key string, rec *WorkRec) error {
	if rec == nil {
		_, err := db.pool.Exec(ctx, "DELETE FROM works WHERE key=$1", key)
		return err
	}
	_, err := db.pool.Exec(ctx,
		"INSERT INTO works(key,status,workers,work_desc,date) VALUES($1,$2,$3,$4,$5) ON CONFLICT(key) DO UPDATE SET status=$2,workers=$3,work_desc=$4,date=$5",
		key, rec.Status, rec.Workers, rec.Work, rec.Date)
	return err
}

// --- работники ---

func (db *Database) GetWorkers(ctx context.Context) ([]Worker, error) {
	rows, err := db.pool.Query(ctx, "SELECT id,name,phone,telegram,added_at FROM workers ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Worker
	for rows.Next() {
		var w Worker
		if err := rows.Scan(&w.ID, &w.Name, &w.Phone, &w.Telegram, &w.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func (db *Database) AddWorker(ctx context.Context, name, phone, telegram string) (int, error) {
	var id int
	err := db.pool.QueryRow(ctx,
		"INSERT INTO workers(name,phone,telegram,added_at) VALUES($1,$2,$3,$4) RETURNING id",
		name, phone, telegram, time.Now().UnixMilli()).Scan(&id)
	return id, err
}

func (db *Database) EditWorker(ctx context.Context, id int, name, phone, telegram string) error {
	res, err := db.pool.Exec(ctx,
		"UPDATE workers SET name=$1,phone=$2,telegram=$3 WHERE id=$4", name, phone, telegram, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (db *Database) RemoveWorker(ctx context.Context, id int) error {
	_, err := db.pool.Exec(ctx, "DELETE FROM workers WHERE id=$1", id)
	return err
}

// --- отзывы ---

func (db *Database) GetReviews(ctx context.Context) ([]Review, error) {
	rows, err := db.pool.Query(ctx, "SELECT id,name,text,stars,color FROM reviews ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.Name, &r.Text, &r.Stars, &r.Color); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (db *Database) AddReview(ctx context.Context, name, text string, stars int, color string) (int, error) {
	var id int
	err := db.pool.QueryRow(ctx,
		"INSERT INTO reviews(name,text,stars,color) VALUES($1,$2,$3,$4) RETURNING id",
		name, text, stars, color).Scan(&id)
	return id, err
}

func (db *Database) EditReview(ctx context.Context, id int, name, text string, stars int, color string) error {
	res, err := db.pool.Exec(ctx,
		"UPDATE reviews SET name=$1,text=$2,stars=$3,color=$4 WHERE id=$5",
		name, text, stars, color, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (db *Database) RemoveReview(ctx context.Context, id int) (bool, error) {
	res, err := db.pool.Exec(ctx, "DELETE FROM reviews WHERE id=$1", id)
	return res.RowsAffected() > 0, err
}

// --- администраторы ---

func (db *Database) GetAdmins(ctx context.Context) ([]Admin, error) {
	rows, err := db.pool.Query(ctx, "SELECT login,salt,hash,is_primary,perms FROM admins ORDER BY login")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Admin
	for rows.Next() {
		var a Admin
		var primary bool
		var permsStr string
		if err := rows.Scan(&a.Login, &a.Salt, &a.Hash, &primary, &permsStr); err != nil {
			return nil, err
		}
		a.Primary = primary
		json.Unmarshal([]byte(permsStr), &a.Perms)
		out = append(out, a)
	}
	return out, nil
}

func (db *Database) GetAdminByLogin(ctx context.Context, login string) (*Admin, error) {
	var a Admin
	var primary bool
	var permsStr string
	err := db.pool.QueryRow(ctx, "SELECT login,salt,hash,is_primary,perms FROM admins WHERE login=$1", login).
		Scan(&a.Login, &a.Salt, &a.Hash, &primary, &permsStr)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Primary = primary
	json.Unmarshal([]byte(permsStr), &a.Perms)
	return &a, nil
}

func (db *Database) AddAdmin(ctx context.Context, login, pass string, perms Perms) error {
	salt, hash := hashPassword(pass)
	permsJSON, _ := json.Marshal(perms)
	_, err := db.pool.Exec(ctx,
		"INSERT INTO admins(login,salt,hash,is_primary,perms) VALUES($1,$2,$3,false,$4)",
		login, salt, hash, string(permsJSON))
	return err
}

func (db *Database) RemoveAdmin(ctx context.Context, login string) (bool, error) {
	res, err := db.pool.Exec(ctx, "DELETE FROM admins WHERE login=$1 AND NOT is_primary", login)
	return res.RowsAffected() > 0, err
}

func (db *Database) HasPerm(ctx context.Context, login, perm string) (bool, error) {
	var primary bool
	var permsStr string
	err := db.pool.QueryRow(ctx, "SELECT is_primary,perms FROM admins WHERE login=$1", login).
		Scan(&primary, &permsStr)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if primary {
		return true, nil
	}
	var p Perms
	json.Unmarshal([]byte(permsStr), &p)
	switch perm {
	case "map":
		return p.Map, nil
	case "workers":
		return p.Workers, nil
	case "admins":
		return p.Admins, nil
	case "reviews":
		return p.Reviews, nil
	}
	return false, nil
}

// --- сессии (Postgres) ---
//
// В отличие от MemSessions переживают перезапуск/передеплой процесса —
// администратора не разлогинивает при каждом обновлении бинарника.

type DBSessions struct {
	pool *pgxpool.Pool
}

func newDBSessions(pool *pgxpool.Pool) *DBSessions {
	s := &DBSessions{pool: pool}
	go s.cleanupLoop()
	return s
}

func (s *DBSessions) Create(ctx context.Context, login string) (string, error) {
	tok := newSessionToken()
	_, err := s.pool.Exec(ctx,
		"INSERT INTO sessions(token,login,expires_at) VALUES($1,$2,$3)",
		tok, login, time.Now().Add(sessionTTL))
	if err != nil {
		return "", err
	}
	return tok, nil
}

func (s *DBSessions) Get(ctx context.Context, tok string) (string, bool) {
	var login string
	err := s.pool.QueryRow(ctx,
		"SELECT login FROM sessions WHERE token=$1 AND expires_at > now()", tok).Scan(&login)
	if err != nil {
		return "", false
	}
	return login, true
}

func (s *DBSessions) Drop(ctx context.Context, tok string) {
	s.pool.Exec(ctx, "DELETE FROM sessions WHERE token=$1", tok)
}

func (s *DBSessions) cleanupLoop() {
	ctx := context.Background()
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for range t.C {
		s.pool.Exec(ctx, "DELETE FROM sessions WHERE expires_at <= now()")
	}
}
