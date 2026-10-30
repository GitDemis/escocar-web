package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"escocar-web/internal/database"
	"escocar-web/internal/models"
)

type contextKey string

const (
	VisitorContextKey contextKey = "visitor_info"
	CookieVisitorID   string     = "escocar_vid"
	CookieSessionID   string     = "escocar_sid"
)

type VisitorContext struct {
	VisitorID   string
	SessionID   string
	IsReturning bool
	DeviceType  string
	IPHash      string
}

type TrackerMiddleware struct {
	db       *database.DB
	viewChan chan *models.PageView
}

func NewTrackerMiddleware(db *database.DB) *TrackerMiddleware {
	tm := &TrackerMiddleware{
		db:       db,
		viewChan: make(chan *models.PageView, 1000),
	}

	// Background worker for non-blocking page view recording
	go tm.processQueue()

	return tm
}

func (tm *TrackerMiddleware) processQueue() {
	for pv := range tm.viewChan {
		_ = tm.db.RecordPageView(pv)
	}
}

func (tm *TrackerMiddleware) Track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		ua := r.UserAgent()
		host := getEffectiveHost(r)

		isBot := isBotOrScanner(ua)
		isExcluded := isIgnoredPath(path) || isAdminHost(host)

		// 1. Get or create Visitor ID
		var visitorID string
		isReturning := false

		if cookie, err := r.Cookie(CookieVisitorID); err == nil && cookie.Value != "" {
			visitorID = cookie.Value
			isReturning = true
		} else if !isBot && !isExcluded {
			visitorID = uuid.New().String()
			http.SetCookie(w, &http.Cookie{
				Name:     CookieVisitorID,
				Value:    visitorID,
				Path:     "/",
				MaxAge:   365 * 24 * 3600, // 1 year
				HttpOnly: false,           // Accessible to frontend for custom event pings
				SameSite: http.SameSiteLaxMode,
			})
		}

		// 2. Get or create Session ID
		var sessionID string
		isNewSession := false
		if cookie, err := r.Cookie(CookieSessionID); err == nil && cookie.Value != "" {
			sessionID = cookie.Value
		} else if !isBot && !isExcluded {
			isNewSession = true
			sessionID = "s_" + uuid.New().String()
			http.SetCookie(w, &http.Cookie{
				Name:     CookieSessionID,
				Value:    sessionID,
				Path:     "/",
				MaxAge:   1800, // 30 minutes
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
			})
		}

		devType := detectDevice(ua)
		ip := getClientIP(r)
		ipHash := database.HashIP(ip)

		vCtx := &VisitorContext{
			VisitorID:   visitorID,
			SessionID:   sessionID,
			IsReturning: isReturning,
			DeviceType:  devType,
			IPHash:      ipHash,
		}

		ctx := context.WithValue(r.Context(), VisitorContextKey, vCtx)

		// Record visitor and pageview ONLY for real human visits on public pages
		if !isBot && !isExcluded && visitorID != "" && tm.db != nil {
			// Register/update visitor in database
			visitor := &models.Visitor{
				ID:         visitorID,
				UserAgent:  ua,
				DeviceType: devType,
				Referrer:   r.Referer(),
				IPHash:     ipHash,
			}
			go func() {
				_, _ = tm.db.UpsertVisitor(visitor, isNewSession)
			}()

			// Extract car_id if path is /auto/:id
			var carID *int
			if strings.HasPrefix(path, "/auto/") {
				idStr := strings.Trim(strings.TrimPrefix(path, "/auto/"), "/")
				if cid, err := strconv.Atoi(idStr); err == nil {
					carID = &cid
				}
			}

			// Record page view async
			select {
			case tm.viewChan <- &models.PageView{
				VisitorID: visitorID,
				SessionID: sessionID,
				Path:      path,
				CarID:     carID,
				Referrer:  r.Referer(),
				CreatedAt: time.Now(),
			}:
			default:
				// queue full, drop silently to prevent backpressure
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isBotOrScanner(ua string) bool {
	if ua == "" {
		return true
	}
	lower := strings.ToLower(ua)
	botKeywords := []string{
		"bot", "crawler", "spider", "crawl", "slurp", "mediapartners",
		"curl", "wget", "httpie", "go-http-client", "python", "urllib", "requests", "aiohttp",
		"scan", "nikto", "sqlmap", "nmap", "zgrab", "censys", "shodan", "leakix", "yokohama",
		"postman", "insomnia", "headlesschrome", "phantomjs", "selenium", "puppeteer", "playwright",
		"ahrefs", "semrush", "dotbot", "mj12bot", "petalbot", "bytespider", "yandex", "baidu",
		"expanse", "netcraft", "internet-measurement", "research", "survey",
	}
	for _, kw := range botKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func isIgnoredPath(path string) bool {
	// Static assets
	if strings.HasPrefix(path, "/static/") ||
		strings.HasPrefix(path, "/uploads/") ||
		path == "/favicon.ico" ||
		path == "/favicon.png" ||
		path == "/robots.txt" ||
		path == "/sitemap.xml" {
		return true
	}

	// Health check, auth, admin, and APIs
	if path == "/health" ||
		path == "/healthz" ||
		path == "/ping" ||
		strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/admin") ||
		strings.HasPrefix(path, "/auth/") ||
		path == "/login" ||
		path == "/logout" {
		return true
	}

	// Common scanner / exploit probes
	lower := strings.ToLower(path)
	scannerExts := []string{
		".php", ".env", ".git", ".asp", ".aspx", ".jsp", ".cgi",
		".sql", ".bak", ".zip", ".tar", ".gz", ".yml", ".yaml",
		".json", ".txt", ".xml", ".action", ".do",
	}
	for _, ext := range scannerExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	if strings.Contains(lower, "setup") ||
		strings.Contains(lower, "wp-") ||
		strings.Contains(lower, "wordpress") ||
		strings.Contains(lower, "cgi-bin") ||
		strings.Contains(lower, "shell") ||
		strings.Contains(lower, "pma") ||
		strings.Contains(lower, "myadmin") {
		return true
	}

	return false
}

func isAdminHost(host string) bool {
	h := strings.ToLower(strings.Split(host, ":")[0])
	return strings.HasPrefix(h, "personalized.") || strings.HasPrefix(h, "admin.")
}

func getEffectiveHost(r *http.Request) string {
	if xh := r.Header.Get("X-Forwarded-Host"); xh != "" {
		return strings.ToLower(strings.TrimSpace(strings.Split(xh, ",")[0]))
	}
	return strings.ToLower(strings.TrimSpace(r.Host))
}

func detectDevice(ua string) string {
	if isBotOrScanner(ua) {
		return "Bot"
	}
	lower := strings.ToLower(ua)
	if strings.Contains(lower, "ipad") || strings.Contains(lower, "tablet") {
		return "Tablet"
	}
	if strings.Contains(lower, "mobile") || strings.Contains(lower, "iphone") || strings.Contains(lower, "android") {
		return "Mobile"
	}
	return "Desktop"
}

func getClientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	return r.RemoteAddr
}
