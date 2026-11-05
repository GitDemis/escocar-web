package config

import (
	"testing"
)

func TestLoadAndAuthorizedAdmins(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("AUTHORIZED_ADMINS", " AdminOne@Example.com , second@escocar.com.ar , ")

	cfg := Load()
	if cfg.Port != "9090" {
		t.Errorf("cfg.Port = %q, want 9090", cfg.Port)
	}
	if len(cfg.AuthorizedAdmins) != 2 {
		t.Fatalf("len(cfg.AuthorizedAdmins) = %d, want 2", len(cfg.AuthorizedAdmins))
	}

	if !cfg.IsAuthorizedAdmin("adminone@example.com") {
		t.Errorf("expected adminone@example.com to be authorized")
	}
	if !cfg.IsAuthorizedAdmin("  SECOND@ESCOCAR.COM.AR ") {
		t.Errorf("expected SECOND@ESCOCAR.COM.AR to be authorized")
	}
	if cfg.IsAuthorizedAdmin("attacker@evil.com") {
		t.Errorf("expected attacker@evil.com NOT to be authorized")
	}
}
