package main

import "testing"

func TestLoginLimiterBlocksAfterMaxAttempts(t *testing.T) {
	l := &LoginLimiter{m: map[string]*loginAttempt{}}
	key := "1.2.3.4|admin"

	for i := 0; i < loginMaxAttempts-1; i++ {
		l.RecordFailure(key)
		if blocked, _ := l.Blocked(key); blocked {
			t.Fatalf("не должно блокироваться раньше %d попыток (попытка %d)", loginMaxAttempts, i+1)
		}
	}
	l.RecordFailure(key)
	blocked, retryAfter := l.Blocked(key)
	if !blocked {
		t.Fatalf("должно блокироваться после %d неудачных попыток", loginMaxAttempts)
	}
	if retryAfter <= 0 {
		t.Fatalf("retryAfter должен быть положительным, получили %d", retryAfter)
	}
}

func TestLoginLimiterSuccessResets(t *testing.T) {
	l := &LoginLimiter{m: map[string]*loginAttempt{}}
	key := "1.2.3.4|admin"

	l.RecordFailure(key)
	l.RecordFailure(key)
	l.RecordSuccess(key)

	if _, ok := l.m[key]; ok {
		t.Fatal("успешный вход должен сбрасывать счётчик попыток")
	}
}

func TestLoginLimiterKeyIsolatesLoginAndIP(t *testing.T) {
	l := &LoginLimiter{m: map[string]*loginAttempt{}}
	for i := 0; i < loginMaxAttempts; i++ {
		l.RecordFailure("1.2.3.4|admin")
	}
	if blocked, _ := l.Blocked("1.2.3.4|other"); blocked {
		t.Fatal("блокировка одного логина не должна задевать другой логин с того же IP")
	}
	if blocked, _ := l.Blocked("5.6.7.8|admin"); blocked {
		t.Fatal("блокировка по IP не должна задевать тот же логин с другого IP")
	}
}
