# Agent Instructions for Network Dashboard

This document provides guidance for AI agents navigating and working with this codebase.

## Project Overview

Network Dashboard is a network reconnaissance system with four components:
- **Scanner** (Go) - Discovers hosts and fingerprints services
- **API** (Python/FastAPI) - REST backend with async database access
- **Database** (PostgreSQL) - Stores hosts, ports, services, events
- **UI** (React/TypeScript) - Web dashboard

## Directory Structure

```
network-dashboard/
├── api/                    # Python FastAPI backend
│   ├── app/
│   │   ├── main.py         # App setup, middleware, routes
│   │   ├── database.py     # Async SQLAlchemy config
│   │   ├── models.py       # ORM models (Host, Port, Service, etc.)
│   │   ├── schemas.py      # Pydantic schemas
│   │   └── routers/        # API endpoints
│   │       ├── hosts.py    # Host CRUD and annotations
│   │       ├── ports.py    # Port queries and summaries
│   │       ├── events.py   # Scan event history
│   │       ├── scan.py     # Scanner result ingestion
│   │       └── annotations.py
│   ├── requirements.txt
│   └── Dockerfile
├── scanner/                # Go network scanner
│   ├── main.go             # Entry point, scheduling, orchestration
│   ├── scanner/
│   │   ├── tcp.go          # TCP connect scanner implementation
│   │   ├── zmap.go         # Zmap-based scanner implementation
│   │   └── fingerprint.go  # Service identification logic
│   ├── config/
│   │   └── config.go       # YAML config loading
│   ├── db/
│   │   └── client.go       # HTTP client for API submission
│   ├── config.yaml         # Runtime configuration
│   ├── go.mod
│   └── Dockerfile
├── db/
│   └── init.sql            # Database schema
├── ui/                     # React frontend
│   ├── src/
│   │   ├── App.tsx         # Main component, routing
│   │   ├── hooks/
│   │   │   └── useApi.ts   # Data fetching hooks
│   │   ├── types/
│   │   │   └── index.ts    # TypeScript interfaces
│   │   └── components/
│   │       ├── Dashboard.tsx
│   │       ├── HostList.tsx
│   │       ├── HostDetail.tsx
│   │       ├── PortList.tsx
│   │       ├── PortsPage.tsx
│   │       ├── EventHistory.tsx
│   │       └── AnnotationForm.tsx
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   └── Dockerfile
└── docker-compose.yml      # Container orchestration
```

## Key Patterns and Conventions

### API (Python/FastAPI)

**Async everywhere**: All database operations use async/await with AsyncSession:
```python
async def get_hosts(db: AsyncSession):
    result = await db.execute(select(Host))
    return result.scalars().all()
```

**Dependency injection**: Database sessions are injected via FastAPI dependencies:
```python
@router.get("/hosts")
async def list_hosts(db: AsyncSession = Depends(get_db)):
```

**Schema separation**: Pydantic schemas in `schemas.py` define API contracts, separate from ORM models in `models.py`.

**Router organization**: Each resource has its own router file in `routers/`.

### Scanner (Go)

**Interface-based design**: Both TCP and Zmap scanners implement the same interface:
```go
type Scanner interface {
    ScanPort(port int) ([]ScanResult, error)
    ScanPorts(ports []int) ([]ScanResult, error)
    ScanAllPorts() ([]ScanResult, error)
}
```

**Concurrent scanning**: Uses goroutines with semaphore for rate limiting:
```go
sem := make(chan struct{}, s.config.Rate)
for _, ip := range ips {
    sem <- struct{}{}
    go func(ip string) {
        defer func() { <-sem }()
        // scan logic
    }(ip)
}
```

**Configuration**: YAML config in `config.yaml`, loaded by `config/config.go`.

### UI (React/TypeScript)

**Custom hooks**: All API calls are wrapped in hooks in `useApi.ts`:
```typescript
const { data: hosts, loading, error, refresh } = useHosts();
```

**Type safety**: All API responses have TypeScript interfaces in `types/index.ts`.

