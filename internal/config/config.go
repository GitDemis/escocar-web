package config

import (
	"os"
	"strings"
)

type Config struct {
	Port               string
	DatabaseURL        string
	UploadDir          string
	AdminUser          string
	AdminPass          string
	BaseURL            string
	AdminDomain        string
	GoogleClientID     string
	GoogleClientSecret string
	AuthorizedAdmins   []string
	SessionSecret      string
	Environment        string
	GAMeasurementID    string
}

func Load() *Config {
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://escocar:escocar_password@localhost:5432/escocardb?sslmode=disable")
	uploadDir := getEnv("UPLOAD_DIR", "./uploads")
	adminUser := getEnv("ADMIN_USER", "admin")
	adminPass := getEnv("ADMIN_PASS", "escocar2026")
	baseURL := getEnv("BASE_URL", "https://escocarautos.com.ar")
	adminDomain := getEnv("ADMIN_DOMAIN", "personalized.escocarautos.com.ar")
	googleClientID := getEnv("GOOGLE_CLIENT_ID", "")
	googleClientSecret := getEnv("GOOGLE_CLIENT_SECRET", "")
	rawAdmins := getEnv("AUTHORIZED_ADMINS", "demis.godoy@gmail.com")
	sessionSecret := getEnv("SESSION_SECRET", "escocar_super_secure_random_session_secret_2026")
	env := getEnv("APP_ENV", "production")
	gaMeasurementID := getEnv("GA_MEASUREMENT_ID", "")

	var authorizedAdmins []string
	for _, a := range strings.Split(rawAdmins, ",") {
		email := strings.ToLower(strings.TrimSpace(a))
		if email != "" {
			authorizedAdmins = append(authorizedAdmins, email)
		}
	}

	return &Config{
		Port:               port,
		DatabaseURL:        dbURL,
		UploadDir:          uploadDir,
		AdminUser:          adminUser,
		AdminPass:          adminPass,
		BaseURL:            baseURL,
		AdminDomain:        adminDomain,
		GoogleClientID:     googleClientID,
		GoogleClientSecret: googleClientSecret,
		AuthorizedAdmins:   authorizedAdmins,
		SessionSecret:      sessionSecret,
		Environment:        env,
		GAMeasurementID:    gaMeasurementID,
	}
}

func (c *Config) IsAuthorizedAdmin(email string) bool {
	clean := strings.ToLower(strings.TrimSpace(email))
	for _, admin := range c.AuthorizedAdmins {
		if clean == admin {
			return true
		}
	}
	return false
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
