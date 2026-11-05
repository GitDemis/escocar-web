package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestIsAdminHost(t *testing.T) {
	adminDomain := "personalized.escocarautos.com.ar"
	if !isAdminHost("personalized.escocarautos.com.ar", adminDomain) {
		t.Errorf("expected personalized.escocarautos.com.ar to be admin host")
	}
	if !isAdminHost("admin.escocarautos.com.ar:8080", adminDomain) {
		t.Errorf("expected admin.escocarautos.com.ar:8080 to be admin host")
	}
	if isAdminHost("escocarautos.com.ar", adminDomain) {
		t.Errorf("expected escocarautos.com.ar NOT to be admin host")
	}
}

func TestIsLocalDevRequest(t *testing.T) {
	// Genuine localhost request
	reqLocal := httptest.NewRequest(http.MethodGet, "http://localhost:8080/admin", nil)
	reqLocal.Host = "localhost:8080"
	if !isLocalDevRequest(reqLocal) {
		t.Errorf("expected genuine localhost request to be allowed")
	}

	// Public host with spoofed X-Forwarded-Host: localhost
	reqSpoof := httptest.NewRequest(http.MethodGet, "https://escocarautos.com.ar/admin", nil)
	reqSpoof.Host = "escocarautos.com.ar"
	reqSpoof.Header.Set("X-Forwarded-Host", "localhost")
	if isLocalDevRequest(reqSpoof) {
		t.Errorf("expected spoofed X-Forwarded-Host on public domain to be blocked")
	}

	// Proxied request where r.Host is localhost but X-Forwarded-Host is public domain
	reqProxiedPublic := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/admin", nil)
	reqProxiedPublic.Host = "127.0.0.1:8080"
	reqProxiedPublic.Header.Set("X-Forwarded-Host", "escocarautos.com.ar")
	if isLocalDevRequest(reqProxiedPublic) {
		t.Errorf("expected proxied public domain request to be blocked from devAdm")
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	h := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q, want SAMEORIGIN", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want strict-origin-when-cross-origin", got)
	}
}

func TestHeadSupportMiddleware(t *testing.T) {
	r := chi.NewRouter()
	r.Use(headSupportMiddleware)

	sub := chi.NewRouter()
	sub.Get("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<urlset></urlset>"))
	})

	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		sub.ServeHTTP(w, req)
	}))

	req := httptest.NewRequest(http.MethodHead, "/sitemap.xml", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD /sitemap.xml status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("HEAD /sitemap.xml body length = %d, want 0", rec.Body.Len())
	}
}

func TestSmartUploadsHandler(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "car_1.jpg"), []byte("orig-jpg"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "car_1.webp"), []byte("full-webp"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "car_1.jxl"), []byte("full-jxl"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "car_1_thumb.webp"), []byte("thumb-webp"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "car_1_thumb.jxl"), []byte("thumb-jxl"), 0644)

	h := newSmartUploadsHandler(tmpDir)

	// 1. Full resolution with JXL support
	reqJXL := httptest.NewRequest(http.MethodGet, "/car_1.jpg", nil)
	reqJXL.Header.Set("Accept", "image/jxl,image/webp,*/*")
	recJXL := httptest.NewRecorder()
	h.ServeHTTP(recJXL, reqJXL)
	if got := recJXL.Header().Get("Content-Type"); got != "image/jxl" || recJXL.Body.String() != "full-jxl" {
		t.Errorf("expected full-jxl (image/jxl), got %q (%q)", recJXL.Body.String(), got)
	}

	// 2. Full resolution with WebP support
	reqWebP := httptest.NewRequest(http.MethodGet, "/car_1.jpg", nil)
	reqWebP.Header.Set("Accept", "image/webp,*/*")
	recWebP := httptest.NewRecorder()
	h.ServeHTTP(recWebP, reqWebP)
	if got := recWebP.Header().Get("Content-Type"); got != "image/webp" || recWebP.Body.String() != "full-webp" {
		t.Errorf("expected full-webp (image/webp), got %q (%q)", recWebP.Body.String(), got)
	}

	// 3. Thumbnail (?thumb=1) with JXL support
	reqThumbJXL := httptest.NewRequest(http.MethodGet, "/car_1.jpg?thumb=1", nil)
	reqThumbJXL.Header.Set("Accept", "image/jxl,image/webp,*/*")
	recThumbJXL := httptest.NewRecorder()
	h.ServeHTTP(recThumbJXL, reqThumbJXL)
	if got := recThumbJXL.Header().Get("Content-Type"); got != "image/jxl" || recThumbJXL.Body.String() != "thumb-jxl" {
		t.Errorf("expected thumb-jxl (image/jxl), got %q (%q)", recThumbJXL.Body.String(), got)
	}

	// 4. Thumbnail (?thumb=1) with WebP support
	reqThumbWebP := httptest.NewRequest(http.MethodGet, "/car_1.jpg?thumb=1", nil)
	reqThumbWebP.Header.Set("Accept", "image/webp,*/*")
	recThumbWebP := httptest.NewRecorder()
	h.ServeHTTP(recThumbWebP, reqThumbWebP)
	if got := recThumbWebP.Header().Get("Content-Type"); got != "image/webp" || recThumbWebP.Body.String() != "thumb-webp" {
		t.Errorf("expected thumb-webp (image/webp), got %q (%q)", recThumbWebP.Body.String(), got)
	}
}

func TestFaviconAndBrandAssets(t *testing.T) {
	requiredAssets := []string{
		"../../web/static/favicon.ico",
		"../../web/static/img/favicon-32x32.png",
		"../../web/static/img/favicon-48x48.png",
		"../../web/static/img/favicon-96x96.png",
		"../../web/static/img/favicon-192x192.png",
		"../../web/static/img/apple-touch-icon.png",
		"../../web/static/site.webmanifest",
	}

	for _, rel := range requiredAssets {
		if _, err := os.Stat(rel); err != nil {
			// Fallback check from project root
			rootPath := filepath.Clean(rel)
			if _, err2 := os.Stat(rootPath); err2 != nil {
				t.Errorf("expected asset %s to exist: %v", rel, err)
			}
		}
	}
}



