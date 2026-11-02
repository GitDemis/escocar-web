package handlers

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"escocar-web/internal/config"
	"escocar-web/internal/models"
)

func TestRenderTemplates(t *testing.T) {
	// Ensure working directory is repo root where web/templates lives
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
}
