package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"escocar-web/internal/config"
	"escocar-web/internal/database"
	"escocar-web/internal/middleware"
	"escocar-web/internal/models"
)

type PageHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewPageHandler(db *database.DB, cfg *config.Config) *PageHandler {
	return &PageHandler{
		db:  db,
		cfg: cfg,
	}
}

func (h *PageHandler) getAdminSession(r *http.Request) *models.AdminSession {
	// 1. Context (if ran through AuthMiddleware)
	if s, ok := r.Context().Value(AdminUserContextKey).(*models.AdminSession); ok && s != nil {
		return s
	}
	// 2. Cookie (Google OAuth session)
	if cookie, err := r.Cookie(AdminSessionCookie); err == nil && cookie.Value != "" && h.db != nil {
		session, err := h.db.GetAdminSession(cookie.Value)
		if err == nil && session != nil {
			return session
		}
	}
	return nil
}

func (h *PageHandler) getGoogleAnalyticsID() string {
	gaID := h.db.GetGoogleAnalyticsID()
	if gaID == "" && h.cfg != nil {
		gaID = h.cfg.GAMeasurementID
	}
	return gaID
}

type PageData struct {
	Title             string
	MetaDescription   string
	MetaKeywords      string
	CanonicalPath     string
	NoIndex           bool
	Cars              []models.Car
	FeaturedCars      []models.Car
	SelectedCar       *models.Car
	VisitorInfo       *middleware.VisitorContext
	Config            *config.Config
	Makes             []string
	SelectedMake      string
	SelectedBodyType  string
	SelectedFuelType  string
	SelectedSortBy    string
	SearchQuery       string
	BodyTypes         []string
	Analytics         *models.AnalyticsSummary
	Leads             []models.Lead
	AdminSession      *models.AdminSession
	SavedLead         *models.Lead
	WhatsApp          string
	GoogleAnalyticsID string
	LookerStudioURL   string
	IsAdmin           bool
}

func (h *PageHandler) Home(w http.ResponseWriter, r *http.Request) {
	featured, _ := h.db.GetCars(models.CarFilter{FeaturedOnly: true})
	allCars, _ := h.db.GetCars(models.CarFilter{})

	vCtx, _ := r.Context().Value(middleware.VisitorContextKey).(*middleware.VisitorContext)
	var savedLead *models.Lead
	if vCtx != nil && vCtx.VisitorID != "" {
		savedLead, _ = h.db.GetLatestLeadByVisitorID(vCtx.VisitorID)
	}

	makes, _ := h.db.GetAvailableMakes()

	data := PageData{
		Title:             "Escocar Autos | Concesionaria de Autos Usados en Escobar, Pilar, Loma Verde, Maschwitz y Garín",
		MetaDescription:   "Escocar Autos (Agencia Escocar en Dr. Travi 970, Belén de Escobar): Concesionaria multimarca líder en Zona Norte (Escobar, Pilar, Loma Verde, Ingeniero Maschwitz, Garín). Venta de autos usados seleccionados, camionetas, motos y náutica. Créditos prendarios, financiación, permutas y consignaciones.",
		CanonicalPath:     "/",
		FeaturedCars:      featured,
		Cars:              allCars,
		Makes:             makes,
		VisitorInfo:       vCtx,
		Config:            h.cfg,
		SavedLead:         savedLead,
		WhatsApp:          h.db.GetWhatsAppNumber(),
		GoogleAnalyticsID: h.getGoogleAnalyticsID(),
		LookerStudioURL:   h.db.GetLookerStudioURL(),
	}

	renderTemplate(w, "index.html", data)
}

