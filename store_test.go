package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	t.Chdir(t.TempDir())
	s := &Store{}
	s.load()
	return s
}

func TestStoreLoadSeeds(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	admins, err := s.GetAdmins(ctx)
	if err != nil || len(admins) != 1 || admins[0].Login != seedAdminLogin {
		t.Fatalf("ожидали одного стартового админа %q, получили %+v (err=%v)", seedAdminLogin, admins, err)
	}
	workers, _ := s.GetWorkers(ctx)
	if len(workers) != len(seedWorkers) {
		t.Fatalf("ожидали %d работников, получили %d", len(seedWorkers), len(workers))
	}
	reviews, _ := s.GetReviews(ctx)
	if len(reviews) != len(seedReviews) {
		t.Fatalf("ожидали %d отзывов, получили %d", len(seedReviews), len(reviews))
	}
}

func TestStoreWorkersCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.AddWorker(ctx, "Тест", "+7 000", "tg", 3)
	if err != nil {
		t.Fatalf("AddWorker: %v", err)
	}
	if err := s.EditWorker(ctx, id, "Тест2", "+7 111", "tg2", 7); err != nil {
		t.Fatalf("EditWorker: %v", err)
	}
	workers, _ := s.GetWorkers(ctx)
	found := false
	for _, w := range workers {
		if w.ID == id {
			found = true
			if w.Name != "Тест2" || w.Phone != "+7 111" || w.Clients != 7 {
				t.Fatalf("работник не обновился: %+v", w)
			}
		}
	}
	if !found {
		t.Fatal("добавленный работник не найден")
	}

	if err := s.EditWorker(ctx, 999999, "x", "y", "z", 0); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("EditWorker на несуществующем id должен вернуть pgx.ErrNoRows, получили %v", err)
	}

	if err := s.RemoveWorker(ctx, id); err != nil {
		t.Fatalf("RemoveWorker: %v", err)
	}
	workers, _ = s.GetWorkers(ctx)
	for _, w := range workers {
		if w.ID == id {
			t.Fatal("работник должен быть удалён")
		}
	}
}

func TestStoreReviewsCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.AddReview(ctx, "Имя", "Текст", 4, "")
	if err != nil {
		t.Fatalf("AddReview: %v", err)
	}
	if err := s.EditReview(ctx, id, "Имя2", "Текст2", 5, "#fff"); err != nil {
		t.Fatalf("EditReview: %v", err)
	}
	reviews, _ := s.GetReviews(ctx)
	found := false
	for _, r := range reviews {
		if r.ID == id {
			found = true
			if r.Name != "Имя2" || r.Stars != 5 {
				t.Fatalf("отзыв не обновился: %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("добавленный отзыв не найден")
	}

	removed, err := s.RemoveReview(ctx, id)
	if err != nil || !removed {
		t.Fatalf("RemoveReview: removed=%v err=%v", removed, err)
	}
	removedAgain, _ := s.RemoveReview(ctx, id)
	if removedAgain {
		t.Fatal("повторное удаление того же отзыва не должно сообщать об успехе")
	}
}

func TestStoreMoveReview(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id1, _ := s.AddReview(ctx, "Первый", "текст1", 5, "")
	id2, _ := s.AddReview(ctx, "Второй", "текст2", 5, "")
	id3, _ := s.AddReview(ctx, "Третий", "текст3", 5, "")

	order := func() []int {
		reviews, _ := s.GetReviews(ctx)
		out := make([]int, 0, len(reviews))
		for _, r := range reviews {
			if r.ID == id1 || r.ID == id2 || r.ID == id3 {
				out = append(out, r.ID)
			}
		}
		return out
	}

	if got := order(); got[0] != id1 || got[1] != id2 || got[2] != id3 {
		t.Fatalf("неожиданный стартовый порядок: %v", got)
	}

	if err := s.MoveReview(ctx, id2, "up"); err != nil {
		t.Fatalf("MoveReview up: %v", err)
	}
	if got := order(); got[0] != id2 || got[1] != id1 || got[2] != id3 {
		t.Fatalf("после move up ожидали [id2,id1,id3], получили %v", got)
	}

	if err := s.MoveReview(ctx, id2, "down"); err != nil {
		t.Fatalf("MoveReview down: %v", err)
	}
	if got := order(); got[0] != id1 || got[1] != id2 || got[2] != id3 {
		t.Fatalf("после move down ожидали вернуться к [id1,id2,id3], получили %v", got)
	}

	// первый элемент нельзя подвинуть выше - должен быть no-op, без ошибки
	if err := s.MoveReview(ctx, id1, "up"); err != nil {
		t.Fatalf("MoveReview up на первом элементе: %v", err)
	}
	if got := order(); got[0] != id1 {
		t.Fatalf("первый элемент не должен был сдвинуться: %v", got)
	}

	// последний элемент нельзя подвинуть ниже - тоже no-op
	if err := s.MoveReview(ctx, id3, "down"); err != nil {
		t.Fatalf("MoveReview down на последнем элементе: %v", err)
	}
	if got := order(); got[2] != id3 {
		t.Fatalf("последний элемент не должен был сдвинуться: %v", got)
	}

	if err := s.MoveReview(ctx, 999999, "up"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("MoveReview на несуществующем id должен вернуть pgx.ErrNoRows, получили %v", err)
	}
}

func TestStoreAdminsDuplicateLogin(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.AddAdmin(ctx, "novy", "pass123", Perms{}); err != nil {
		t.Fatalf("AddAdmin: %v", err)
	}
	if err := s.AddAdmin(ctx, "novy", "pass456", Perms{}); !errors.Is(err, errDuplicateLogin) {
		t.Fatalf("ожидали errDuplicateLogin, получили %v", err)
	}
}

func TestStoreRemoveAdminProtectsPrimary(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	removed, err := s.RemoveAdmin(ctx, seedAdminLogin)
	if err != nil {
		t.Fatalf("RemoveAdmin: %v", err)
	}
	if removed {
		t.Fatal("главного администратора нельзя удалить")
	}
}

func TestStoreHasPerm(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.AddAdmin(ctx, "limited", "pass123", Perms{Reviews: true})

	ok, err := s.HasPerm(ctx, "limited", "reviews")
	if err != nil || !ok {
		t.Fatalf("limited должен иметь право reviews: ok=%v err=%v", ok, err)
	}
	ok, err = s.HasPerm(ctx, "limited", "admins")
	if err != nil || ok {
		t.Fatalf("limited не должен иметь право admins: ok=%v err=%v", ok, err)
	}
	ok, err = s.HasPerm(ctx, seedAdminLogin, "admins")
	if err != nil || !ok {
		t.Fatalf("главный администратор должен иметь все права: ok=%v err=%v", ok, err)
	}
}
