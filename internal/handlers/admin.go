package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"escocar-web/internal/config"
	"escocar-web/internal/database"
	"escocar-web/internal/models"
)

const (
	AdminSessionCookie = "escocar_admin_session"
	OAuthStateCookie   = "escocar_oauth_state"
)

type adminContextKey string
const AdminUserContextKey adminContextKey = "admin_user"

type AdminHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAdminHandler(db *database.DB, cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		db:  db,
		cfg: cfg,
	}
}

// AuthMiddleware verifies session cookie or basic auth fallback
func (h *AdminHandler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check Session Cookie from Google OAuth
		if cookie, err := r.Cookie(AdminSessionCookie); err == nil && cookie.Value != "" {
			session, err := h.db.GetAdminSession(cookie.Value)
			if err == nil && session != nil {
				ctx := context.WithValue(r.Context(), AdminUserContextKey, session)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// 2. Fallback Basic Auth (for API / CLI or while setting up OAuth)
		user, pass, ok := r.BasicAuth()
		if ok && subtle.ConstantTimeCompare([]byte(user), []byte(h.cfg.AdminUser)) == 1 &&
			subtle.ConstantTimeCompare([]byte(pass), []byte(h.cfg.AdminPass)) == 1 {
			adminSession := &models.AdminSession{
				Email: h.cfg.AdminUser + "@escocar.local",
				Name:  "Administrador Local",
			}
			ctx := context.WithValue(r.Context(), AdminUserContextKey, adminSession)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 3. If HTML browser request, redirect to login page
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Otherwise return 401 Unauthorized
		w.Header().Set("WWW-Authenticate", `Basic realm="Escocar Admin"`)
		http.Error(w, "Acceso no autorizado", http.StatusUnauthorized)
	})
}

// LoginPage renders the sleek Google Sign-In portal
func (h *AdminHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to admin dashboard
	if cookie, err := r.Cookie(AdminSessionCookie); err == nil && cookie.Value != "" {
		if session, err := h.db.GetAdminSession(cookie.Value); err == nil && session != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}

	tmpl := `
<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <link rel="icon" type="image/png" href="/static/img/logochicosq.png">
  <link rel="apple-touch-icon" href="/static/img/logochicosq.png">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Outfit:wght@600;700;800&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #08090c;
      --card: rgba(18, 20, 26, 0.88);
      --border: rgba(255, 255, 255, 0.08);
      --red: #ff0000;
      --red-light: #ff2222;
      --gold: #ff0000;
      --gold-light: #ff2222;
      --text: #f0f6fc;
      --text-muted: #8b949e;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: radial-gradient(circle at 50% 0%, #161f30 0%, var(--bg) 70%);
      color: var(--text);
      font-family: 'Inter', sans-serif;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 1.5rem;
    }
    .login-card {
      background: var(--card);
      border: 1px solid var(--border);
      backdrop-filter: blur(20px);
      -webkit-backdrop-filter: blur(20px);
      border-radius: 20px;
      padding: 3rem 2.5rem;
      max-width: 440px;
      width: 100%;
      text-align: center;
      box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.7);
    }
    .brand-logo {
      display: inline-flex;
      align-items: center;
      gap: 0.6rem;
      font-family: 'Outfit', sans-serif;
      font-size: 1.8rem;
      font-weight: 800;
      margin-bottom: 0.5rem;
    }
    .brand-icon {
      width: 42px;
      height: 42px;
      background: linear-gradient(135deg, var(--gold-light), var(--gold));
      color: #000;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 10px;
      font-size: 1.3rem;
    }
    .portal-tag {
      font-size: 0.75rem;
      font-weight: 700;
      letter-spacing: 0.1em;
      text-transform: uppercase;
      color: var(--gold);
      margin-bottom: 2rem;
      display: block;
    }
    .desc {
      color: var(--text-muted);
      font-size: 0.95rem;
      line-height: 1.6;
      margin-bottom: 2.2rem;
    }
    .btn-google {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 0.85rem;
      width: 100%;
      padding: 0.9rem 1.25rem;
      background: #ffffff;
      color: #1f2937;
      border: none;
      border-radius: 12px;
      font-size: 0.95rem;
      font-weight: 600;
      text-decoration: none;
      box-shadow: 0 4px 12px rgba(0,0,0,0.15);
      transition: all 0.2s ease;
      cursor: pointer;
    }
    .btn-google:hover {
      background: #f3f4f6;
      transform: translateY(-2px);
      box-shadow: 0 8px 20px rgba(0,0,0,0.25);
    }
    .notice {
      margin-top: 2rem;
      padding-top: 1.5rem;
      border-top: 1px solid var(--border);
      font-size: 0.8rem;
      color: #6e7681;
    }
    .alert-error {
      background: rgba(239, 68, 68, 0.12);
      border: 1px solid rgba(239, 68, 68, 0.3);
      color: #f87171;
      padding: 0.85rem;
      border-radius: 10px;
      font-size: 0.88rem;
      margin-bottom: 1.5rem;
      text-align: left;
    }
  </style>
</head>
<body>
  <div class="login-card">
    <div class="brand-logo" style="margin-bottom: 1.25rem;">
      <img src="/static/img/logochico.png" alt="Escocar Autos" style="height: 46px; max-width: 100%; object-fit: contain;">
    </div>
    <span class="portal-tag">Portal Administrativo Privado</span>

    {{if .Error}}
    <div class="alert-error">
      {{.Error}}
    </div>
    {{end}}

    <p class="desc">
      El acceso a este portal está restringido exclusivamente a cuentas de Google autorizadas de la concesionaria.
    </p>

    <a href="/auth/google/login" class="btn-google">
      <svg width="20" height="20" viewBox="0 0 24 24">
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
        <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
      </svg>
      Iniciar sesión con Google
    </a>

    <div class="notice">
      Subdominio seguro • SSL Cifrado • Escocar Autos &copy; 2026
    </div>
  </div>
</body>
</html>
`
	t, _ := template.New("login").Parse(tmpl)
	data := map[string]string{
		"Error": r.URL.Query().Get("error"),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.Execute(w, data)
}

// GoogleLogin initiates the OAuth2 redirect
func (h *AdminHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GoogleClientID == "" {
		http.Redirect(w, r, "/login?error=Google+OAuth+no+está+configurado+aún+(falta+GOOGLE_CLIENT_ID)", http.StatusFound)
		return
	}

	state := generateRandomString(32)
	http.SetCookie(w, &http.Cookie{
		Name:     OAuthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})

	redirectURI := h.getRedirectURI(r)

	params := url.Values{}
	params.Set("client_id", h.cfg.GoogleClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	params.Set("access_type", "online")
	params.Set("prompt", "select_account")

	googleAuthURL := "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
	http.Redirect(w, r, googleAuthURL, http.StatusFound)
}

// GoogleCallback processes code exchange and validates admin whitelist
func (h *AdminHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(OAuthStateCookie)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Redirect(w, r, "/login?error=Validación+CSRF+de+estado+inválida.+Reintentar.", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/login?error=Código+de+autorización+no+proporcionado+por+Google", http.StatusFound)
		return
	}

	redirectURI := h.getRedirectURI(r)

	// Exchange code for token
	tokenResp, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"code":          {code},
		"client_id":     {h.cfg.GoogleClientID},
		"client_secret": {h.cfg.GoogleClientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		http.Redirect(w, r, "/login?error=Fallo+en+el+intercambio+de+tokens+con+Google", http.StatusFound)
		return
	}
	defer tokenResp.Body.Close()

	var tokenData struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil || tokenData.AccessToken == "" {
		http.Redirect(w, r, "/login?error=Respuesta+inválida+de+Google:+"+tokenData.ErrorDesc, http.StatusFound)
		return
	}

	// Fetch user profile from Google UserInfo
	userInfoReq, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	userInfoReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	userResp, err := http.DefaultClient.Do(userInfoReq)
	if err != nil {
		http.Redirect(w, r, "/login?error=No+se+pudo+obtener+el+perfil+del+usuario", http.StatusFound)
		return
	}
	defer userResp.Body.Close()

	var googleUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&googleUser); err != nil || googleUser.Email == "" {
		http.Redirect(w, r, "/login?error=No+se+pudo+leer+el+email+de+la+cuenta", http.StatusFound)
		return
	}

	// Create session (Authorized users are defined directly in the GCP OAuth client)
	sessionToken := generateRandomString(32)
	expiresAt := time.Now().Add(24 * 7 * time.Hour) // 7 days session

	if err := h.db.CreateAdminSession(sessionToken, googleUser.Email, googleUser.Name, googleUser.Picture, expiresAt); err != nil {
		fmt.Printf("Error creating admin session for %s: %v\n", googleUser.Email, err)
		http.Redirect(w, r, "/login?error=Error+al+guardar+la+sesión", http.StatusFound)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     AdminSessionCookie,
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int((24 * 7 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// Logout terminates the session
func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(AdminSessionCookie); err == nil {
		_ = h.db.DeleteAdminSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     AdminSessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *AdminHandler) renderUnauthorizedPage(w http.ResponseWriter, email, name, picture string) {
	tmpl := `
<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <title>Acceso No Autorizado | Escocar Autos</title>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700&family=Outfit:wght@700;800&display=swap" rel="stylesheet">
  <style>
    body { background: #0a0d12; color: #fff; font-family: 'Inter', sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
    .card { background: #161c26; border: 1px solid rgba(239, 68, 68, 0.4); border-radius: 16px; padding: 2.5rem; max-width: 440px; text-align: center; }
    .avatar { width: 70px; height: 70px; border-radius: 50%; border: 3px solid #ef4444; margin-bottom: 1rem; }
    h2 { font-family: 'Outfit', sans-serif; color: #ef4444; margin-bottom: 0.5rem; }
    p { color: #94a3b8; font-size: 0.95rem; line-height: 1.5; margin-bottom: 1.5rem; }
    .email-box { background: rgba(0,0,0,0.4); padding: 0.5rem 1rem; border-radius: 6px; font-weight: 600; color: #f87171; display: inline-block; margin-bottom: 1.5rem; font-size: 0.9rem; }
    .btn { display: inline-block; background: #374151; color: #fff; text-decoration: none; padding: 0.75rem 1.5rem; border-radius: 8px; font-weight: 600; }
    .btn:hover { background: #4b5563; }
  </style>
</head>
<body>
  <div class="card">
    {{if .Picture}}<img src="{{.Picture}}" class="avatar">{{end}}
    <h2>Acceso No Autorizado</h2>
    <div class="email-box">{{.Email}}</div>
    <p>Esta cuenta de Google no tiene permisos de administrador en Escocar Autos. Si creés que se trata de un error, solicitá a la administración que agregue tu correo a la lista autorizada.</p>
    <a href="/login" class="btn">Intentar con otra cuenta</a>
  </div>
</body>
</html>`
	t, _ := template.New("unauth").Parse(tmpl)
	w.WriteHeader(http.StatusForbidden)
	_ = t.Execute(w, map[string]string{
		"Email":   email,
		"Name":    name,
		"Picture": picture,
	})
}

func (h *AdminHandler) getRedirectURI(r *http.Request) string {
	proto := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" && !strings.Contains(r.Host, "personalized.escocarautos.com.ar") {
		proto = "http"
	}
	return fmt.Sprintf("%s://%s/auth/google/callback", proto, r.Host)
}

func generateRandomString(n int) string {
	bytes := make([]byte, n)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Admin API endpoints

func (h *AdminHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	summary, err := h.db.GetAnalyticsSummary()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func (h *AdminHandler) GetLeads(w http.ResponseWriter, r *http.Request) {
	leads, err := h.db.GetLeads()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(leads)
}

func (h *AdminHandler) GetLeadByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID de lead inválido", http.StatusBadRequest)
		return
	}

	lead, err := h.db.GetLeadByID(id)
	if err != nil {
		http.Error(w, "Lead no encontrado: "+err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lead)
}

type UpdateLeadRequest struct {
	Status string `json:"status"`
	Notes  string `json:"notes"`
}

func (h *AdminHandler) UpdateLead(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID de lead inválido", http.StatusBadRequest)
		return
	}

	var req UpdateLeadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Cuerpo JSON inválido", http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{
		"nuevo":           true,
		"en_gestion":      true,
		"contactado":      true,
		"visita_agendada": true,
		"cerrado_ganado":  true,
		"descartado":      true,
		"archivado":       true,
	}

	status := strings.TrimSpace(req.Status)
	if !validStatuses[status] {
		status = "nuevo"
	}

	if err := h.db.UpdateLead(id, status, req.Notes); err != nil {
		http.Error(w, "Error al actualizar consulta: "+err.Error(), http.StatusInternalServerError)
		return
	}

	updated, err := h.db.GetLeadByID(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "id": id, "status": status, "notes": req.Notes})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

func (h *AdminHandler) CreateCar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Error al procesar formulario: "+err.Error(), http.StatusBadRequest)
		return
	}

	year, _ := strconv.Atoi(r.FormValue("year"))
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	downPayment, _ := strconv.ParseFloat(r.FormValue("down_payment"), 64)
	currency := strings.ToUpper(strings.TrimSpace(r.FormValue("currency")))
	if currency != "ARS" {
		currency = "USD"
	}
	mileage, _ := strconv.Atoi(r.FormValue("mileage"))
	doors, _ := strconv.Atoi(r.FormValue("doors"))
	featured := r.FormValue("featured") == "true" || r.FormValue("featured") == "on"

	var features []string
	featuresRaw := r.FormValue("features")
	for _, f := range strings.Split(featuresRaw, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			features = append(features, f)
		}
	}

	var images []string
	files := r.MultipartForm.File["images"]
	for _, fileHeader := range files {
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}

		file, err := fileHeader.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		filename := fmt.Sprintf("car_%d_%s%s", time.Now().UnixNano(), sanitizeFilename(fileHeader.Filename), ext)
		targetPath := filepath.Join(h.cfg.UploadDir, filename)

		dst, err := os.Create(targetPath)
		if err != nil {
			continue
		}
		_, _ = io.Copy(dst, file)
		dst.Close()

		images = append(images, "/uploads/"+filename)
	}

	if existingImgs := r.FormValue("existing_images"); existingImgs != "" {
		for _, img := range strings.Split(existingImgs, ",") {
			img = strings.TrimSpace(img)
			if img != "" {
				images = append(images, img)
			}
		}
	}

	if len(images) == 0 {
		images = []string{"/static/img/hero_showroom.jpg"}
	}

	car := &models.Car{
		Make:         r.FormValue("make"),
		Model:        r.FormValue("model"),
		Year:         year,
		Price:        price,
		Currency:     currency,
		DownPayment:  downPayment,
		Mileage:      mileage,
		FuelType:     r.FormValue("fuel_type"),
		Transmission: r.FormValue("transmission"),
		BodyType:     r.FormValue("body_type"),
		Color:        r.FormValue("color"),
		Doors:        doors,
		Engine:       r.FormValue("engine"),
		Description:  r.FormValue("description"),
		Features:     features,
		Images:       images,
		Status:       r.FormValue("status"),
		Featured:     featured,
	}

	if car.Status == "" {
		car.Status = "disponible"
	}

	if err := h.db.CreateCar(car); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(car)
}