func (h *PageHandler) Catalog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	minYear, _ := strconv.Atoi(q.Get("min_year"))
	maxYear, _ := strconv.Atoi(q.Get("max_year"))
	minPrice, _ := strconv.ParseFloat(q.Get("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(q.Get("max_price"), 64)

	makeFilter := strings.TrimSpace(q.Get("make"))
	bodyType := strings.TrimSpace(q.Get("body_type"))
	fuelType := strings.TrimSpace(q.Get("fuel_type"))
	searchQuery := strings.TrimSpace(q.Get("q"))
	sortBy := strings.TrimSpace(q.Get("sort_by"))

	filter := models.CarFilter{
		Query:        searchQuery,
		Make:         makeFilter,
		BodyType:     bodyType,
		FuelType:     fuelType,
		Transmission: q.Get("transmission"),
		MinYear:      minYear,
		MaxYear:      maxYear,
		MinPrice:     minPrice,
		MaxPrice:     maxPrice,
		SortBy:       sortBy,
	}

	cars, err := h.db.GetCars(filter)
	if err != nil {
		cars = []models.Car{}
	}

	vCtx, _ := r.Context().Value(middleware.VisitorContextKey).(*middleware.VisitorContext)
	var savedLead *models.Lead
	if vCtx != nil && vCtx.VisitorID != "" {
		savedLead, _ = h.db.GetLatestLeadByVisitorID(vCtx.VisitorID)
	}

	makes, _ := h.db.GetAvailableMakes()

	title := "Catálogo de Autos Usados, Camionetas, Permutas y Financiación en Escobar y Pilar | Escocar Autos"
	metaDesc := "Explorá el inventario de Escocar Autos en Belén de Escobar (cerca de Pilar, Loma Verde, Maschwitz y Garín). Autos usados seleccionados, camionetas 4x4, SUVs, motos y náutica con créditos prendarios, permutas y consignación."
	canonicalPath := "/catalogo"

	if makeFilter != "" && bodyType != "" {
		title = fmt.Sprintf("%s %s Usados en Escobar, Pilar y Zona Norte | Permuta y Financiación | Escocar Autos", makeFilter, bodyType)
		metaDesc = fmt.Sprintf("Comprá tu %s (%s) en Escocar Autos, Belén de Escobar. Tomamos tu usado en permuta, créditos prendarios en cuotas y entrega inmediata para Pilar, Loma Verde, Maschwitz y Garín.", makeFilter, bodyType)
	} else if makeFilter != "" {
		title = fmt.Sprintf("Autos y Camionetas %s Usados en Escobar, Pilar y Zona Norte | Escocar Autos", makeFilter)
		metaDesc = fmt.Sprintf("Venta de vehículos %s seleccionados en Escocar Autos (Belén de Escobar). Financiación prendaria, permutas llave por llave y consignación en Zona Norte (Pilar, Maschwitz, Loma Verde, Garín).", makeFilter)
	} else if bodyType != "" {
		title = fmt.Sprintf("Venta de %s en Escobar, Pilar, Maschwitz y Garín | Financiación y Permuta | Escocar Autos", bodyType)
		metaDesc = fmt.Sprintf("Catálogo de %s seleccionados en Escocar Autos (Dr. Travi 970, Belén de Escobar). Créditos prendarios, permutas y consignaciones para clientes de Escobar, Pilar, Loma Verde, Maschwitz y Garín.", bodyType)
		canonicalPath = "/catalogo?body_type=" + bodyType
	}

	data := PageData{
		Title:             title,
		MetaDescription:   metaDesc,
		CanonicalPath:     canonicalPath,
		Cars:              cars,
		Makes:             makes,
		SelectedMake:      makeFilter,
		SelectedBodyType:  bodyType,
		SelectedFuelType:  fuelType,
		SelectedSortBy:    sortBy,
		SearchQuery:       searchQuery,
		VisitorInfo:       vCtx,
		Config:            h.cfg,
		SavedLead:         savedLead,
		WhatsApp:          h.db.GetWhatsAppNumber(),
		GoogleAnalyticsID: h.getGoogleAnalyticsID(),
		LookerStudioURL:   h.db.GetLookerStudioURL(),
	}

	renderTemplate(w, "catalog.html", data)
}

func (h *PageHandler) CarDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/catalogo", http.StatusFound)
		return
	}

	car, err := h.db.GetCarByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	adminSession := h.getAdminSession(r)
	isAdmin := adminSession != nil

	// If publication is archived or draft and user is NOT an admin, display the unavailable view
	if (car.Status == "archivado" || car.Status == "borrador") && !isAdmin {
		// Suggest other active vehicles so the user doesn't bounce
		suggested, _ := h.db.GetCars(models.CarFilter{FeaturedOnly: true})
		if len(suggested) == 0 {
			suggested, _ = h.db.GetCars(models.CarFilter{})
		}

		data := PageData{
			Title:             "Publicación no disponible | Escocar Autos",
			CanonicalPath:     fmt.Sprintf("/auto/%d", car.ID),
			NoIndex:           true,
			SelectedCar:       car,
			Cars:              suggested,
			Config:            h.cfg,
			WhatsApp:          h.db.GetWhatsAppNumber(),
			IsAdmin:           false,
			AdminSession:      nil,
			GoogleAnalyticsID: h.getGoogleAnalyticsID(),
			LookerStudioURL:   h.db.GetLookerStudioURL(),
		}
		renderTemplate(w, "detail.html", data)
		return
	}

	if car.Status != "borrador" {
		_ = h.db.IncrementCarViews(id)
	}

	vCtx, _ := r.Context().Value(middleware.VisitorContextKey).(*middleware.VisitorContext)
	var savedLead *models.Lead
	if vCtx != nil && vCtx.VisitorID != "" {
		savedLead, _ = h.db.GetLatestLeadByVisitorID(vCtx.VisitorID)
	}

	// Related cars (only active, excluding archived and drafts)
	related, _ := h.db.GetCars(models.CarFilter{BodyType: car.BodyType})
	var filteredRelated []models.Car
	for _, rel := range related {
		if rel.ID != car.ID && rel.Status != "archivado" && rel.Status != "borrador" {
			filteredRelated = append(filteredRelated, rel)
		}
	}

	desc := fmt.Sprintf("%s %s (%d) con %s en venta en Escocar Autos, Belén de Escobar (cerca de Pilar, Loma Verde, Maschwitz y Garín). Créditos prendarios, financiación, permutas y consignación. %s", car.Make, car.Model, car.Year, car.FormattedMileage(), strings.TrimSpace(car.Description))
	if len(desc) > 300 {
		desc = desc[:297] + "..."
	}

	data := PageData{
		Title:             fmt.Sprintf("%s %s %d en Venta en Escobar y Zona Norte | Permuta y Financiación | Escocar Autos", car.Make, car.Model, car.Year),
		MetaDescription:   desc,
		CanonicalPath:     fmt.Sprintf("/auto/%d", car.ID),
		NoIndex:           car.Status == "archivado" || car.Status == "borrador",
		SelectedCar:       car,
		Cars:              filteredRelated,
		VisitorInfo:       vCtx,
		Config:            h.cfg,
		SavedLead:         savedLead,
		WhatsApp:          h.db.GetWhatsAppNumber(),
		AdminSession:      adminSession,
		IsAdmin:           isAdmin,
		GoogleAnalyticsID: h.getGoogleAnalyticsID(),
		LookerStudioURL:   h.db.GetLookerStudioURL(),
	}

	renderTemplate(w, "detail.html", data)
}

