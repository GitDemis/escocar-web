package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"escocar-web/internal/config"
	"escocar-web/internal/database"
	"escocar-web/internal/handlers"
	"escocar-web/internal/middleware"
)

func main() {
	// Initialize default timezone to Buenos Aires (UTC-3)
	if loc, err := time.LoadLocation("America/Argentina/Buenos_Aires"); err == nil {
		time.Local = loc
		log.Printf("Timezone configured: %s", loc.String())
	} else {
		time.Local = time.FixedZone("ART", -3*3600)
		log.Printf("Timezone fallback configured: ART (UTC-3)")
	}

	cfg := config.Load()
	log.Printf("Starting Escocar Autos web server on port :%s (env: %s, adminDomain: %s)", cfg.Port, cfg.Environment, cfg.AdminDomain)

	// Ensure upload directory exists
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory %s: %v", cfg.UploadDir, err)
	}

	// Connect to Database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Could not initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Handlers & Middleware
	trackerMW := middleware.NewTrackerMiddleware(db)
	carHandler := handlers.NewCarHandler(db)
	trackerHandler := handlers.NewTrackerHandler(db)
	adminHandler := handlers.NewAdminHandler(db, cfg)
	pageHandler := handlers.NewPageHandler(db, cfg)

	workDir, _ := os.Getwd()
	staticDir := http.Dir(filepath.Join(workDir, "web", "static"))

	staticHandler := http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.FileServer(staticDir).ServeHTTP(w, r)
	}))

	uploadsHandler := http.StripPrefix("/uploads/", newSmartUploadsHandler(cfg.UploadDir))

	faviconPath := filepath.Join(workDir, "web", "static", "favicon.ico")
	appleTouchPath := filepath.Join(workDir, "web", "static", "img", "apple-touch-icon.png")
	manifestPath := filepath.Join(workDir, "web", "static", "site.webmanifest")

	faviconHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		http.ServeFile(w, r, faviconPath)
	}

	appleTouchHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		http.ServeFile(w, r, appleTouchPath)
	}

	manifestHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		http.ServeFile(w, r, manifestPath)
	}

	// ==========================================
	// 1. ADMIN SUBDOMAIN ROUTER (personalized.escocarautos.com.ar)
	// ==========================================
	adminRouter := chi.NewRouter()
	adminRouter.Use(chimw.GetHead)
	adminRouter.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			next.ServeHTTP(w, r)
		})
	})
	adminRouter.Handle("/static/*", staticHandler)
	adminRouter.Handle("/uploads/*", uploadsHandler)
	adminRouter.Get("/favicon.ico", faviconHandler)
	adminRouter.Get("/apple-touch-icon.png", appleTouchHandler)
	adminRouter.Get("/apple-touch-icon-precomposed.png", appleTouchHandler)
	adminRouter.Get("/site.webmanifest", manifestHandler)

	// Public Auth endpoints on admin subdomain
	adminRouter.Get("/login", adminHandler.LoginPage)
	adminRouter.Get("/auth/google/login", adminHandler.GoogleLogin)
	adminRouter.Get("/auth/google/callback", adminHandler.GoogleCallback)
	adminRouter.Get("/auth/logout", adminHandler.Logout)

	mountAdminAPI := func(r chi.Router) {
		r.Get("/analytics", adminHandler.GetAnalytics)
		r.Get("/leads", adminHandler.GetLeads)
		r.Get("/leads/{id}", adminHandler.GetLeadByID)
		r.Put("/leads/{id}", adminHandler.UpdateLead)
		r.Delete("/leads/{id}", adminHandler.DeleteLead)
		r.Put("/cars/featured-order", adminHandler.UpdateFeaturedOrder)
		r.Post("/cars/featured-order", adminHandler.UpdateFeaturedOrder)
		r.Get("/cars", carHandler.List)
		r.Get("/cars/{id}", carHandler.GetByID)
		r.Post("/cars", adminHandler.CreateCar)
		r.Put("/cars/{id}", adminHandler.UpdateCar)
		r.Post("/cars/{id}", adminHandler.UpdateCar)
		r.Delete("/cars/{id}", adminHandler.DeleteCar)
		r.Get("/settings", adminHandler.GetSettings)
		r.Put("/settings", adminHandler.UpdateSettings)
	}

	// Protected Admin Dashboard & APIs
	adminRouter.Group(func(adm chi.Router) {
		adm.Use(adminHandler.AuthMiddleware)

		// Root of subdomain loads Admin Dashboard directly
		adm.Get("/", pageHandler.Admin)

		adm.Route("/api", mountAdminAPI)
		adm.Route("/admin", func(sub chi.Router) {
			sub.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/", http.StatusMovedPermanently)
			})
			sub.Route("/api", mountAdminAPI)
		})

		// Allow tracking & catalog preview on admin subdomain
		adm.Post("/api/track/event", trackerHandler.RecordEvent)
		adm.Get("/catalogo", pageHandler.Catalog)
		adm.Get("/auto/{id}", pageHandler.CarDetail)
	})

	// ==========================================
	// 2. PUBLIC SITE ROUTER (escocarautos.com.ar & www)
	// ==========================================
	publicRouter := chi.NewRouter()
	publicRouter.Use(chimw.GetHead)
	publicRouter.Handle("/static/*", staticHandler)
	publicRouter.Handle("/uploads/*", uploadsHandler)

	publicRouter.Get("/", pageHandler.Home)
	publicRouter.Get("/catalogo", pageHandler.Catalog)
	publicRouter.Get("/auto/{id}", pageHandler.CarDetail)
	publicRouter.Get("/robots.txt", pageHandler.RobotsTxt)
	publicRouter.Get("/sitemap.xml", pageHandler.SitemapXML)
	publicRouter.Get("/favicon.ico", faviconHandler)
	publicRouter.Get("/apple-touch-icon.png", appleTouchHandler)
	publicRouter.Get("/apple-touch-icon-precomposed.png", appleTouchHandler)
	publicRouter.Get("/site.webmanifest", manifestHandler)

	publicRouter.Route("/api", func(api chi.Router) {
		api.Get("/cars", carHandler.List)
		api.Get("/cars/{id}", carHandler.GetByID)
		api.Post("/track/event", trackerHandler.RecordEvent)
		api.Post("/leads", trackerHandler.RecordLead)
	})

	// If accessing via localhost / internal IP in local dev mode, allow /admin
	publicRouter.Group(func(devAdm chi.Router) {
		devAdm.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if isLocalDevRequest(r) {
					next.ServeHTTP(w, r)
					return
				}
				// On public domain, /admin is 404 (hidden)
				http.NotFound(w, r)
			})
		})
		devAdm.Get("/login", adminHandler.LoginPage)
		devAdm.Get("/auth/google/login", adminHandler.GoogleLogin)
		devAdm.Get("/auth/google/callback", adminHandler.GoogleCallback)
		devAdm.Get("/auth/logout", adminHandler.Logout)
		devAdm.With(adminHandler.AuthMiddleware).Get("/admin", pageHandler.Admin)
		devAdm.With(adminHandler.AuthMiddleware).Route("/admin/api", mountAdminAPI)
	})

	// ==========================================
	// 3. MAIN TOP-LEVEL ROUTER WITH HOST DISPATCH
	// ==========================================
	mainRouter := chi.NewRouter()
	mainRouter.Use(chimw.RequestID)
	mainRouter.Use(chimw.RealIP)
	mainRouter.Use(chimw.Logger)
	mainRouter.Use(chimw.Recoverer)
	mainRouter.Use(securityHeadersMiddleware)
	mainRouter.Use(headSupportMiddleware)
	mainRouter.Use(chimw.Compress(5))
	mainRouter.Use(trackerMW.Track)

	// Health check for Podman/Container orchestrator
	mainRouter.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Host-based dispatcher
	mainRouter.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := getEffectiveHost(r)
		if isAdminHost(host, cfg.AdminDomain) {
			adminRouter.ServeHTTP(w, r)
			return
		}
		// Canonical redirect: www.escocarautos.com.ar -> escocarautos.com.ar
		if strings.HasPrefix(host, "www.") {
			target := "https://" + strings.TrimPrefix(host, "www.") + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		publicRouter.ServeHTTP(w, r)
	}))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mainRouter,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🚗 Escocar Web is listening on http://0.0.0.0:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

