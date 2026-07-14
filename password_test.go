package main

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	salt, hash := hashPassword("secret123")
	if !verifyPassword("secret123", salt, hash) {
		t.Fatal("правильный пароль не прошёл проверку")
	}
	if verifyPassword("wrong", salt, hash) {
		t.Fatal("неправильный пароль прошёл проверку")
	}
}

func TestVerifyPasswordBadEncoding(t *testing.T) {
	if verifyPassword("x", "not-hex", "not-hex") {
		t.Fatal("verifyPassword должен вернуть false на битой соли/хэше")
	}
}

func TestParseInt(t *testing.T) {
	cases := map[string]int{
		"0": 0, "1": 1, "42": 42, "": 0, "-1": 0, "1a": 0, "abc": 0,
	}
	for in, want := range cases {
		if got := parseInt(in); got != want {
			t.Errorf("parseInt(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestWithinLimits(t *testing.T) {
	if !withinLimits("abc", 3) {
		t.Fatal("строка длиной 3 должна укладываться в лимит 3")
	}
	if withinLimits("abcd", 3) {
		t.Fatal("строка длиной 4 не должна укладываться в лимит 3")
	}
}