**Tailwind styling**: All components use Tailwind utility classes.

## Common Tasks

### Adding a new API endpoint

1. Add Pydantic schemas to `api/app/schemas.py`
2. Add route to appropriate router in `api/app/routers/`
3. If needed, add new ORM model to `api/app/models.py`

### Adding a new scanner protocol fingerprint

1. Edit `scanner/scanner/fingerprint.go`
2. Add port to `portServiceNames` map
3. Create fingerprint function (e.g., `fingerprintMyService`)
4. Add case to `FingerprintService()` switch statement

### Adding a new UI component

1. Create component in `ui/src/components/`
2. Add any needed types to `ui/src/types/index.ts`
3. Add API hook to `ui/src/hooks/useApi.ts` if fetching new data
4. Add route to `ui/src/App.tsx` if it's a new page

### Modifying the database schema

1. Edit `db/init.sql` with new tables/columns
2. Update `api/app/models.py` with corresponding ORM changes
3. Update `api/app/schemas.py` with Pydantic changes
4. Rebuild containers: `docker compose down && docker compose up --build`

## Data Flow Understanding

**Scan ingestion** is the critical path:

1. `scanner/main.go:runScan()` orchestrates a scan
2. Scanner discovers hosts via `tcp.go` or `zmap.go`
3. `fingerprint.go:FingerprintService()` identifies services
4. `db/client.go:SubmitResults()` POSTs to API
5. `api/app/routers/scan.py:receive_scan_results()` processes:
   - Upserts hosts (creates or updates)
   - Upserts ports with state tracking
   - Creates services with fingerprint data
   - Generates scan_events for state changes

## Database Schema

```
hosts (1) ─────< ports (many)
  │                │
  │                └────< services (many)
  │
  └────< annotations (many)

scan_events ─────> hosts (optional FK)
      │
      └─────> ports (optional FK)
```

**Key relationships:**
- Host has many ports (cascade delete)
- Port has many services (cascade delete)
- Host/Port have many annotations
- ScanEvents reference hosts/ports but don't cascade

## Testing and Debugging

**View API docs**: http://localhost:8000/docs (Swagger UI)

**Check scanner logs**:
```bash
docker compose logs -f scanner
```

**Query database directly**:
```bash
docker compose exec postgres psql -U postgres -d network_dashboard
```

**Test API endpoints**:
```bash
curl http://localhost:8000/api/stats
curl http://localhost:8000/api/hosts
```

## Configuration Reference

**Environment variables** (`.env`):
- `POSTGRES_USER`: Database username (default: postgres)
- `POSTGRES_PASSWORD`: Database password (required)
- `POSTGRES_DB`: Database name (default: network_dashboard)

**Scanner config** (`scanner/config.yaml` - copy from `config.yaml.example`):
- `networks`: List of CIDR ranges to scan
- `scanner_mode`: "tcp" or "zmap"
- `ports`: List of specific ports (or `scan_all_ports: true`)
- `schedule`: Cron expression
- `rate`: Concurrency/packet rate
- `timeout`: Connection timeout seconds
- `interface`: Network interface (leave empty for auto-detect)

**Docker environment variables** (set automatically from .env):
- `DATABASE_URL`: PostgreSQL connection string
- `API_URL`: Scanner's target API endpoint
- `CONFIG_PATH`: Path to scanner config file

## Important Files for Common Changes

| Change | Files to modify |
|--------|-----------------|
| Add new scanned port | `scanner/config.yaml` |
| Change scan schedule | `scanner/config.yaml` |
| Add API endpoint | `api/app/routers/*.py`, `api/app/schemas.py` |
| Add database table | `db/init.sql`, `api/app/models.py` |
| Add UI page | `ui/src/components/*.tsx`, `ui/src/App.tsx` |
| Add API data hook | `ui/src/hooks/useApi.ts`, `ui/src/types/index.ts` |
| Change fingerprint logic | `scanner/scanner/fingerprint.go` |