func getEffectiveHost(r *http.Request) string {
	if xh := r.Header.Get("X-Forwarded-Host"); xh != "" {
		return strings.ToLower(strings.TrimSpace(strings.Split(xh, ",")[0]))
	}
	return strings.ToLower(strings.TrimSpace(r.Host))
}

func isAdminHost(host, adminDomain string) bool {
	h := strings.Split(host, ":")[0]
	admin := strings.Split(adminDomain, ":")[0]
	return h == admin || strings.HasPrefix(h, "personalized.") || strings.HasPrefix(h, "admin.")
}

func isLocalhost(host string) bool {
	h := strings.Split(strings.ToLower(strings.TrimSpace(host)), ":")[0]
	return h == "localhost" || h == "127.0.0.1" || h == "0.0.0.0"
}

// isLocalDevRequest returns true only when the request's direct Host is localhost
// and X-Forwarded-Host is either absent or also localhost, preventing header-spoofing
// from exposing /admin on the public router.
func isLocalDevRequest(r *http.Request) bool {
	if !isLocalhost(r.Host) {
		return false
	}
	if xh := r.Header.Get("X-Forwarded-Host"); xh != "" {
		return isLocalhost(getEffectiveHost(r))
	}
	return true
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

type headResponseWriter struct {
	http.ResponseWriter
}

func (w *headResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

// headSupportMiddleware routes HTTP HEAD requests through GET handlers while discarding the body,
// ensuring crawlers (Googlebot, Search Console, link previewers) receive 200 OK instead of 405.
func headSupportMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			rClone := r.Clone(r.Context())
			rClone.Method = http.MethodGet
			next.ServeHTTP(&headResponseWriter{ResponseWriter: w}, rClone)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newSmartUploadsHandler serves vehicle images from uploadDir with automatic content negotiation:
// - When ?thumb=1 is requested (Home & Catalog cards), it serves the lightweight _thumb.jxl or _thumb.webp variant.
// - When requested without ?thumb=1 (Car Detail page & Lightbox zoom), it serves the 100% original pixel count .jxl or .webp variant.
// - Falls back to the EXIF-stripped base file if a modern variant is unavailable or not supported by the client.
func newSmartUploadsHandler(uploadDir string) http.Handler {
	fallbackFS := http.FileServer(http.Dir(uploadDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("CDN-Cache-Control", "no-store")
		w.Header().Set("Cloudflare-CDN-Cache-Control", "no-store")
		w.Header().Add("Vary", "Accept")

		baseName := filepath.Base(r.URL.Path)
		if baseName == "." || baseName == "/" || baseName == "" {
			http.NotFound(w, r)
			return
		}

		ext := filepath.Ext(baseName)
		stem := strings.TrimSuffix(baseName, ext)
		accept := strings.ToLower(r.Header.Get("Accept"))
		wantsJXL := strings.Contains(accept, "image/jxl")
		wantsWebP := strings.Contains(accept, "image/webp")
		isThumb := r.URL.Query().Get("thumb") == "1"

		type candidate struct {
			filename    string
			contentType string
		}
		var candidates []candidate

		if isThumb {
			if wantsJXL {
				candidates = append(candidates, candidate{stem + "_thumb.jxl", "image/jxl"})
			}
			if wantsWebP {
				candidates = append(candidates, candidate{stem + "_thumb.webp", "image/webp"})
			}
			candidates = append(candidates, candidate{stem + "_thumb.webp", "image/webp"})
		}

		if wantsJXL {
			candidates = append(candidates, candidate{stem + ".jxl", "image/jxl"})
		}
		if wantsWebP {
			candidates = append(candidates, candidate{stem + ".webp", "image/webp"})
		}

		for _, c := range candidates {
			fullPath := filepath.Join(uploadDir, c.filename)
			if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
				w.Header().Set("Content-Type", c.contentType)
				http.ServeFile(w, r, fullPath)
				return
			}
		}

		fallbackFS.ServeHTTP(w, r)
	})
}
