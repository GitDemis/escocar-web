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
	uploadsDir := http.Dir(cfg.UploadDir)

	staticHandler := http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		http.FileServer(staticDir).ServeHTTP(w, r)
	}))

	// ==========================================
	// 1. ADMIN SUBDOMAIN ROUTER (personalized.escocarautos.com.ar)
	// ==========================================
	adminRouter := chi.NewRouter()
	adminRouter.Handle("/static/*", staticHandler)
	adminRouter.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(uploadsDir)))

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
	publicRouter.Handle("/static/*", staticHandler)
	publicRouter.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(uploadsDir)))

	publicRouter.Get("/", pageHandler.Home)
	publicRouter.Get("/catalogo", pageHandler.Catalog)
	publicRouter.Get("/auto/{id}", pageHandler.CarDetail)
	publicRouter.Get("/robots.txt", pageHandler.RobotsTxt)
	publicRouter.Get("/sitemap.xml", pageHandler.SitemapXML)

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
				host := getEffectiveHost(r)
				if isLocalhost(host) {
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
	h := strings.Split(host, ":")[0]
	return h == "localhost" || h == "127.0.0.1" || h == "0.0.0.0"
}
