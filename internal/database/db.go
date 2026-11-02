package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"escocar-web/internal/models"
)

type DB struct {
	*sql.DB
}

func Connect(dbURL string) (*DB, error) {
	var db *sql.DB
	var err error

	// Retry connection for up to 30 seconds to allow Postgres container initialization
	maxRetries := 15
	for i := 1; i <= maxRetries; i++ {
		db, err = sql.Open("postgres", dbURL)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Printf("Successfully connected to PostgreSQL on attempt %d", i)
				break
			}
		}
		log.Printf("Waiting for database connection (attempt %d/%d): %v", i, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after retries: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Ensure all database queries run in Buenos Aires timezone (UTC-3)
	if _, err := db.Exec("SET timezone = 'America/Argentina/Buenos_Aires'"); err != nil {
		log.Printf("Warning: Failed to set PostgreSQL timezone: %v", err)
	}

	database := &DB{DB: db}
	if err := database.initSchema(); err != nil {
		return nil, fmt.Errorf("schema migration failed: %w", err)
	}

	if err := database.seedCars(); err != nil {
		log.Printf("Warning: Seeding cars failed: %v", err)
	}

	return database, nil
}

func (d *DB) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS cars (
		id SERIAL PRIMARY KEY,
		make VARCHAR(100) NOT NULL,
		model VARCHAR(100) NOT NULL,
		year INT NOT NULL,
		price NUMERIC(12,2) NOT NULL,
		currency VARCHAR(10) DEFAULT 'USD',
		mileage INT NOT NULL,
		fuel_type VARCHAR(50) NOT NULL,
		transmission VARCHAR(50) NOT NULL,
		body_type VARCHAR(50) NOT NULL,
		color VARCHAR(50) NOT NULL,
		doors INT DEFAULT 4,
		engine VARCHAR(100),
		description TEXT,
		features TEXT DEFAULT '[]',
		images TEXT DEFAULT '[]',
		status VARCHAR(50) DEFAULT 'disponible',
		featured BOOLEAN DEFAULT false,
		views_count INT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS visitors (
		id UUID PRIMARY KEY,
		first_seen_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		visit_count INT DEFAULT 1,
		user_agent TEXT,
		device_type VARCHAR(50) DEFAULT 'Desktop',
		referrer TEXT,
		ip_hash VARCHAR(64),
		notes TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS page_views (
		id BIGSERIAL PRIMARY KEY,
		visitor_id UUID NOT NULL REFERENCES visitors(id) ON DELETE CASCADE,
		session_id VARCHAR(64) NOT NULL,
		path TEXT NOT NULL,
		car_id INT REFERENCES cars(id) ON DELETE SET NULL,
		duration_seconds INT DEFAULT 0,
		referrer TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS traffic_events (
		id BIGSERIAL PRIMARY KEY,
		visitor_id UUID NOT NULL REFERENCES visitors(id) ON DELETE CASCADE,
		session_id VARCHAR(64) NOT NULL,
		car_id INT REFERENCES cars(id) ON DELETE SET NULL,
		event_type VARCHAR(50) NOT NULL,
		event_data TEXT DEFAULT '{}',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS leads (
		id SERIAL PRIMARY KEY,
		visitor_id UUID REFERENCES visitors(id) ON DELETE SET NULL,
		car_id INT REFERENCES cars(id) ON DELETE SET NULL,
		name VARCHAR(100) NOT NULL,
		phone VARCHAR(50) NOT NULL,
		email VARCHAR(100),
		message TEXT,
		status VARCHAR(50) DEFAULT 'nuevo',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS admin_sessions (
		id SERIAL PRIMARY KEY,
		token VARCHAR(255) UNIQUE NOT NULL,
		email VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		picture TEXT DEFAULT '',
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS site_settings (
		key VARCHAR(100) PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	INSERT INTO site_settings (key, value) VALUES ('whatsapp_number', '5491100000000') ON CONFLICT (key) DO NOTHING;
	INSERT INTO site_settings (key, value) VALUES ('google_analytics_id', '') ON CONFLICT (key) DO NOTHING;
	INSERT INTO site_settings (key, value) VALUES ('looker_studio_url', '') ON CONFLICT (key) DO NOTHING;

	ALTER TABLE cars ADD COLUMN IF NOT EXISTS down_payment NUMERIC(12,2) DEFAULT 0;
	ALTER TABLE cars ADD COLUMN IF NOT EXISTS featured_order INT DEFAULT 0;
	ALTER TABLE admin_sessions ALTER COLUMN token TYPE VARCHAR(255);

	ALTER TABLE leads ADD COLUMN IF NOT EXISTS notes TEXT DEFAULT '';
	ALTER TABLE leads ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

	ALTER TABLE visitors ADD COLUMN IF NOT EXISTS name VARCHAR(100) DEFAULT '';
	ALTER TABLE visitors ADD COLUMN IF NOT EXISTS phone VARCHAR(50) DEFAULT '';
	ALTER TABLE visitors ADD COLUMN IF NOT EXISTS email VARCHAR(100) DEFAULT '';

	UPDATE visitors v
	SET name = l.name, phone = l.phone, email = l.email
	FROM (
		SELECT DISTINCT ON (visitor_id) visitor_id, name, phone, email
		FROM leads
		WHERE visitor_id IS NOT NULL AND name != ''
		ORDER BY visitor_id, created_at DESC
	) l
	WHERE v.id = l.visitor_id AND (v.name IS NULL OR v.name = '');

	CREATE INDEX IF NOT EXISTS idx_cars_status ON cars(status);
	CREATE INDEX IF NOT EXISTS idx_cars_featured ON cars(featured);
	CREATE INDEX IF NOT EXISTS idx_cars_featured_order ON cars(featured, featured_order);
	CREATE INDEX IF NOT EXISTS idx_leads_status ON leads(status);
	CREATE INDEX IF NOT EXISTS idx_leads_created ON leads(created_at);
	CREATE INDEX IF NOT EXISTS idx_page_views_visitor ON page_views(visitor_id);
	CREATE INDEX IF NOT EXISTS idx_page_views_created ON page_views(created_at);
	CREATE INDEX IF NOT EXISTS idx_visitors_last_seen ON visitors(last_seen_at);
	CREATE INDEX IF NOT EXISTS idx_visitors_name ON visitors(name);
	CREATE INDEX IF NOT EXISTS idx_admin_sessions_token ON admin_sessions(token);
	`
	_, err := d.Exec(query)
	return err
}

func (d *DB) CreateAdminSession(token, email, name, picture string, expiresAt time.Time) error {
	query := `
		INSERT INTO admin_sessions (token, email, name, picture, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`
	_, err := d.Exec(query, token, email, name, picture, expiresAt)
	return err
}

func (d *DB) GetAdminSession(token string) (*models.AdminSession, error) {
	query := `
		SELECT id, token, email, name, picture, expires_at, created_at
		FROM admin_sessions
		WHERE token = $1 AND expires_at > NOW()
	`
	var s models.AdminSession
	err := d.QueryRow(query, token).Scan(
		&s.ID, &s.Token, &s.Email, &s.Name, &s.Picture, &s.ExpiresAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *DB) DeleteAdminSession(token string) error {
	_, err := d.Exec("DELETE FROM admin_sessions WHERE token = $1", token)
	return err
}

func (d *DB) seedCars() error {
	var count int
	err := d.QueryRow("SELECT COUNT(*) FROM cars").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	initialCars := []models.Car{
		{
			Make:         "BMW",
			Model:        "M4 Competition Coupé",
			Year:         2023,
			Price:        138000,
			Currency:     "USD",
			DownPayment:  40000,
			Mileage:      9500,
			FuelType:     "Nafta",
			Transmission: "Automática",
			BodyType:     "Coupé",
			Color:        "Verde Isle of Man",
			Doors:        2,
			Engine:       "3.0L M TwinPower Turbo 510 CV",
			Description:  "Impecable estado. Único dueño. M Carbon exterior package, frenos cerámicos M Carbon, escape deportivo con válvulas, tapizado en cuero Merino y audio Harman Kardon. Service al día.",
			Features: []string{
				"Tracción trasera M xDrive desconectable",
				"Head-Up Display M con modo pista",
				"Llantas forjadas M 19''/20''",
				"Faros láser BMW Laserlight",
				"Apple CarPlay y Android Auto inalámbrico",
				"Documentación al día y lista para transferir",
			},
			Images:     []string{"/uploads/car_bmw_m4.jpg"},
			Status:     "disponible",
			Featured:   true,
			ViewsCount: 42,
		},
		{
			Make:         "Audi",
			Model:        "RS Q8 Quattro",
			Year:         2024,
			Price:        195000,
			Currency:     "USD",
			DownPayment:  60000,
			Mileage:      4200,
			FuelType:     "Híbrido",
			Transmission: "Automática",
			BodyType:     "SUV",
			Color:        "Gris Nardo",
			Doors:        5,
			Engine:       "4.0L V8 Biturbo Mild-Hybrid 600 CV",
			Description:  "El SUV deportivo más exclusivo de Audi. Paquete Black Optics exterior, suspensión neumática adaptativa deportiva, eje trasero direccional y sistema de escape deportivo RS.",
			Features: []string{
				"Tracción integral quattro con diferencial deportivo",
				"Suspensión neumática RS con altura variable",
				"Audi Virtual Cockpit Plus con vistas RS",
				"Audio Bang & Olufsen 3D Advanced 1920W",
				"Cámaras de entorno 360° en 3D",
				"Techo panorámico corredizo eléctrico",
			},
			Images:     []string{"/uploads/car_audi_q8.jpg"},
			Status:     "disponible",
			Featured:   true,
			ViewsCount: 78,
		},
		{
			Make:         "Porsche",
			Model:        "Taycan 4S",
			Year:         2023,
			Price:        165000,
			Currency:     "USD",
			DownPayment:  50000,
			Mileage:      14000,
			FuelType:     "Eléctrico",
			Transmission: "Automática",
			BodyType:     "Sedán",
			Color:        "Azul Frozen Metálico",
			Doors:        4,
			Engine:       "Batería Performance Plus 530 CV",
			Description:  "100% Eléctrico. Autonomía superior a 460 km (WLTP). Carga ultrarrápida 800V de 5% a 80% en 22 minutos. Interior en cuero bi-tono negro/tiza con techo panorámico de cristal atérmico.",
			Features: []string{
				"Tracción total eléctrica con Porsche Traction Management",
				"Porsche Active Suspension Management (PASM)",
				"Porsche Electric Sport Sound",
				"Pantalla táctil para acompañante de 10.9 pulgadas",
				"Cargador de a bordo de 22 kW CA",
				"Peritaje técnico oficial de batería al día",
			},
			Images:     []string{"/uploads/car_porsche_taycan.jpg"},
			Status:     "disponible",
			Featured:   true,
			ViewsCount: 65,
		},
	}

	for _, car := range initialCars {
		if err := d.CreateCar(&car); err != nil {
			return err
		}
	}
	log.Printf("Successfully seeded %d initial luxury showcase cars", len(initialCars))
	return nil
}

// Car queries

func (d *DB) GetCars(filter models.CarFilter) ([]models.Car, error) {
	query := `
		SELECT id, make, model, year, price, currency, COALESCE(down_payment, 0), mileage, fuel_type, 
		       transmission, body_type, color, doors, engine, description, 
		       features, images, status, featured, COALESCE(featured_order, 0), views_count, created_at, updated_at
		FROM cars
		WHERE 1=1
	`
	var args []interface{}
	idx := 1

	if filter.Query != "" {
		query += fmt.Sprintf(" AND (LOWER(make) LIKE $%d OR LOWER(model) LIKE $%d OR LOWER(description) LIKE $%d)", idx, idx, idx)
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		idx++
	}

	if filter.Make != "" {
		query += fmt.Sprintf(" AND LOWER(TRIM(make)) = LOWER(TRIM($%d))", idx)
		args = append(args, filter.Make)
		idx++
	}

	if filter.BodyType != "" {
		query += fmt.Sprintf(" AND TRANSLATE(LOWER(body_type), 'áéíóú', 'aeiou') = TRANSLATE(LOWER($%d), 'áéíóú', 'aeiou')", idx)
		args = append(args, filter.BodyType)
		idx++
	}

	if filter.FuelType != "" {
		query += fmt.Sprintf(" AND LOWER(fuel_type) = LOWER($%d)", idx)
		args = append(args, filter.FuelType)
		idx++
	}

	if filter.Transmission != "" {
		query += fmt.Sprintf(" AND LOWER(transmission) = LOWER($%d)", idx)
		args = append(args, filter.Transmission)
		idx++
	}

	if filter.MinYear > 0 {
		query += fmt.Sprintf(" AND year >= $%d", idx)
		args = append(args, filter.MinYear)
		idx++
	}

	if filter.MaxYear > 0 {
		query += fmt.Sprintf(" AND year <= $%d", idx)
		args = append(args, filter.MaxYear)
		idx++
	}

	if filter.MinPrice > 0 {
		query += fmt.Sprintf(" AND price >= $%d", idx)
		args = append(args, filter.MinPrice)
		idx++
	}

	if filter.MaxPrice > 0 {
		query += fmt.Sprintf(" AND price <= $%d", idx)
		args = append(args, filter.MaxPrice)
		idx++
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, filter.Status)
		idx++
	} else if !filter.IncludeArchived {
		query += " AND status != 'archivado'"
	}

	if filter.FeaturedOnly {
		query += " AND featured = true"
	}

	switch filter.SortBy {
	case "price_asc":
		query += " ORDER BY price ASC"
	case "price_desc":
		query += " ORDER BY price DESC"
	case "year_desc":
		query += " ORDER BY year DESC, id DESC"
	case "mileage_asc":
		query += " ORDER BY mileage ASC"
	default:
		query += " ORDER BY featured DESC, CASE WHEN COALESCE(featured_order, 0) > 0 THEN featured_order ELSE 999999 END ASC, id DESC"
	}

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cars []models.Car
	for rows.Next() {
		var c models.Car
		var featuresStr, imagesStr string
		err := rows.Scan(
			&c.ID, &c.Make, &c.Model, &c.Year, &c.Price, &c.Currency, &c.DownPayment,
			&c.Mileage, &c.FuelType, &c.Transmission, &c.BodyType, &c.Color,
			&c.Doors, &c.Engine, &c.Description, &featuresStr, &imagesStr,
			&c.Status, &c.Featured, &c.FeaturedOrder, &c.ViewsCount, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		_ = json.Unmarshal([]byte(featuresStr), &c.Features)
		_ = json.Unmarshal([]byte(imagesStr), &c.Images)
		cars = append(cars, c)
	}

	return cars, nil
}

func (d *DB) GetCarByID(id int) (*models.Car, error) {
	query := `
		SELECT id, make, model, year, price, currency, COALESCE(down_payment, 0), mileage, fuel_type, 
		       transmission, body_type, color, doors, engine, description, 
		       features, images, status, featured, COALESCE(featured_order, 0), views_count, created_at, updated_at
		FROM cars
		WHERE id = $1
	`
	var c models.Car
	var featuresStr, imagesStr string
	err := d.QueryRow(query, id).Scan(
		&c.ID, &c.Make, &c.Model, &c.Year, &c.Price, &c.Currency, &c.DownPayment,
		&c.Mileage, &c.FuelType, &c.Transmission, &c.BodyType, &c.Color,
		&c.Doors, &c.Engine, &c.Description, &featuresStr, &imagesStr,
		&c.Status, &c.Featured, &c.FeaturedOrder, &c.ViewsCount, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(featuresStr), &c.Features)
	_ = json.Unmarshal([]byte(imagesStr), &c.Images)
	return &c, nil
}

func (d *DB) CreateCar(c *models.Car) error {
	if c.Featured && c.FeaturedOrder == 0 {
		_ = d.QueryRow("SELECT COALESCE(MAX(featured_order), 0) + 1 FROM cars WHERE featured = true").Scan(&c.FeaturedOrder)
	}
	query := `
		INSERT INTO cars (make, model, year, price, currency, down_payment, mileage, fuel_type, 
		                 transmission, body_type, color, doors, engine, description, 
		                 features, images, status, featured, featured_order, views_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id, created_at, updated_at
	`
	featuresJSON := c.FeaturesJSON()
	imagesJSON := c.ImagesJSON()

	return d.QueryRow(
		query,
		c.Make, c.Model, c.Year, c.Price, c.Currency, c.DownPayment, c.Mileage, c.FuelType,
		c.Transmission, c.BodyType, c.Color, c.Doors, c.Engine, c.Description,
		featuresJSON, imagesJSON, c.Status, c.Featured, c.FeaturedOrder, c.ViewsCount,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (d *DB) UpdateCar(c *models.Car) error {
	if c.Featured && c.FeaturedOrder == 0 {
		_ = d.QueryRow("SELECT COALESCE(MAX(featured_order), 0) + 1 FROM cars WHERE featured = true AND id != $1", c.ID).Scan(&c.FeaturedOrder)
	}
	query := `
		UPDATE cars SET 
			make = $1, model = $2, year = $3, price = $4, currency = $5, down_payment = $6,
			mileage = $7, fuel_type = $8, transmission = $9, body_type = $10,
			color = $11, doors = $12, engine = $13, description = $14,
			features = $15, images = $16, status = $17, featured = $18,
			featured_order = $19,
			updated_at = NOW()
		WHERE id = $20
	`
	featuresJSON := c.FeaturesJSON()
	imagesJSON := c.ImagesJSON()

	_, err := d.Exec(
		query,
		c.Make, c.Model, c.Year, c.Price, c.Currency, c.DownPayment, c.Mileage, c.FuelType,
		c.Transmission, c.BodyType, c.Color, c.Doors, c.Engine, c.Description,
		featuresJSON, imagesJSON, c.Status, c.Featured, c.FeaturedOrder, c.ID,
	)
	return err
}

func (d *DB) UpdateCarStatus(id int, status string) error {
	_, err := d.Exec("UPDATE cars SET status = $1, updated_at = NOW() WHERE id = $2", status, id)
	return err
}

func (d *DB) UpdateCarFeatured(id int, featured bool) error {
	if featured {
		_, err := d.Exec(`
			UPDATE cars
			SET featured = true,
			    featured_order = CASE WHEN COALESCE(featured_order, 0) > 0 THEN featured_order ELSE (SELECT COALESCE(MAX(featured_order), 0) + 1 FROM cars WHERE featured = true) END,
			    updated_at = NOW()
			WHERE id = $1
		`, id)
		return err
	}
	_, err := d.Exec("UPDATE cars SET featured = false, featured_order = 0, updated_at = NOW() WHERE id = $1", id)
	return err
}

func (d *DB) UpdateFeaturedOrder(carIDs []int) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range carIDs {
		order := i + 1
		if _, err := tx.Exec("UPDATE cars SET featured_order = $1, featured = true, updated_at = NOW() WHERE id = $2", order, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) DeleteCar(id int) error {
	// Soft delete / archive publication so it remains accessible only to administrators
	_, err := d.Exec("UPDATE cars SET status = 'archivado', featured = false, updated_at = NOW() WHERE id = $1", id)
	return err
}

// ErrNotArchived is returned when trying to permanently delete a record that is not archived.
var ErrNotArchived = fmt.Errorf("el registro debe estar archivado antes de eliminarse")

// PurgeCar permanently deletes an archived car. Related leads/page views keep their rows
// (car_id is set to NULL by the FK constraints).
func (d *DB) PurgeCar(id int) error {
	res, err := d.Exec("DELETE FROM cars WHERE id = $1 AND status = 'archivado'", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotArchived
	}
	return nil
}

// GetAvailableMakes returns the distinct brands currently in (non-archived) stock.
func (d *DB) GetAvailableMakes() ([]string, error) {
	rows, err := d.Query(`
		SELECT MIN(TRIM(make)) AS make
		FROM cars
		WHERE status != 'archivado' AND TRIM(make) != ''
		GROUP BY LOWER(TRIM(make))
		ORDER BY LOWER(MIN(TRIM(make)))
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var makes []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err == nil {
			makes = append(makes, m)
		}
	}
	return makes, rows.Err()
}

func (d *DB) IncrementCarViews(id int) error {
	_, err := d.Exec("UPDATE cars SET views_count = views_count + 1 WHERE id = $1", id)
	return err
}

// Visitor & Traffic Tracking methods

func (d *DB) UpsertVisitor(v *models.Visitor, isNewSession bool) (isReturning bool, err error) {
	// First check if visitor exists
	var currentVisits int
	err = d.QueryRow("SELECT visit_count FROM visitors WHERE id = $1", v.ID).Scan(&currentVisits)
	if err == sql.ErrNoRows {
		// New visitor
		insertQuery := `
			INSERT INTO visitors (id, first_seen_at, last_seen_at, visit_count, user_agent, device_type, referrer, ip_hash)
			VALUES ($1, NOW(), NOW(), 1, $2, $3, $4, $5)
		`
		_, err = d.Exec(insertQuery, v.ID, v.UserAgent, v.DeviceType, v.Referrer, v.IPHash)
		return false, err
	} else if err != nil {
		return false, err
	}

	// Returning visitor: update last seen. Only increment visit_count if starting a new session (30m inactivity)
	var updateQuery string
	if isNewSession {
		updateQuery = `
			UPDATE visitors SET 
				last_seen_at = NOW(),
				visit_count = visit_count + 1,
				user_agent = COALESCE(NULLIF($2, ''), user_agent),
				device_type = COALESCE(NULLIF($3, ''), device_type),
				referrer = COALESCE(NULLIF($4, ''), referrer)
			WHERE id = $1
		`
	} else {
		updateQuery = `
			UPDATE visitors SET 
				last_seen_at = NOW(),
				user_agent = COALESCE(NULLIF($2, ''), user_agent),
				device_type = COALESCE(NULLIF($3, ''), device_type),
				referrer = COALESCE(NULLIF($4, ''), referrer)
			WHERE id = $1
		`
	}
	_, err = d.Exec(updateQuery, v.ID, v.UserAgent, v.DeviceType, v.Referrer)
	return true, err
}

func (d *DB) RecordPageView(pv *models.PageView) error {
	query := `
		INSERT INTO page_views (visitor_id, session_id, path, car_id, duration_seconds, referrer, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`
	_, err := d.Exec(query, pv.VisitorID, pv.SessionID, pv.Path, pv.CarID, pv.DurationSeconds, pv.Referrer)
	return err
}

func (d *DB) RecordEvent(e *models.TrafficEvent) error {
	query := `
		INSERT INTO traffic_events (visitor_id, session_id, car_id, event_type, event_data, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`
	_, err := d.Exec(query, e.VisitorID, e.SessionID, e.CarID, e.EventType, e.EventData)
	return err
}

func (d *DB) RecordLead(l *models.Lead) error {
	query := `
		INSERT INTO leads (visitor_id, car_id, name, phone, email, message, status, notes, updated_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'nuevo', '', NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	err := d.QueryRow(query, l.VisitorID, l.CarID, l.Name, l.Phone, l.Email, l.Message).Scan(&l.ID, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return err
	}

	// Permanently link visitor cookie to name, phone, and email
	if l.VisitorID != "" {
		_, _ = d.Exec(`
			UPDATE visitors 
			SET name = $1, phone = $2, email = $3,
			    notes = CASE 
			        WHEN notes IS NULL OR notes = '' THEN 'Lead registrado: ' || $1
			        ELSE notes 
			    END
			WHERE id = $4
		`, l.Name, l.Phone, l.Email, l.VisitorID)
	}

	return nil
}

func (d *DB) GetLeads() ([]models.Lead, error) {
	query := `
		SELECT l.id, COALESCE(l.visitor_id::text, ''), l.car_id, 
		       COALESCE(c.make || ' ' || c.model, 'Consulta General') as car_title,
		       l.name, l.phone, COALESCE(l.email, ''), COALESCE(l.message, ''), 
		       COALESCE(l.status, 'nuevo'), COALESCE(l.notes, ''), l.created_at, COALESCE(l.updated_at, l.created_at)
		FROM leads l
		LEFT JOIN cars c ON l.car_id = c.id
		ORDER BY l.created_at DESC
		LIMIT 200
	`
	rows, err := d.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leads []models.Lead
	for rows.Next() {
		var lead models.Lead
		err := rows.Scan(
			&lead.ID, &lead.VisitorID, &lead.CarID, &lead.CarTitle,
			&lead.Name, &lead.Phone, &lead.Email, &lead.Message,
			&lead.Status, &lead.Notes, &lead.CreatedAt, &lead.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		leads = append(leads, lead)
	}
	return leads, nil
}

func (d *DB) GetLeadByID(id int) (*models.Lead, error) {
	query := `
		SELECT l.id, COALESCE(l.visitor_id::text, ''), l.car_id, 
		       COALESCE(c.make || ' ' || c.model, 'Consulta General') as car_title,
		       l.name, l.phone, COALESCE(l.email, ''), COALESCE(l.message, ''), 
		       COALESCE(l.status, 'nuevo'), COALESCE(l.notes, ''), l.created_at, COALESCE(l.updated_at, l.created_at)
		FROM leads l
		LEFT JOIN cars c ON l.car_id = c.id
		WHERE l.id = $1
	`
	var lead models.Lead
	err := d.QueryRow(query, id).Scan(
		&lead.ID, &lead.VisitorID, &lead.CarID, &lead.CarTitle,
		&lead.Name, &lead.Phone, &lead.Email, &lead.Message,
		&lead.Status, &lead.Notes, &lead.CreatedAt, &lead.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Enrich with visitor browsing insights if visitor_id is known
	if lead.VisitorID != "" {
		_ = d.QueryRow(`
			SELECT COALESCE(visit_count, 1), COALESCE(device_type, 'desktop')
			FROM visitors WHERE id = $1
		`, lead.VisitorID).Scan(&lead.VisitorVisits, &lead.VisitorDevice)

		carRows, err := d.Query(`
			SELECT DISTINCT c.make || ' ' || c.model || ' (' || c.year || ')'
			FROM (
				SELECT car_id FROM tracking_events WHERE visitor_id = $1 AND car_id IS NOT NULL
				UNION
				SELECT car_id FROM page_views WHERE visitor_id = $1 AND car_id IS NOT NULL
			) v_cars
			JOIN cars c ON v_cars.car_id = c.id
			LIMIT 5
		`, lead.VisitorID)
		if err == nil {
			defer carRows.Close()
			for carRows.Next() {
				var carName string
				if err := carRows.Scan(&carName); err == nil {
					lead.VisitorCarsSeen = append(lead.VisitorCarsSeen, carName)
				}
			}
		}
	}

	return &lead, nil
}

func (d *DB) UpdateLead(id int, status string, notes string) error {
	query := `
		UPDATE leads
		SET status = $1, notes = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := d.Exec(query, status, notes, id)
	return err
}

// DeleteLead permanently deletes an archived lead.
func (d *DB) DeleteLead(id int) error {
	res, err := d.Exec("DELETE FROM leads WHERE id = $1 AND status = 'archivado'", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotArchived
	}
	return nil
}

func (d *DB) GetLatestLeadByVisitorID(visitorID string) (*models.Lead, error) {
	if visitorID == "" {
		return nil, sql.ErrNoRows
	}
	query := `
		SELECT id, visitor_id, car_id, name, phone, COALESCE(email, ''), COALESCE(message, ''), 
		       COALESCE(status, 'nuevo'), COALESCE(notes, ''), created_at, COALESCE(updated_at, created_at)
		FROM leads
		WHERE visitor_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var lead models.Lead
	err := d.QueryRow(query, visitorID).Scan(
		&lead.ID, &lead.VisitorID, &lead.CarID,
		&lead.Name, &lead.Phone, &lead.Email, &lead.Message,
		&lead.Status, &lead.Notes, &lead.CreatedAt, &lead.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &lead, nil
}

func (d *DB) GetAnalyticsSummary() (*models.AnalyticsSummary, error) {
	summary := &models.AnalyticsSummary{
		DeviceBreakdown: make(map[string]int),
	}

	// Total visitors & returning visitors (excluding automated bots)
	row := d.QueryRow(`
		SELECT 
			COUNT(*),
			COUNT(CASE WHEN visit_count > 1 THEN 1 END),
			COUNT(CASE WHEN last_seen_at >= CURRENT_DATE THEN 1 END)
		FROM visitors
		WHERE device_type != 'Bot'
	`)
	err := row.Scan(&summary.TotalVisitors, &summary.ReturningVisitors, &summary.TodayVisitors)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if summary.TotalVisitors > 0 {
		summary.ReturnRate = (float64(summary.ReturningVisitors) / float64(summary.TotalVisitors)) * 100.0
	}

	// Pageviews (excluding bots, API, and admin traffic)
	rowPV := d.QueryRow(`
		SELECT 
			COUNT(*),
			COUNT(CASE WHEN pv.created_at >= CURRENT_DATE THEN 1 END)
		FROM page_views pv
		LEFT JOIN visitors v ON pv.visitor_id = v.id
		WHERE (v.device_type IS NULL OR v.device_type != 'Bot')
		  AND pv.path NOT LIKE '/api/%'
		  AND pv.path NOT LIKE '/admin%'
		  AND pv.path NOT LIKE '/auth/%'
		  AND pv.path NOT LIKE '%.php%'
	`)
	_ = rowPV.Scan(&summary.TotalPageViews, &summary.TodayPageViews)

	// Top viewed cars
	rowsCars, err := d.Query(`
		SELECT c.id, c.make, c.model, c.year, c.price, c.views_count,
		       COUNT(DISTINCT pv.visitor_id) as unique_users
		FROM cars c
		LEFT JOIN page_views pv ON c.id = pv.car_id
		GROUP BY c.id
		ORDER BY c.views_count DESC, unique_users DESC
		LIMIT 5
	`)
	if err == nil {
		defer rowsCars.Close()
		for rowsCars.Next() {
			var m models.TopCarMetric
			var price float64
			if err := rowsCars.Scan(&m.CarID, &m.Make, &m.Model, &m.Year, &price, &m.ViewsCount, &m.UniqueUsers); err == nil {
				m.Price = fmt.Sprintf("USD %d", int(price))
				summary.TopCars = append(summary.TopCars, m)
			}
		}
	}

	// Device breakdown (excluding Bot)
	rowsDev, err := d.Query(`
		SELECT COALESCE(device_type, 'Desktop'), COUNT(*)
		FROM visitors
		WHERE device_type != 'Bot'
		GROUP BY device_type
	`)
	if err == nil {
		defer rowsDev.Close()
		for rowsDev.Next() {
			var dev string
			var count int
			if err := rowsDev.Scan(&dev, &count); err == nil {
				summary.DeviceBreakdown[dev] = count
			}
		}
	}

	// Recent visitors (prioritizing identified contacts, excluding bots)
	rowsVis, err := d.Query(`
		SELECT v.id, v.first_seen_at, v.last_seen_at, v.visit_count,
		       v.device_type, COALESCE(v.referrer, ''),
		       (SELECT COUNT(*) FROM page_views WHERE visitor_id = v.id AND path NOT LIKE '/api/%' AND path NOT LIKE '/auth/%' AND path NOT LIKE '/admin%') as pv_count,
		       COALESCE(NULLIF(v.name, ''), (SELECT name FROM leads WHERE visitor_id = v.id AND name != '' ORDER BY created_at DESC LIMIT 1), '') as name,
		       COALESCE(NULLIF(v.phone, ''), (SELECT phone FROM leads WHERE visitor_id = v.id AND phone != '' ORDER BY created_at DESC LIMIT 1), '') as phone,
		       COALESCE(NULLIF(v.email, ''), (SELECT email FROM leads WHERE visitor_id = v.id AND email != '' ORDER BY created_at DESC LIMIT 1), '') as email,
		       COALESCE((
		           SELECT string_agg(DISTINCT c.make || ' ' || c.model, ', ')
		           FROM page_views pv
		           JOIN cars c ON pv.car_id = c.id
		           WHERE pv.visitor_id = v.id
		       ), '') as cars_viewed_str
		FROM visitors v
		WHERE v.device_type != 'Bot'
		ORDER BY (CASE WHEN COALESCE(NULLIF(v.name, ''), (SELECT name FROM leads WHERE visitor_id = v.id AND name != '' LIMIT 1), '') != '' THEN 0 ELSE 1 END), v.last_seen_at DESC
		LIMIT 30
	`)
	if err == nil {
		defer rowsVis.Close()
		for rowsVis.Next() {
			var v models.Visitor
			if err := rowsVis.Scan(&v.ID, &v.FirstSeenAt, &v.LastSeenAt, &v.VisitCount, &v.DeviceType, &v.Referrer, &v.TotalPageViews, &v.Name, &v.Phone, &v.Email, &v.CarsViewedStr); err == nil {
				summary.RecentVisitors = append(summary.RecentVisitors, v)
			}
		}
	}

	return summary, nil
}

func HashIP(ip string) string {
	h := sha256.Sum256([]byte(ip + "-escocar-salt"))
	return hex.EncodeToString(h[:16])
}

func (d *DB) GetSetting(key, defaultValue string) string {
	var val string
	err := d.QueryRow("SELECT value FROM site_settings WHERE key = $1", key).Scan(&val)
	if err != nil || strings.TrimSpace(val) == "" {
		return defaultValue
	}
	return val
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.Exec(`
		INSERT INTO site_settings (key, value, updated_at) 
		VALUES ($1, $2, NOW()) 
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, key, value)
	return err
}

func (d *DB) GetWhatsAppNumber() string {
	return d.GetSetting("whatsapp_number", "5491100000000")
}

func (d *DB) SetWhatsAppNumber(number string) error {
	clean := ""
	for _, r := range number {
		if r >= '0' && r <= '9' {
			clean += string(r)
		}
	}
	if clean == "" {
		return fmt.Errorf("el número de WhatsApp no puede estar vacío")
	}
	return d.SetSetting("whatsapp_number", clean)
}

func (d *DB) GetGoogleAnalyticsID() string {
	return d.GetSetting("google_analytics_id", "")
}

func (d *DB) SetGoogleAnalyticsID(id string) error {
	clean := strings.TrimSpace(id)
	return d.SetSetting("google_analytics_id", clean)
}

func (d *DB) GetLookerStudioURL() string {
	return d.GetSetting("looker_studio_url", "")
}

func (d *DB) SetLookerStudioURL(urlStr string) error {
	clean := strings.TrimSpace(urlStr)
	return d.SetSetting("looker_studio_url", clean)
}

