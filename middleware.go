package main

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const csrfCookieName = "snt_csrf"

// maxBodyBytes — верхняя граница размера тела запроса. С запасом покрывает
// любые легитимные формы (отзыв, работник, админ), но не даёт залить в
// хранилище гигантские payload'ы.
const maxBodyBytes = 1 << 20 // 1 МиБ

// isSecureRequest определяет, пришёл ли запрос по HTTPS — напрямую или через
// доверенный обратный прокси (Render/Railway/Fly и т.п. всегда ставят
// X-Forwarded-Proto для запросов, реально пришедших по HTTPS).
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// cspHeader — Content-Security-Policy. Инлайн-скрипт переключателя темы
// разрешён по хэшу (он одинаков в index.html и admin.html), инлайн-стили —
// through 'unsafe-inline', потому что вёрстка активно использует style="".
const cspHeader = "default-src 'self'; " +
	"script-src 'self' 'sha256-FzDp/PPh0TddWaiTi0ZiMBVMRXc/TZQy5m4jqw5h6yI='; " +
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"img-src 'self' data:; " +
	"connect-src 'self'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"object-src 'none'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", cspHeader)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		next.ServeHTTP(w, r)
	})
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

var csrfProtectedMethods = map[string]bool{
	http.MethodPost: true, http.MethodPut: true, http.MethodDelete: true, http.MethodPatch: true,
}

// csrfMiddleware защищает мутирующие запросы от авторизованных пользователей
// схемой double-submit cookie: JS обязан продублировать значение
// не-HttpOnly cookie snt_csrf в заголовке X-CSRF-Token. Анонимные запросы
// (например /api/login, когда сессии ещё нет) не проверяются — там нечего
// сравнивать, а угонять чужую анонимную сессию нечем.
func (srv *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !csrfProtectedMethods[r.Method] {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := srv.currentLogin(r); ok {
			c, err := r.Cookie(csrfCookieName)
			header := r.Header.Get("X-CSRF-Token")
			if err != nil || header == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(header)) != 1 {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "неверный csrf-токен"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