func (h *AdminHandler) UpdateCar(w http.ResponseWriter, r *http.Request) {
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

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			Status string `json:"status"`
		}
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			if err := json.Unmarshal(bodyBytes, &req); err == nil && req.Status != "" && len(bodyBytes) < 80 {
				if err := h.db.UpdateCarStatus(id, req.Status); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status":     "success",
					"car_id":     id,
					"new_status": req.Status,
				})
				return
			}
			_ = json.Unmarshal(bodyBytes, car)
		}
	} else if err := r.ParseMultipartForm(32 << 20); err == nil {
		if val := r.FormValue("make"); val != "" {
			car.Make = val
		}
		if val := r.FormValue("model"); val != "" {
			car.Model = val
		}
		if val, err := strconv.Atoi(r.FormValue("year")); err == nil && val > 0 {
			car.Year = val
		}
		if val, err := strconv.ParseFloat(r.FormValue("price"), 64); err == nil && val > 0 {
			car.Price = val
		}
		if val := strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))); val == "ARS" || val == "USD" {
			car.Currency = val
		}
		if val, err := strconv.ParseFloat(r.FormValue("down_payment"), 64); err == nil {
			car.DownPayment = val
		}
		if val, err := strconv.Atoi(r.FormValue("mileage")); err == nil {
			car.Mileage = val
		}
		if val := r.FormValue("body_type"); val != "" {
			car.BodyType = val
		}
		if val := r.FormValue("fuel_type"); val != "" {
			car.FuelType = val
		}
		if val := r.FormValue("transmission"); val != "" {
			car.Transmission = val
		}
		if val := r.FormValue("color"); val != "" {
			car.Color = val
		}
		if val, err := strconv.Atoi(r.FormValue("doors")); err == nil {
			car.Doors = val
		}
		if val := r.FormValue("engine"); val != "" {
			car.Engine = val
		}
		if val := r.FormValue("description"); val != "" {
			car.Description = val
		}
		if featuresRaw := r.FormValue("features"); featuresRaw != "" {
			var feats []string
			for _, f := range strings.Split(featuresRaw, "\n") {
				f = strings.TrimSpace(f)
				if f != "" {
					feats = append(feats, f)
				}
			}
			car.Features = feats
		}
		if val := r.FormValue("status"); val != "" {
			car.Status = val
		}
		if r.FormValue("featured_set") != "" || r.FormValue("featured") != "" {
			car.Featured = r.FormValue("featured") == "true" || r.FormValue("featured") == "on"
		}

		if existingRaw := r.FormValue("existing_images"); existingRaw != "" {
			var kept []string
			trimmed := strings.TrimSpace(existingRaw)
			if strings.HasPrefix(trimmed, "[") {
				_ = json.Unmarshal([]byte(trimmed), &kept)
			} else {
				for _, img := range strings.Split(trimmed, ",") {
					img = strings.TrimSpace(img)
					if img != "" {
						kept = append(kept, img)
					}
				}
			}
			car.Images = kept
		}

		if r.MultipartForm != nil {
			files := r.MultipartForm.File["images"]
			for _, fileHeader := range files {
				ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
				if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
					continue
				}
				file, err := fileHeader.Open()
				if err != nil {
					continue
				}
				defer file.Close()

				filename := fmt.Sprintf("car_%d_%s%s", time.Now().UnixNano(), sanitizeFilename(fileHeader.Filename), ext)
				targetPath := filepath.Join(h.cfg.UploadDir, filename)

				dst, err := os.Create(targetPath)
				if err != nil {
					continue
				}
				_, _ = io.Copy(dst, file)
				dst.Close()

				car.Images = append(car.Images, "/uploads/"+filename)
			}
		}

		if len(car.Images) == 0 {
			car.Images = []string{"/static/img/hero_showroom.jpg"}
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(car); err != nil {
			http.Error(w, "Error al decodificar datos", http.StatusBadRequest)
			return
		}
	}

	car.ID = id
	if err := h.db.UpdateCar(car); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(car)
}

