# 🚗 Escocar Autos - Plataforma Web Concesionaria

Plataforma web de alto rendimiento para concesionaria de automóviles, desarrollada en **Golang**, empaquetada con **Podman y Podman-Compose**, con persistencia en **PostgreSQL**, volumen montado para imágenes y **motor de recopilación de tráfico y seguimiento de visitantes anónimos recurrentes** mediante cookies.

---

## 🌟 Características Principales

1. **Backend Ultrarrápido en Go**:
   - Router `chi` ligero y de latencia mínima.
   - Conexión a PostgreSQL con pool de conexiones y migraciones automáticas.
   - Datos iniciales (*seed data*) con vehículos de alta gama precargados.

2. **Volumen Montado para Fotos (`/app/uploads`)**:
   - Volumen bind-mount `./uploads:/app/uploads:Z` compatible con SELinux en Podman.
   - Carga de imágenes en panel de administración y entrega estática con caché.

3. **Recopilador de Tráfico y Cookies de Retorno**:
   - Cookie de visitante `escocar_vid` (UUIDv4) con duración de 1 año.
   - Cookie de sesión `escocar_sid` (30 minutos de inactividad).
   - Identificación de clientes **nuevos vs recurrentes** (% de fidelidad).
   - Registro de páginas vistas, tiempo en página y vehículos de mayor interés.
   - Eventos de interacción: clics a WhatsApp, simulador de crédito y consultas.

4. **Experiencia de Usuario (UI/UX) de Alta Gama**:
   - Tema oscuro inspirado en marcas premium de automóviles.
   - Catálogo interactivo con filtros por marca, combustible, carrocería, transmisión y rango de precio.
   - Ficha detallada de vehículo con visor de galería y simulador de financiación en tiempo real.
   - Integración directa con WhatsApp para recepción inmediata de leads.

5. **Panel de Control Administrativo (`/admin`)**:
   - Métricas y KPIs en vivo (Visitantes únicos, recurrentes, páginas vistas, desglose de dispositivos).
   - Registro en tiempo real de visitantes anónimos y sus sesiones.
   - Gestión completa de inventario: agregar autos con fotos, editar estado (disponible, reservado, vendido) y eliminar.
   - Bandeja de leads y consultas recibidas.

---

## 🚀 Despliegue en VM Cloud con Podman

### 1. Clonar o transferir el repositorio a la VM:
```bash
git clone <url-del-repo> escocar-web
cd escocar-web
```

### 2. Configurar variables de entorno (opcional):
```bash
cp .env.example .env
# Modificar contraseñas si se desea
```

### 3. Levantar los servicios con Podman Compose:
```bash
podman-compose up -d --build
```
*(También podés utilizar `make up`)*

### 4. Verificar estado de los contenedores:
```bash
podman ps
podman-compose logs -f
```

La web estará disponible en:
- **Sitio Público**: `http://TU_IP_O_DOMINIO:8080/`
- **Catálogo**: `http://TU_IP_O_DOMINIO:8080/catalogo`
- **Panel Administrativo**: `http://TU_IP_O_DOMINIO:8080/admin`
  - **Usuario por defecto**: `admin`
  - **Contraseña por defecto**: `escocar2026`

---

## 🛠️ Desarrollo Local sin Contenedores

Si tenés PostgreSQL corriendo localmente:
```bash
export DATABASE_URL="postgres://escocar:escocar_secure_password_2026@localhost:5432/escocardb?sslmode=disable"
export UPLOAD_DIR="./uploads"
go run ./cmd/server
```

---

## 📁 Estructura del Proyecto

```
escocar-web/
├── cmd/server/main.go            # Punto de entrada HTTP y enrutamiento
├── internal/
│   ├── config/config.go          # Configuración y variables de entorno
│   ├── database/db.go            # Conexión PG, migraciones y seed data
│   ├── models/car.go             # Modelos de vehículos y filtros
│   ├── models/tracker.go         # Modelos de analíticas y visitantes
│   ├── handlers/cars.go          # API de vehículos
│   ├── handlers/tracker.go       # API de telemetría y leads
│   ├── handlers/admin.go         # API y controlador del panel admin
│   ├── handlers/pages.go         # Renderizador de plantillas HTML
│   └── middleware/tracker.go     # Middleware de cookies y métricas
├── web/
│   ├── static/
│   │   ├── css/style.css         # Estilos principales de diseño
│   │   ├── css/admin.css         # Estilos del dashboard de analíticas
│   │   ├── js/app.js             # Lógica de simulador y modales
│   │   ├── js/tracker.js         # Agente de rastreo de cliente
│   │   └── js/admin.js           # Lógica del panel administrativo
│   └── templates/
│       ├── base.html             # Layout base y footer
│       ├── index.html            # Portada y presentación
│       ├── catalog.html          # Catálogo con filtros avanzados
│       ├── detail.html           # Ficha y simulador financiero
│       └── admin.html            # Dashboard de métricas e inventario
├── uploads/                      # Volumen montado para fotos de autos
├── Containerfile                 # Multi-stage build optimizado
├── compose.yaml                  # Orquestación Podman/Docker
├── Makefile                      # Atajos de comandos comunes
└── README.md
```
