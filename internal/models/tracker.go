package models

import (
	"time"
)

type Visitor struct {
	ID          string    `json:"id"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	VisitCount  int       `json:"visit_count"`
	UserAgent   string    `json:"user_agent"`
	DeviceType  string    `json:"device_type"`
	Referrer    string    `json:"referrer"`
	IPHash      string    `json:"ip_hash"`
	Notes       string    `json:"notes"`
	Name        string    `json:"name"`
	Phone       string    `json:"phone"`
	Email       string    `json:"email"`
	CarsViewedStr string  `json:"cars_viewed_str"`
	// Dynamic stats
	TotalPageViews int      `json:"total_page_views,omitempty"`
	CarsViewed     []string `json:"cars_viewed,omitempty"`
}

type PageView struct {
	ID              int64     `json:"id"`
	VisitorID       string    `json:"visitor_id"`
	SessionID       string    `json:"session_id"`
	Path            string    `json:"path"`
	CarID           *int      `json:"car_id,omitempty"`
	CarTitle        string    `json:"car_title,omitempty"`
	DurationSeconds int       `json:"duration_seconds"`
	Referrer        string    `json:"referrer"`
	CreatedAt       time.Time `json:"created_at"`
}

type TrafficEvent struct {
	ID        int64     `json:"id"`
	VisitorID string    `json:"visitor_id"`
	SessionID string    `json:"session_id"`
	CarID     *int      `json:"car_id,omitempty"`
	EventType string    `json:"event_type"` // view_detail, whatsapp_click, finance_simulate, filter_search
	EventData string    `json:"event_data"`
	CreatedAt time.Time `json:"created_at"`
}

type Lead struct {
	ID              int       `json:"id"`
	VisitorID       string    `json:"visitor_id"`
	CarID           *int      `json:"car_id,omitempty"`
	CarTitle        string    `json:"car_title,omitempty"`
	Name            string    `json:"name"`
	Phone           string    `json:"phone"`
	Email           string    `json:"email"`
	Message         string    `json:"message"`
	Status          string    `json:"status"` // nuevo, en_gestion, contactado, visita_agendada, cerrado_ganado, descartado
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	VisitorVisits   int       `json:"visitor_visits,omitempty"`
	VisitorDevice   string    `json:"visitor_device,omitempty"`
	VisitorCarsSeen []string  `json:"visitor_cars_seen,omitempty"`
}

type AdminSession struct {
	ID        int       `json:"id"`
	Token     string    `json:"token"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Picture   string    `json:"picture"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type AnalyticsSummary struct {
	TotalVisitors     int            `json:"total_visitors"`
	ReturningVisitors int            `json:"returning_visitors"`
	ReturnRate        float64        `json:"return_rate"`
	TotalPageViews    int            `json:"total_page_views"`
	TodayPageViews    int            `json:"today_page_views"`
	TodayVisitors     int            `json:"today_visitors"`
	TopCars           []TopCarMetric `json:"top_cars"`
	RecentVisitors    []Visitor      `json:"recent_visitors"`
	RecentEvents      []TrafficEvent `json:"recent_events"`
	DeviceBreakdown   map[string]int `json:"device_breakdown"`
}

type TopCarMetric struct {
	CarID       int    `json:"car_id"`
	Make        string `json:"make"`
	Model       string `json:"model"`
	Year        int    `json:"year"`
	Price       string `json:"price"`
	ViewsCount  int    `json:"views_count"`
	UniqueUsers int    `json:"unique_users"`
}
