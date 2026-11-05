package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"escocar-web/internal/models"
)

func TestIsBotOrScanner(t *testing.T) {
	bots := []string{
		"",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"curl/8.5.0",
		"python-requests/2.31.0",
		"Mozilla/5.0 (compatible; SemrushBot/7~bl)",
		"sqlmap/1.7",
		"HeadlessChrome/120.0.0.0",
	}
	for _, ua := range bots {
		if !isBotOrScanner(ua) {
			t.Errorf("isBotOrScanner(%q) = false, want true", ua)
		}
	}

	humans := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
	}
	for _, ua := range humans {
		if isBotOrScanner(ua) {
			t.Errorf("isBotOrScanner(%q) = true, want false", ua)
		}
	}
}

func TestIsIgnoredPath(t *testing.T) {
	ignored := []string{
		"/static/css/style.css",
		"/uploads/car_1.jpg",
		"/favicon.ico",
		"/robots.txt",
		"/sitemap.xml",
		"/health",
		"/api/cars",
		"/admin",
		"/login",
		"/wp-login.php",
		"/.env",
		"/.git/config",
	}
	for _, p := range ignored {
		if !isIgnoredPath(p) {
			t.Errorf("isIgnoredPath(%q) = false, want true", p)
		}
	}

	tracked := []string{
		"/",
		"/catalogo",
		"/auto/15",
	}
	for _, p := range tracked {
		if isIgnoredPath(p) {
			t.Errorf("isIgnoredPath(%q) = true, want false", p)
		}
	}
}

func TestDetectDevice(t *testing.T) {
	if got := detectDevice("curl/8.0"); got != "Bot" {
		t.Errorf("detectDevice(curl) = %q, want Bot", got)
	}
	if got := detectDevice("Mozilla/5.0 (iPad; CPU OS 16_0 like Mac OS X)"); got != "Tablet" {
		t.Errorf("detectDevice(iPad) = %q, want Tablet", got)
	}
	if got := detectDevice("Mozilla/5.0 (Linux; Android 14; Pixel 8) Mobile"); got != "Mobile" {
		t.Errorf("detectDevice(Android) = %q, want Mobile", got)
	}
	if got := detectDevice("Mozilla/5.0 (X11; Linux x86_64) Chrome/122.0.0.0"); got != "Desktop" {
		t.Errorf("detectDevice(Linux Desktop) = %q, want Desktop", got)
	}
}

func TestGetClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	if got := getClientIP(r); got != "10.0.0.5:1234" {
		t.Errorf("getClientIP RemoteAddr = %q", got)
	}

	r.Header.Set("X-Real-IP", "181.45.10.22")
	if got := getClientIP(r); got != "181.45.10.22" {
		t.Errorf("getClientIP X-Real-IP = %q", got)
	}

	r.Header.Set("X-Forwarded-For", "190.12.34.56, 10.0.0.1")
	if got := getClientIP(r); got != "190.12.34.56" {
		t.Errorf("getClientIP X-Forwarded-For = %q", got)
	}
}

func TestTrackerMiddlewareCookies(t *testing.T) {
	tm := &TrackerMiddleware{
		db:       nil,
		viewChan: make(chan *models.PageView, 10),
	}

	var capturedCtx *VisitorContext
	handler := tm.Track(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, _ = r.Context().Value(VisitorContextKey).(*VisitorContext)
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Human request on "/" sets cookies
	req := httptest.NewRequest(http.MethodGet, "https://escocarautos.com.ar/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/122.0.0.0")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedCtx == nil || capturedCtx.VisitorID == "" || capturedCtx.SessionID == "" {
		t.Fatalf("expected VisitorID and SessionID to be populated for human visitor, got %+v", capturedCtx)
	}
	if len(rec.Result().Cookies()) < 2 {
		t.Errorf("expected at least 2 cookies set, got %d", len(rec.Result().Cookies()))
	}

	// 2. Bot request does NOT set cookies
	reqBot := httptest.NewRequest(http.MethodGet, "https://escocarautos.com.ar/", nil)
	reqBot.Header.Set("User-Agent", "Googlebot/2.1")
	recBot := httptest.NewRecorder()
	handler.ServeHTTP(recBot, reqBot)

	if len(recBot.Result().Cookies()) != 0 {
		t.Errorf("expected 0 cookies for bot, got %d", len(recBot.Result().Cookies()))
	}
}
