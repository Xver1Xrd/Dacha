package main

// Границы длины пользовательского ввода — чтобы никто не мог залить
// в хранилище гигантские строки через API.
const (
	maxLoginLen    = 64
	minPassLen     = 6
	maxPassLen     = 200
	maxNameLen     = 100
	maxPhoneLen    = 40
	maxTelegramLen = 200
	maxTextLen     = 2000
	maxColorLen    = 40
)

func withinLimits(s string, max int) bool { return len(s) <= max }
