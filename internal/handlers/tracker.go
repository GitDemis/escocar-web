package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

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

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

type EventRequest struct {
	CarID     *int   `json:"car_id"`
	EventType string `json:"event_type"` // e.g. "whatsapp_click", "finance_calculate", "call_click"
	EventData string `json:"event_data"`
}

func (h *TrackerHandler) RecordEvent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64 KB max payload

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

	if visitorID == "" || uuid.Validate(visitorID) != nil {
		http.Error(w, "Visitor ID ausente o inválido", http.StatusBadRequest)
		return
	}

	event := &models.TrafficEvent{
		VisitorID: visitorID,
		SessionID: truncateString(sessionID, 64),
		CarID:     req.CarID,
		EventType: truncateString(req.EventType, 64),
		EventData: truncateString(req.EventData, 2048),
	}

	if err := h.db.RecordEvent(event); err != nil {
		http.Error(w, "Error al registrar evento", http.StatusInternalServerError)
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
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64 KB max payload

	var req LeadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	req.Name = truncateString(req.Name, 120)
	req.Phone = truncateString(req.Phone, 50)
	req.Email = truncateString(req.Email, 160)
	req.Message = truncateString(req.Message, 2000)

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
	if uuid.Validate(visitorID) != nil {
		visitorID = ""
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
		http.Error(w, "Error al guardar consulta", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "¡Gracias! Nos pondremos en contacto contigo a la brevedad.",
		"lead_id": lead.ID,
	})
}
