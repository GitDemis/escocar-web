package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"escocar-web/internal/config"
	"escocar-web/internal/models"
)

func TestRenderTemplates(t *testing.T) {
	wd, _ := os.Getwd()
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	_ = os.Chdir(root)
	defer func() { _ = os.Chdir(wd) }()

	templates := []string{"index.html", "catalog.html", "detail.html", "admin.html"}
	for _, tmpl := range templates {
		w := httptest.NewRecorder()
		data := PageData{
			Title: "Test " + tmpl,
			Cars: []models.Car{
				{ID: 1, Make: "Sea-Doo", Model: "GTX 230", Year: 2024, Price: 28000, Currency: "USD", BodyType: "Náutica", Featured: true, FeaturedOrder: 1, Status: "disponible"},
			},
			FeaturedCars: []models.Car{
				{ID: 1, Make: "Sea-Doo", Model: "GTX 230", Year: 2024, Price: 28000, Currency: "USD", BodyType: "Náutica", Featured: true, FeaturedOrder: 1, Status: "disponible"},
			},
			SelectedCar: &models.Car{
				ID: 1, Make: "Sea-Doo", Model: "GTX 230", Year: 2024, Price: 28000, Currency: "USD", BodyType: "Náutica", Featured: true, FeaturedOrder: 1, Status: "disponible",
			},
			Analytics: &models.AnalyticsSummary{
				DeviceBreakdown: map[string]int{"Desktop": 1},
			},
			Config:   &config.Config{},
			WhatsApp: "5491100000000",
			IsAdmin:  tmpl == "admin.html",
		}
		renderTemplate(w, tmpl, data)
		if w.Code != 200 {
			t.Fatalf("template %s returned status %d: %s", tmpl, w.Code, w.Body.String())
		}
	}

	// Test detail.html with draft status for non-admin visitor
	wDraftPublic := httptest.NewRecorder()
	renderTemplate(wDraftPublic, "detail.html", PageData{
		Title:       "Test Draft Public",
		SelectedCar: &models.Car{ID: 99, Make: "Audi", Model: "RS3", Status: "borrador"},
		IsAdmin:     false,
	})
	if !strings.Contains(wDraftPublic.Body.String(), "Esta publicación ya no está disponible") {
		t.Errorf("expected draft car to show unavailable screen to public visitor")
	}

	// Test detail.html with draft status for admin preview
	wDraftAdmin := httptest.NewRecorder()
	renderTemplate(wDraftAdmin, "detail.html", PageData{
		Title:       "Test Draft Admin",
		SelectedCar: &models.Car{ID: 99, Make: "Audi", Model: "RS3", Status: "borrador"},
		IsAdmin:     true,
	})
	if !strings.Contains(wDraftAdmin.Body.String(), "Publicación en Modo Borrador") {
		t.Errorf("expected draft car to show draft admin banner to admin")
	}
	if !strings.Contains(wDraftAdmin.Body.String(), "publishDraftCar(") || !strings.Contains(wDraftAdmin.Body.String(), "99") {
		t.Errorf("expected draft car to have publishDraftCar button for admin")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"mi foto auto.jpg", "mi_foto_auto"},
		{"../../etc/passwd.png", "passwd"},
		{"frente@#%$!.webp", "frente"},
		{"   .jpg", "img"},
		{"abcdefghijklmnopqrstuvwxyz123456.jpg", "abcdefghijklmnopqrst"},
	}

	for _, tc := range tests {
		got := sanitizeFilename(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsValidImageMime(t *testing.T) {
	// JPEG magic bytes
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	if !isValidImageMime(jpegBytes) {
		t.Errorf("expected JPEG magic bytes to be valid")
	}

	// PNG magic bytes
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	if !isValidImageMime(pngBytes) {
		t.Errorf("expected PNG magic bytes to be valid")
	}

	// HTML/Script disguised as image
	htmlBytes := []byte("<html><script>alert(1)</script></html>")
	if isValidImageMime(htmlBytes) {
		t.Errorf("expected HTML payload to be rejected by isValidImageMime")
	}

	// JXL naked codestream magic bytes (0xFF 0x0A)
	if !isValidJXL([]byte{0xFF, 0x0A, 0x00, 0x01}) {
		t.Errorf("expected JXL codestream magic bytes to be valid")
	}
	// JXL ISOBMFF container magic bytes
	jxlContainer := []byte{0x00, 0x00, 0x00, 0x0C, 'J', 'X', 'L', ' ', 0x0D, 0x0A, 0x87, 0x0A}
	if !isValidJXL(jxlContainer) {
		t.Errorf("expected JXL container magic bytes to be valid")
	}
	if isValidJXL(htmlBytes) {
		t.Errorf("expected HTML payload to be rejected by isValidJXL")
	}
}

func TestAuthMiddlewareGoogleSessionOnly(t *testing.T) {
	cfg := &config.Config{
		GoogleClientID: "my-google-client-id.apps.googleusercontent.com",
	}
	adminHandler := NewAdminHandler(nil, cfg)

	protected := adminHandler.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Unauthenticated browser request -> 302 redirect to /login
	reqHTML := httptest.NewRequest(http.MethodGet, "https://personalized.escocarautos.com.ar/", nil)
	reqHTML.Header.Set("Accept", "text/html")
	recHTML := httptest.NewRecorder()
	protected.ServeHTTP(recHTML, reqHTML)
	if recHTML.Code != http.StatusFound {
		t.Errorf("expected 302 redirect for unauthenticated HTML request, got %d", recHTML.Code)
	}

	// 2. Unauthenticated API request (even with Basic Auth header) -> 401 Rejected and NO WWW-Authenticate header!
	reqAPI := httptest.NewRequest(http.MethodPost, "https://personalized.escocarautos.com.ar/api/cars", nil)
	reqAPI.Host = "personalized.escocarautos.com.ar"
	reqAPI.SetBasicAuth("admin", "escocar2026")
	recAPI := httptest.NewRecorder()
	protected.ServeHTTP(recAPI, reqAPI)
	if recAPI.Code != http.StatusUnauthorized {
		t.Errorf("expected unauthenticated API request to be rejected (401), got %d", recAPI.Code)
	}
	if wwwAuth := recAPI.Header().Get("WWW-Authenticate"); wwwAuth != "" {
		t.Errorf("expected no WWW-Authenticate header, got %q", wwwAuth)
	}
}

func TestTrackerHandlerValidation(t *testing.T) {
	th := NewTrackerHandler(nil)

	// 1. RecordEvent with invalid non-UUID visitor cookie -> 400 Bad Request
	reqEvt := httptest.NewRequest(http.MethodPost, "/api/track/event", bytes.NewBufferString(`{"event_type":"whatsapp_click"}`))
	reqEvt.AddCookie(&http.Cookie{Name: "escocar_vid", Value: "not-a-valid-uuid"})
	recEvt := httptest.NewRecorder()
	th.RecordEvent(recEvt, reqEvt)
	if recEvt.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid visitor UUID in RecordEvent, got %d", recEvt.Code)
	}

	// 2. RecordLead missing phone -> 400 Bad Request
	reqLead := httptest.NewRequest(http.MethodPost, "/api/leads", bytes.NewBufferString(`{"name":"Juan","phone":"   "}`))
	recLead := httptest.NewRecorder()
	th.RecordLead(recLead, reqLead)
	if recLead.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing phone in RecordLead, got %d", recLead.Code)
	}
}

func TestRobotsTxt(t *testing.T) {
	ph := NewPageHandler(nil, &config.Config{})
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rec := httptest.NewRecorder()
	ph.RobotsTxt(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("RobotsTxt status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Disallow: /admin") {
		t.Errorf("RobotsTxt body missing Disallow: /admin")
	}
}

func TestSEOContentAndBrandEntity(t *testing.T) {
	wd, _ := os.Getwd()
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	_ = os.Chdir(root)
	defer func() { _ = os.Chdir(wd) }()

	w := httptest.NewRecorder()
	renderTemplate(w, "index.html", PageData{
		Title:         "Escocar Autos | Concesionaria en Belén de Escobar",
		CanonicalPath: "/",
	})
	body := w.Body.String()

	requiredTerms := []string{
		"Escocar Autos",
		"Belén de Escobar",
		"Pilar",
		"Loma Verde",
		"Ingeniero Maschwitz",
		"Garín",
		"Créditos Prendarios",
		"Permutas",
		"Consignaciones",
		`"@type": "FAQPage"`,
		`"@type": "Brand"`,
	}

	for _, term := range requiredTerms {
		if !strings.Contains(body, term) {
			t.Errorf("expected rendered index.html to contain SEO term %q", term)
		}
	}
}