func (h *PageHandler) Admin(w http.ResponseWriter, r *http.Request) {
	cars, _ := h.db.GetCars(models.CarFilter{IncludeArchived: true})
	featured, _ := h.db.GetCars(models.CarFilter{FeaturedOnly: true})
	analytics, _ := h.db.GetAnalyticsSummary()
	leads, _ := h.db.GetLeads()
	adminSession := h.getAdminSession(r)

	data := PageData{
		Title:             "Panel de Control | Escocar Autos",
		CanonicalPath:     "/admin",
		NoIndex:           true,
		Cars:              cars,
		FeaturedCars:      featured,
		Analytics:         analytics,
		Leads:             leads,
		Config:            h.cfg,
		AdminSession:      adminSession,
		IsAdmin:           true,
		WhatsApp:          h.db.GetWhatsAppNumber(),
		GoogleAnalyticsID: h.getGoogleAnalyticsID(),
		LookerStudioURL:   h.db.GetLookerStudioURL(),
	}

	renderTemplate(w, "admin.html", data)
}

var bsAsLocation = func() *time.Location {
	loc, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	if err != nil {
		return time.FixedZone("ART", -3*3600)
	}
	return loc
}()

func renderTemplate(w http.ResponseWriter, tmplName string, data PageData) {
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"seq": func(n int) []int {
			res := make([]int, n)
			for i := range res {
				res[i] = i
			}
			return res
		},
		"formatNumber": func(n int) string {
			return models.FormatNumber(n)
		},
		"formatDateTime": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.In(bsAsLocation).Format("02/01/2006 15:04")
		},
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.In(bsAsLocation).Format("02/01/2006")
		},
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.In(bsAsLocation).Format("15:04")
		},
		"leadStatusLabel": func(s string) string {
			switch s {
			case "nuevo":
				return "Nuevo"
			case "en_gestion":
				return "En Gestión"
			case "contactado":
				return "Contactado"
			case "visita_agendada":
				return "Visita Agendada"
			case "cerrado_ganado":
				return "Vendido"
			case "descartado":
				return "Descartado"
			case "archivado":
				return "Archivado"
			default:
				if s == "" {
					return "Nuevo"
				}
				return s
			}
		},
	}

	tmpl, err := template.New("base").Funcs(funcs).ParseFiles(
		"web/templates/base.html",
		"web/templates/"+tmplName,
	)
	if err != nil {
		http.Error(w, "Error al cargar plantilla: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, "Error al renderizar plantilla: "+err.Error(), http.StatusInternalServerError)
	}
}

