package models

import (
	"testing"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.000"},
		{15400, "15.400"},
		{1250000, "1.250.000"},
		{28500000, "28.500.000"},
	}

	for _, tc := range tests {
		got := FormatNumber(tc.input)
		if got != tc.want {
			t.Errorf("FormatNumber(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCarFormattingAndHelpers(t *testing.T) {
	carUSD := &Car{
		Price:       24500,
		Currency:    "USD",
		DownPayment: 8000,
		Mileage:     45200,
		Features:    []string{"Cuero", "Techo solar"},
		Images:      []string{"/uploads/car1.jpg", "/uploads/car2.jpg"},
	}

	if got := carUSD.FormattedPrice(); got != "USD 24.500" {
		t.Errorf("FormattedPrice USD = %q, want %q", got, "USD 24.500")
	}
	if !carUSD.HasDownPayment() {
		t.Errorf("HasDownPayment() = false, want true")
	}
	if got := carUSD.FormattedDownPayment(); got != "USD 8.000" {
		t.Errorf("FormattedDownPayment USD = %q, want %q", got, "USD 8.000")
	}
	if got := carUSD.FormattedMileage(); got != "45.200 km" {
		t.Errorf("FormattedMileage = %q, want %q", got, "45.200 km")
	}
	if got := carUSD.PrimaryImage(); got != "/uploads/car1.jpg" {
		t.Errorf("PrimaryImage = %q, want %q", got, "/uploads/car1.jpg")
	}
	if got := carUSD.FeaturesJSON(); got != `["Cuero","Techo solar"]` {
		t.Errorf("FeaturesJSON = %q, want %q", got, `["Cuero","Techo solar"]`)
	}
	if got := carUSD.ImagesJSON(); got != `["/uploads/car1.jpg","/uploads/car2.jpg"]` {
		t.Errorf("ImagesJSON = %q, want %q", got, `["/uploads/car1.jpg","/uploads/car2.jpg"]`)
	}

	carARS := &Car{
		Price:       18500000,
		Currency:    "ARS",
		DownPayment: 0,
		Images:      nil,
	}

	if got := carARS.FormattedPrice(); got != "$ 18.500.000" {
		t.Errorf("FormattedPrice ARS = %q, want %q", got, "$ 18.500.000")
	}
	if carARS.HasDownPayment() {
		t.Errorf("HasDownPayment() = true, want false")
	}
	if got := carARS.FormattedDownPayment(); got != "" {
		t.Errorf("FormattedDownPayment zero = %q, want empty", got)
	}
	if got := carARS.PrimaryImage(); got != "/static/img/hero_showroom.jpg" {
		t.Errorf("PrimaryImage fallback = %q, want %q", got, "/static/img/hero_showroom.jpg")
	}
}
