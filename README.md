# Network Dashboard

A comprehensive network reconnaissance and monitoring system that discovers hosts, scans ports, fingerprints services, and tracks changes over time through a web-based dashboard.

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Scanner   │────▶│     API     │────▶│  PostgreSQL │     │     UI      │
│    (Go)     │     │  (FastAPI)  │◀────│             │     │   (React)   │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
                           ▲                                       │
                           └───────────────────────────────────────┘
```

**Technology Stack:**
- **Scanner**: Go 1.24 - Network scanning with TCP connect or Zmap
- **API**: Python 3.12 / FastAPI - REST API with async SQLAlchemy
- **Database**: PostgreSQL 15
- **UI**: React 18 / TypeScript / Tailwind CSS / Vite

## Components

### Scanner (`/scanner`)

A Go-based network scanner that discovers hosts and fingerprints services.

**Features:**
- Two scanning modes: native TCP connect or Zmap (for faster large-scale scans)
- CIDR notation support for network ranges
- Service fingerprinting for common protocols (SSH, HTTP, FTP, databases, etc.)
- Cron-based scheduling for continuous monitoring
- Rate limiting to control network impact

**Key files:**
- `main.go` - Orchestration, scheduling, and API submission
- `scanner/tcp.go` - Native TCP connect scanner
- `scanner/zmap.go` - Zmap-based scanner for large networks
- `scanner/fingerprint.go` - Protocol-specific service identification
- `config.yaml` - Network targets, ports, schedule, and mode settings

### API (`/api`)

A FastAPI backend providing REST endpoints for data access and scanner integration.

**Endpoints:**
| Endpoint | Description |
|----------|-------------|
| `GET /api/hosts` | List discovered hosts |
| `GET /api/hosts/{id}` | Host details with open ports |
| `GET /api/ports/summary` | Ports grouped by number with host counts |
| `GET /api/ports/by-number/{port}` | All hosts with a specific port open |
| `GET /api/events` | Scan event history |
| `POST /api/scan/results` | Receive scan results from scanner |
| `GET /api/stats` | Dashboard statistics |

**Key files:**
- `app/main.py` - FastAPI application setup and middleware
- `app/models.py` - SQLAlchemy ORM models
- `app/schemas.py` - Pydantic request/response schemas
- `app/routers/` - Endpoint implementations (hosts, ports, events, scan)

### Database (`/db`)

PostgreSQL schema with tables for hosts, ports, services, scan events, and annotations.

**Tables:**
- `hosts` - Discovered network hosts with IP, hostname, MAC
- `ports` - Open ports linked to hosts
- `services` - Service fingerprints with banners and metadata
- `scan_events` - Audit trail of discoveries and state changes
- `annotations` - User notes on hosts and ports

### UI (`/ui`)

A React dashboard for visualizing network data.

**Features:**
- Dashboard with statistics (total/active hosts, ports, recent events)
- Host list with drill-down to port details
- Port analysis page showing all hosts per port
- Scan event timeline
- Annotation system for notes on hosts and ports

**Key files:**
- `src/App.tsx` - Main component with routing
- `src/hooks/useApi.ts` - Data fetching hooks
- `src/components/Dashboard.tsx` - Main dashboard view
- `src/components/HostDetail.tsx` - Individual host page
- `src/components/PortsPage.tsx` - Port analysis view

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Linux host (required for Zmap scanning mode)
- Network access to target ranges from the scanner container

### Configuration

**1. Environment variables:**

```bash
# Copy the example environment file
cp .env.example .env

# Edit .env and set a secure password
# POSTGRES_PASSWORD=your-secure-password
```

**2. Scanner configuration:**

```bash
# Copy the example scanner config
cp scanner/config.yaml.example scanner/config.yaml

# Edit scanner/config.yaml with your network ranges
```

Edit `scanner/config.yaml` to configure:

```yaml
networks:
  - 192.168.1.0/24    # Networks to scan (CIDR notation)

mode: tcp             # "tcp" for native Go, "zmap" for faster scanning

ports:                # Specific ports to scan
  - 22
  - 80
  - 443
# all_ports: true     # Uncomment to scan all 65535 ports

schedule: "*/15 * * * *"  # Cron schedule (every 15 minutes)

rate: 10000           # Packets/sec (zmap) or concurrent connections (tcp)
timeout: 5            # Connection timeout in seconds
```

### Running

```bash
# Start all services
docker compose up -d

# View logs
docker compose logs -f

# Stop services
docker compose down
```

**Service URLs:**
- UI: http://localhost:3000
- API: http://localhost:8000
- API Docs: http://localhost:8000/docs

### Startup Order

1. PostgreSQL starts and initializes schema
2. API waits for database health check, then starts
3. Scanner waits for API health check, then begins scanning
4. UI serves the React application

## Data Flow

1. **Scanner** runs on schedule, scanning configured networks
2. For each discovered host:port, scanner fingerprints the service
3. Scanner POSTs results to `/api/scan/results`
4. **API** upserts hosts/ports, creates service entries, generates events
5. **UI** fetches data via API hooks and displays dashboard

## Development

### API Development

```bash
cd api
pip install -r requirements.txt
uvicorn app.main:app --reload
```

### Scanner Development

```bash
cd scanner
go build -o scanner .
./scanner
```

### UI Development

```bash
cd ui
npm install
npm run dev
```

## Security Notes

- **Database credentials**: Set `POSTGRES_PASSWORD` in `.env` before running (required)
- **Scanner config**: `scanner/config.yaml` is gitignored to protect your network topology
- **CORS**: Configured to allow all origins (development only) - restrict for production
- **Scanner privileges**: Requires elevated privileges (NET_ADMIN, NET_RAW capabilities)
- **Authentication**: Add API authentication before production deployment