func (h *PageHandler) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	content := "User-agent: *\n" +
		"Allow: /\n" +
		"Disallow: /admin\n" +
		"Disallow: /login\n" +
		"Disallow: /auth/\n" +
		"Disallow: /api/\n\n" +
		"Sitemap: https://escocarautos.com.ar/sitemap.xml\n"
	_, _ = w.Write([]byte(content))
}

func (h *PageHandler) SitemapXML(w http.ResponseWriter, r *http.Request) {
	cars, err := h.db.GetCars(models.CarFilter{})
	if err != nil {
		cars = []models.Car{}
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	now := time.Now().Format("2006-01-02")

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	// Home
	sb.WriteString("  <url>\n")
	sb.WriteString("    <loc>https://escocarautos.com.ar/</loc>\n")
	sb.WriteString("    <lastmod>" + now + "</lastmod>\n")
	sb.WriteString("    <changefreq>daily</changefreq>\n")
	sb.WriteString("    <priority>1.0</priority>\n")
	sb.WriteString("  </url>\n")

	// Catalog
	sb.WriteString("  <url>\n")
	sb.WriteString("    <loc>https://escocarautos.com.ar/catalogo</loc>\n")
	sb.WriteString("    <lastmod>" + now + "</lastmod>\n")
	sb.WriteString("    <changefreq>daily</changefreq>\n")
	sb.WriteString("    <priority>0.9</priority>\n")
	sb.WriteString("  </url>\n")

	// Category Landing Pages (Body Types)
	categories := []string{"SUV", "Pickup", "Sedán", "Hatchback", "Coupé", "Moto", "Utilitario", "Náutica"}
	for _, cat := range categories {
		sb.WriteString("  <url>\n")
		sb.WriteString("    <loc>https://escocarautos.com.ar/catalogo?body_type=" + cat + "</loc>\n")
		sb.WriteString("    <lastmod>" + now + "</lastmod>\n")
		sb.WriteString("    <changefreq>weekly</changefreq>\n")
		sb.WriteString("    <priority>0.85</priority>\n")
		sb.WriteString("  </url>\n")
	}

	// Vehicles
	for _, car := range cars {
		if car.Status == "archivado" || car.Status == "borrador" {
			continue
		}
		carLastMod := car.UpdatedAt.Format("2006-01-02")
		if carLastMod == "" || car.UpdatedAt.IsZero() {
			carLastMod = now
		}
		sb.WriteString("  <url>\n")
		sb.WriteString(fmt.Sprintf("    <loc>https://escocarautos.com.ar/auto/%d</loc>\n", car.ID))
		sb.WriteString("    <lastmod>" + carLastMod + "</lastmod>\n")
		sb.WriteString("    <changefreq>weekly</changefreq>\n")
		sb.WriteString("    <priority>0.8</priority>\n")
		sb.WriteString("  </url>\n")
	}

	sb.WriteString("</urlset>\n")
	_, _ = w.Write([]byte(sb.String()))
}

