package models

import (
	"encoding/json"
	"fmt"
	"time"
)

type Car struct {
	ID           int       `json:"id"`
	Make         string    `json:"make"`
	Model        string    `json:"model"`
	Year         int       `json:"year"`
	Price        float64   `json:"price"`
	Currency     string    `json:"currency"`
	DownPayment  float64   `json:"down_payment"`
	Mileage      int       `json:"mileage"`
	FuelType     string    `json:"fuel_type"`
	Transmission string    `json:"transmission"`
	BodyType     string    `json:"body_type"`
	Color        string    `json:"color"`
	Doors        int       `json:"doors"`
	Engine       string    `json:"engine"`
	Description  string    `json:"description"`
	Features     []string  `json:"features"`
	Images       []string  `json:"images"`
	Status       string    `json:"status"` // disponible, reservado, vendido
	Featured     bool      `json:"featured"`
	ViewsCount   int       `json:"views_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (c *Car) FormattedPrice() string {
	if c.Currency == "ARS" {
		return fmt.Sprintf("$ %s", formatNumber(int(c.Price)))
	}
	return fmt.Sprintf("USD %s", formatNumber(int(c.Price)))
}

func (c *Car) FormattedDownPayment() string {
	if c.DownPayment <= 0 {
		return ""
	}
	if c.Currency == "ARS" {
		return fmt.Sprintf("$ %s", formatNumber(int(c.DownPayment)))
	}
	return fmt.Sprintf("USD %s", formatNumber(int(c.DownPayment)))
}

func (c *Car) HasDownPayment() bool {
	return c.DownPayment > 0
}

func (c *Car) FormattedMileage() string {
	return fmt.Sprintf("%s km", formatNumber(c.Mileage))
}

func (c *Car) PrimaryImage() string {
	if len(c.Images) > 0 && c.Images[0] != "" {
		return c.Images[0]
	}
	return "/static/img/hero_showroom.jpg"
}

func (c *Car) FeaturesJSON() string {
	bytes, _ := json.Marshal(c.Features)
	return string(bytes)
}

func (c *Car) ImagesJSON() string {
	bytes, _ := json.Marshal(c.Images)
	return string(bytes)
}

type CarFilter struct {
	Query        string
	Make         string
	BodyType     string
	FuelType     string
	Transmission string
	MinYear      int
	MaxYear      int
	MinPrice     float64
	MaxPrice     float64
	Status       string
	SortBy          string // price_asc, price_desc, year_desc, mileage_asc
	FeaturedOnly    bool
	IncludeArchived bool
}

func FormatNumber(n int) string {
	in := fmt.Sprintf("%d", n)
	out := ""
	length := len(in)
	for i, c := range in {
		if (length-i)%3 == 0 && i != 0 {
			out += "."
		}
		out += string(c)
	}
	return out
}

func formatNumber(n int) string {
	return FormatNumber(n)
}
