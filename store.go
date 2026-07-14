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
	salt, hash := hashPassword("2606")
	perms, _ := json.Marshal(Perms{Map: true, Workers: true, Admins: true, Reviews: true})
	now := time.Now().UnixMilli()

	alleys := []struct{ label string; count int }{
		{"Дачная аллея", 13}, {"Западная улица", 18}, {"Набережная улица", 26},
		{"Восточная улица", 22}, {"Лучевая аллея", 22}, {"Зелёная аллея", 24},
		{"Ключевая аллея", 26}, {"Луговая аллея", 28}, {"Цветочная аллея", 30},
		{"Полевая аллея", 32}, {"Родниковая аллея", 30}, {"Лесная аллея", 28},
		{"Тенистая аллея", 24}, {"Озёрная аллея", 20}, {"Южная аллея", 23},
	}
	for _, a := range alleys {
		db.pool.Exec(ctx, "INSERT INTO alleys(label,count) VALUES($1,$2) ON CONFLICT DO NOTHING", a.label, a.count)
	}

	works := map[string]WorkRec{
		"Лучевая аллея 7":   {Status: "done", Workers: []string{"Ярослав", "Роман"}, Work: "Покос травы и уборка мусора", Date: "июнь 2026"},
		"Южная аллея 5":     {Status: "done", Workers: []string{"Ярослав"}, Work: "Покос травы", Date: "июнь 2026"},
		"Цветочная аллея 9": {Status: "done", Workers: []string{"Денис", "Роман"}, Work: "Чистка канав, покос травы", Date: "май 2026"},
		"Луговая аллея 14":  {Status: "progress", Workers: []string{"Денис"}, Work: "Перекопка земли под грядки"},
		"Полевая аллея 21":  {Status: "progress", Workers: []string{"Ярослав", "Роман", "Денис"}, Work: "Уборка участка и перекопка щебня"},
		"Зелёная аллея 3":   {Status: "planned", Workers: []string{"Ярослав"}, Work: "Спил мелких деревьев и поросли"},
		"Ключевая аллея 2":  {Status: "planned", Workers: []string{"Роман"}, Work: "Покос травы"},
	}
	for k, w := range works {
		db.pool.Exec(ctx, "INSERT INTO works(key,status,workers,work_desc,date) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING",
			k, w.Status, w.Workers, w.Work, w.Date)
	}

	workers := []struct{ name, phone, telegram string }{
		{"Ярослав", "+7 967 592 58 71", "https://t.me/+79675925871"},
		{"Роман", "+7 981 204 11 78", "https://t.me/+79812041178"},
		{"Денис", "+7 950 029 03 98", "https://t.me/+79500290398"},
	}
	for _, w := range workers {
		db.pool.Exec(ctx, "INSERT INTO workers(name,phone,telegram,added_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING",
			w.name, w.phone, w.telegram, now)
	}

	db.pool.Exec(ctx, "INSERT INTO admins(login,salt,hash,is_primary,perms) VALUES($1,$2,$3,true,$4) ON CONFLICT DO NOTHING",
		"xverlxrd", salt, hash, string(perms))

	reviews := []Review{
		{Name: "Людмила", Text: "Скосили всё за полдня, участок было не узнать. Договорились по телефону, приехали минута в минуту.", Stars: 5},
		{Name: "Виктор", Text: "Перекопали землю под грядки и вывезли весь хлам после стройки. Сделали аккуратно, цену обговорили заранее.", Stars: 5, Color: "var(--sun)"},
		{Name: "Нина Петровна", Text: "Канавы стояли годами, после дождя топило. Прочистили — вода уходит. Спасибо, выручили.", Stars: 5, Color: "#3f7d2e"},
		{Name: "Олег", Text: "Помогли разобрать старый сарай и всё вывезти, спилили пару старых яблонь. Работящие, не ленятся. Будем звать ещё.", Stars: 5},
	}
	for _, r := range reviews {
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
