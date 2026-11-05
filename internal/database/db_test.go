package database

import (
	"testing"
)

func TestHashIP(t *testing.T) {
	h1 := HashIP("192.168.1.10")
	h2 := HashIP("192.168.1.10")
	h3 := HashIP("10.0.0.1")

	if len(h1) != 32 {
		t.Errorf("HashIP length = %d, want 32", len(h1))
	}
	if h1 != h2 {
		t.Errorf("HashIP not deterministic: %q != %q", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("HashIP collision between different IPs: %q", h1)
	}
}

func TestValidateGoogleAnalyticsID(t *testing.T) {
	valid := []string{
		"",
		"G-ABC12345",
		"G-1234567890",
		"  g-xyz98765  ",
	}
	for _, v := range valid {
		got, err := ValidateGoogleAnalyticsID(v)
		if err != nil {
			t.Errorf("ValidateGoogleAnalyticsID(%q) unexpected error: %v", v, err)
		}
		if v == "  g-xyz98765  " && got != "G-XYZ98765" {
			t.Errorf("ValidateGoogleAnalyticsID(%q) = %q, want G-XYZ98765", v, got)
		}
	}

	invalid := []string{
		"UA-12345-1",
		"G-123'; DROP TABLE cars;--",
		`G-123"><script>alert(1)</script>`,
		"G-12",
		"random_string",
	}
	for _, inv := range invalid {
		if _, err := ValidateGoogleAnalyticsID(inv); err == nil {
			t.Errorf("ValidateGoogleAnalyticsID(%q) expected error, got nil", inv)
		}
	}
}

func TestValidateLookerStudioURL(t *testing.T) {
	valid := []string{
		"",
		"https://lookerstudio.google.com/embed/reporting/12345/page/67890",
		"https://datastudio.google.com/embed/reporting/abcdef",
	}
	for _, v := range valid {
		if _, err := ValidateLookerStudioURL(v); err != nil {
			t.Errorf("ValidateLookerStudioURL(%q) unexpected error: %v", v, err)
		}
	}

	invalid := []string{
		"javascript:alert(1)",
		"http://lookerstudio.google.com/embed/reporting/123", // not https
		"https://evil.com/lookerstudio.google.com",
		"https://lookerstudio.google.com.evil.com/embed",
		`https://lookerstudio.google.com/embed" onload="alert(1)`,
	}
	for _, inv := range invalid {
		if _, err := ValidateLookerStudioURL(inv); err == nil {
			t.Errorf("ValidateLookerStudioURL(%q) expected error, got nil", inv)
		}
	}
}
