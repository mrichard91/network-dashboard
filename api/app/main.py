from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.database import engine, Base
from app.routers import hosts, ports, events, annotations, scan


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Create tables on startup
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    yield


app = FastAPI(
    title="Network Dashboard API",
    description="API for network scanning dashboard",
    version="1.0.0",
    lifespan=lifespan,
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(hosts.router, prefix="/api")
app.include_router(ports.router, prefix="/api")
app.include_router(events.router, prefix="/api")
app.include_router(annotations.router, prefix="/api")
app.include_router(scan.router, prefix="/api")


@app.get("/api/health")
async def health_check():
    return {"status": "healthy"}
