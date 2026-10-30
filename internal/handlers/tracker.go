package handlers

import (
	"encoding/json"
	"net/http"

	"escocar-web/internal/database"
	"escocar-web/internal/middleware"
	"escocar-web/internal/models"
)

type TrackerHandler struct {
	db *database.DB
}

func NewTrackerHandler(db *database.DB) *TrackerHandler {
	return &TrackerHandler{db: db}
}

type EventRequest struct {
	CarID     *int   `json:"car_id"`
	EventType string `json:"event_type"` // e.g. "whatsapp_click", "finance_calculate", "call_click"
	EventData string `json:"event_data"`
}

func (h *TrackerHandler) RecordEvent(w http.ResponseWriter, r *http.Request) {
	var req EventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	var visitorID, sessionID string
	if vCtx, ok := r.Context().Value(middleware.VisitorContextKey).(*middleware.VisitorContext); ok && vCtx != nil {
		visitorID = vCtx.VisitorID
		sessionID = vCtx.SessionID
	} else {
		if cookie, err := r.Cookie(middleware.CookieVisitorID); err == nil {
			visitorID = cookie.Value
		}
		if cookie, err := r.Cookie(middleware.CookieSessionID); err == nil {
			sessionID = cookie.Value
		}
	}

	if visitorID == "" {
		http.Error(w, "Visitor ID ausente", http.StatusBadRequest)
		return
	}

	event := &models.TrafficEvent{
		VisitorID: visitorID,
		SessionID: sessionID,
		CarID:     req.CarID,
		EventType: req.EventType,
		EventData: req.EventData,
	}

	if err := h.db.RecordEvent(event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type LeadRequest struct {
	CarID   *int   `json:"car_id"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

func (h *TrackerHandler) RecordLead(w http.ResponseWriter, r *http.Request) {
	var req LeadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Phone == "" {
		http.Error(w, "Nombre y teléfono son obligatorios", http.StatusBadRequest)
		return
	}

	var visitorID string
	if vCtx, ok := r.Context().Value(middleware.VisitorContextKey).(*middleware.VisitorContext); ok && vCtx != nil {
		visitorID = vCtx.VisitorID
	} else if cookie, err := r.Cookie(middleware.CookieVisitorID); err == nil {
		visitorID = cookie.Value
	}

	lead := &models.Lead{
		VisitorID: visitorID,
		CarID:     req.CarID,
		Name:      req.Name,
		Phone:     req.Phone,
		Email:     req.Email,
		Message:   req.Message,
		Status:    "nuevo",
	}

	if err := h.db.RecordLead(lead); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "¡Gracias! Nos pondremos en contacto contigo a la brevedad.",
		"lead_id": lead.ID,
	})
}
