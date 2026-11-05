package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"escocar-web/internal/database"
	"escocar-web/internal/middleware"
	"escocar-web/internal/models"
)

type CarHandler struct {
	db *database.DB
}

func NewCarHandler(db *database.DB) *CarHandler {
	return &CarHandler{db: db}
}

func (h *CarHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	minYear, _ := strconv.Atoi(q.Get("min_year"))
	maxYear, _ := strconv.Atoi(q.Get("max_year"))
	minPrice, _ := strconv.ParseFloat(q.Get("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(q.Get("max_price"), 64)

	isAdmin := false
	if r.Context().Value(AdminUserContextKey) != nil {
		isAdmin = true
	} else if cookie, err := r.Cookie(AdminSessionCookie); err == nil && cookie.Value != "" {
		if s, err := h.db.GetAdminSession(cookie.Value); err == nil && s != nil {
			isAdmin = true
		}
	}

	filter := models.CarFilter{
		Query:           q.Get("q"),
		Make:            q.Get("make"),
		BodyType:        q.Get("body_type"),
		FuelType:        q.Get("fuel_type"),
		Transmission:    q.Get("transmission"),
		MinYear:         minYear,
		MaxYear:         maxYear,
		MinPrice:        minPrice,
		MaxPrice:        maxPrice,
		Status:          q.Get("status"),
		SortBy:          q.Get("sort_by"),
		FeaturedOnly:    q.Get("featured") == "true",
		IncludeArchived: isAdmin,
	}

	cars, err := h.db.GetCars(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cars)
}

func (h *CarHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	car, err := h.db.GetCarByID(id)
	if err != nil {
		http.Error(w, "Vehículo no encontrado", http.StatusNotFound)
		return
	}

	if car.Status == "archivado" || car.Status == "borrador" {
		isAdmin := false
		if r.Context().Value(AdminUserContextKey) != nil {
			isAdmin = true
		} else if cookie, err := r.Cookie(AdminSessionCookie); err == nil && cookie.Value != "" {
			if s, err := h.db.GetAdminSession(cookie.Value); err == nil && s != nil {
				isAdmin = true
			}
		}
		if !isAdmin {
			http.Error(w, "Esta publicación ya no está disponible", http.StatusGone)
			return
		}
	}

	// Increment view count only for active public cars (not drafts)
	if car.Status != "borrador" {
		_ = h.db.IncrementCarViews(id)
	}

	// If there's visitor context, record car page view
	if vCtx, ok := r.Context().Value(middleware.VisitorContextKey).(*middleware.VisitorContext); ok && vCtx != nil {
		_ = h.db.RecordPageView(&models.PageView{
			VisitorID: vCtx.VisitorID,
			SessionID: vCtx.SessionID,
			Path:      r.URL.Path,
			CarID:     &id,
			Referrer:  r.Referer(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(car)
}