func (h *AdminHandler) DeleteCar(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("permanent") == "1" {
		car, err := h.db.GetCarByID(id)
		if err != nil {
			http.Error(w, "Vehículo no encontrado", http.StatusNotFound)
			return
		}
		if car.Status != "archivado" {
			http.Error(w, "Solo se pueden eliminar publicaciones archivadas", http.StatusConflict)
			return
		}
		if err := h.db.PurgeCar(id); err != nil {
			if errors.Is(err, database.ErrNotArchived) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Remove uploaded photos from disk (never touch bundled /static assets)
		for _, img := range car.Images {
			if strings.HasPrefix(img, "/uploads/") {
				_ = os.Remove(filepath.Join(h.cfg.UploadDir, filepath.Base(img)))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "purged"})
		return
	}

	if err := h.db.DeleteCar(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// DeleteLead permanently removes a lead. Only archived leads can be deleted.
func (h *AdminHandler) DeleteLead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID de lead inválido", http.StatusBadRequest)
		return
	}
	if err := h.db.DeleteLead(id); err != nil {
		if errors.Is(err, database.ErrNotArchived) {
			http.Error(w, "Solo se pueden eliminar consultas archivadas", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	if len(name) > 20 {
		name = name[:20]
	}
	return name
}

type UpdateSettingsRequest struct {
	WhatsAppNumber    string `json:"whatsapp_number"`
	GoogleAnalyticsID string `json:"google_analytics_id"`
	LookerStudioURL   string `json:"looker_studio_url"`
}

func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	gaID := h.db.GetGoogleAnalyticsID()
	if gaID == "" && h.cfg != nil {
		gaID = h.cfg.GAMeasurementID
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"whatsapp_number":     h.db.GetWhatsAppNumber(),
		"google_analytics_id": gaID,
		"looker_studio_url":   h.db.GetLookerStudioURL(),
	})
}

func (h *AdminHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Cuerpo inválido", http.StatusBadRequest)
		return
	}

	if req.WhatsAppNumber != "" {
		if err := h.db.SetWhatsAppNumber(req.WhatsAppNumber); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Google Analytics ID (can be updated or cleared)
	if err := h.db.SetGoogleAnalyticsID(req.GoogleAnalyticsID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Looker Studio URL (can be updated or cleared)
	if err := h.db.SetLookerStudioURL(req.LookerStudioURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	gaID := h.db.GetGoogleAnalyticsID()
	if gaID == "" && h.cfg != nil {
		gaID = h.cfg.GAMeasurementID
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":             true,
		"whatsapp_number":     h.db.GetWhatsAppNumber(),
		"google_analytics_id": gaID,
		"looker_studio_url":   h.db.GetLookerStudioURL(),
	})
}

