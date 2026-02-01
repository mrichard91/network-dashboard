from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy import select, func
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from app.database import get_db
from app.models import Port, Service, Annotation, Host
from app.schemas import (
    Port as PortSchema,
    PortWithServices,
    Service as ServiceSchema,
    Annotation as AnnotationSchema,
    AnnotationCreate,
)

router = APIRouter(prefix="/ports", tags=["ports"])


@router.get("/summary")
async def get_ports_summary(
    active_only: bool = True,
    db: AsyncSession = Depends(get_db),
):
    """Get ports grouped by port number with host count."""
    query = select(
        Port.port_number,
        Port.protocol,
        func.count(Port.id).label("host_count"),
    )
    if active_only:
        query = query.where(Port.is_active == True)
    query = query.group_by(Port.port_number, Port.protocol).order_by(Port.port_number)

    result = await db.execute(query)
    rows = result.all()

    return [
        {
            "port_number": row.port_number,
            "protocol": row.protocol,
            "host_count": row.host_count,
        }
        for row in rows
    ]


@router.get("/by-number/{port_number}")
async def get_hosts_by_port(
    port_number: int,
    protocol: str = "tcp",
    active_only: bool = True,
    db: AsyncSession = Depends(get_db),
):
    """Get all hosts that have a specific port open."""
    query = (
        select(Port, Host)
        .join(Host, Port.host_id == Host.id)
        .where(Port.port_number == port_number, Port.protocol == protocol)
    )
    if active_only:
        query = query.where(Port.is_active == True)
    query = query.order_by(Host.ip_address)

    result = await db.execute(query)
    rows = result.all()

    # Get service info for each port
    hosts_data = []
    for port, host in rows:
        # Get latest service for this port
        service_query = (
            select(Service)
            .where(Service.port_id == port.id)
            .order_by(Service.detected_at.desc())
            .limit(1)
        )
        service_result = await db.execute(service_query)
        service = service_result.scalar_one_or_none()

        hosts_data.append({
            "host": {
                "id": host.id,
                "ip_address": str(host.ip_address),
                "hostname": host.hostname,
                "is_active": host.is_active,
            },
            "port": {
                "id": port.id,
                "state": port.state,
                "first_seen": port.first_seen,
                "last_seen": port.last_seen,
            },
            "service": {
                "name": service.service_name if service else None,
                "version": service.service_version if service else None,
                "banner": service.banner if service else None,
            } if service else None,
        })

    return {
        "port_number": port_number,
        "protocol": protocol,
        "host_count": len(hosts_data),
        "hosts": hosts_data,
    }


@router.get("", response_model=list[PortSchema])
async def list_ports(
    skip: int = 0,
    limit: int = 100,
    active_only: bool = True,
    host_id: int | None = None,
    db: AsyncSession = Depends(get_db),
):
    query = select(Port)
    if active_only:
        query = query.where(Port.is_active == True)
    if host_id:
        query = query.where(Port.host_id == host_id)
    query = query.offset(skip).limit(limit).order_by(Port.port_number)

    result = await db.execute(query)
    return result.scalars().all()


@router.get("/{port_id}", response_model=PortWithServices)
async def get_port(port_id: int, db: AsyncSession = Depends(get_db)):
    query = (
        select(Port)
        .options(selectinload(Port.services))
        .where(Port.id == port_id)
    )
    result = await db.execute(query)
    port = result.scalar_one_or_none()

    if not port:
        raise HTTPException(status_code=404, detail="Port not found")

    return port


@router.get("/{port_id}/services", response_model=list[ServiceSchema])
async def get_port_services(port_id: int, db: AsyncSession = Depends(get_db)):
    # Verify port exists
    port_query = select(Port).where(Port.id == port_id)
    port_result = await db.execute(port_query)
    if not port_result.scalar_one_or_none():
        raise HTTPException(status_code=404, detail="Port not found")

    query = (
        select(Service)
        .where(Service.port_id == port_id)
        .order_by(Service.detected_at.desc())
    )
    result = await db.execute(query)
    return result.scalars().all()


@router.get("/{port_id}/annotations", response_model=list[AnnotationSchema])
async def get_port_annotations(port_id: int, db: AsyncSession = Depends(get_db)):
    query = (
        select(Annotation)
        .where(Annotation.port_id == port_id)
        .order_by(Annotation.created_at.desc())
    )
    result = await db.execute(query)
    return result.scalars().all()


@router.post("/{port_id}/annotations", response_model=AnnotationSchema)
async def create_port_annotation(
    port_id: int, annotation: AnnotationCreate, db: AsyncSession = Depends(get_db)
):
    # Verify port exists
    port_query = select(Port).where(Port.id == port_id)
    port_result = await db.execute(port_query)
    if not port_result.scalar_one_or_none():
        raise HTTPException(status_code=404, detail="Port not found")

    db_annotation = Annotation(port_id=port_id, note=annotation.note)
    db.add(db_annotation)
    await db.commit()
    await db.refresh(db_annotation)
    return db_annotation
