from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy import select, func
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from app.database import get_db
from app.models import Host, Port, ScanEvent, Annotation
from app.schemas import (
    Host as HostSchema,
    HostList,
    HostWithPorts,
    HostCreate,
    HostUpdate,
    ScanEvent as ScanEventSchema,
    Annotation as AnnotationSchema,
    AnnotationCreate,
)

router = APIRouter(prefix="/hosts", tags=["hosts"])


@router.get("", response_model=list[HostList])
async def list_hosts(
    skip: int = 0,
    limit: int = 100,
    active_only: bool = False,
    db: AsyncSession = Depends(get_db),
):
    query = select(Host)
    if active_only:
        query = query.where(Host.is_active == True)
    query = query.offset(skip).limit(limit).order_by(Host.last_seen.desc())

    result = await db.execute(query)
    hosts = result.scalars().all()

    # Get port counts and latest annotations
    host_ids = [h.id for h in hosts]
    if host_ids:
        port_counts_query = (
            select(Port.host_id, func.count(Port.id).label("count"))
            .where(Port.host_id.in_(host_ids), Port.is_active == True)
            .group_by(Port.host_id)
        )
        port_counts_result = await db.execute(port_counts_query)
        port_counts = {row.host_id: row.count for row in port_counts_result}

        # Get latest annotation for each host
        from sqlalchemy import distinct
        from sqlalchemy.orm import aliased

        subquery = (
            select(
                Annotation.host_id,
                func.max(Annotation.created_at).label("max_created")
            )
            .where(Annotation.host_id.in_(host_ids))
            .group_by(Annotation.host_id)
            .subquery()
        )
        annotations_query = (
            select(Annotation)
            .join(
                subquery,
                (Annotation.host_id == subquery.c.host_id)
                & (Annotation.created_at == subquery.c.max_created)
            )
        )
        annotations_result = await db.execute(annotations_query)
        latest_annotations = {a.host_id: a.note for a in annotations_result.scalars()}
    else:
        port_counts = {}
        latest_annotations = {}

    return [
        HostList(
            id=h.id,
            ip_address=str(h.ip_address),
            hostname=h.hostname,
            mac_address=h.mac_address,
            first_seen=h.first_seen,
            last_seen=h.last_seen,
            is_active=h.is_active,
            created_at=h.created_at,
            updated_at=h.updated_at,
            port_count=port_counts.get(h.id, 0),
            latest_annotation=latest_annotations.get(h.id),
        )
        for h in hosts
    ]


@router.get("/{host_id}", response_model=HostWithPorts)
async def get_host(host_id: int, db: AsyncSession = Depends(get_db)):
    query = (
        select(Host)
        .options(selectinload(Host.ports))
        .where(Host.id == host_id)
    )
    result = await db.execute(query)
    host = result.scalar_one_or_none()

    if not host:
        raise HTTPException(status_code=404, detail="Host not found")

    active_ports = [p for p in host.ports if p.is_active]
    return HostWithPorts(
        id=host.id,
        ip_address=str(host.ip_address),
        hostname=host.hostname,
        mac_address=host.mac_address,
        first_seen=host.first_seen,
        last_seen=host.last_seen,
        is_active=host.is_active,
        created_at=host.created_at,
        updated_at=host.updated_at,
        ports=active_ports,
        port_count=len(active_ports),
    )


@router.post("", response_model=HostSchema)
async def create_host(host: HostCreate, db: AsyncSession = Depends(get_db)):
    db_host = Host(
        ip_address=host.ip_address,
        hostname=host.hostname,
        mac_address=host.mac_address,
    )
    db.add(db_host)
    await db.commit()
    await db.refresh(db_host)
    return HostSchema(
        id=db_host.id,
        ip_address=str(db_host.ip_address),
        hostname=db_host.hostname,
        mac_address=db_host.mac_address,
        first_seen=db_host.first_seen,
        last_seen=db_host.last_seen,
        is_active=db_host.is_active,
        created_at=db_host.created_at,
        updated_at=db_host.updated_at,
    )


@router.patch("/{host_id}", response_model=HostSchema)
async def update_host(
    host_id: int, host_update: HostUpdate, db: AsyncSession = Depends(get_db)
):
    query = select(Host).where(Host.id == host_id)
    result = await db.execute(query)
    host = result.scalar_one_or_none()

    if not host:
        raise HTTPException(status_code=404, detail="Host not found")

    update_data = host_update.model_dump(exclude_unset=True)
    for field, value in update_data.items():
        setattr(host, field, value)

    await db.commit()
    await db.refresh(host)
    return HostSchema(
        id=host.id,
        ip_address=str(host.ip_address),
        hostname=host.hostname,
        mac_address=host.mac_address,
        first_seen=host.first_seen,
        last_seen=host.last_seen,
        is_active=host.is_active,
        created_at=host.created_at,
        updated_at=host.updated_at,
    )


@router.get("/{host_id}/events", response_model=list[ScanEventSchema])
async def get_host_events(
    host_id: int,
    skip: int = 0,
    limit: int = 50,
    db: AsyncSession = Depends(get_db),
):
    # Verify host exists
    host_query = select(Host).where(Host.id == host_id)
    host_result = await db.execute(host_query)
    if not host_result.scalar_one_or_none():
        raise HTTPException(status_code=404, detail="Host not found")

    query = (
        select(ScanEvent)
        .where(ScanEvent.host_id == host_id)
        .order_by(ScanEvent.created_at.desc())
        .offset(skip)
        .limit(limit)
    )
    result = await db.execute(query)
    return result.scalars().all()


@router.get("/{host_id}/annotations", response_model=list[AnnotationSchema])
async def get_host_annotations(host_id: int, db: AsyncSession = Depends(get_db)):
    query = (
        select(Annotation)
        .where(Annotation.host_id == host_id)
        .order_by(Annotation.created_at.desc())
    )
    result = await db.execute(query)
    return result.scalars().all()


@router.post("/{host_id}/annotations", response_model=AnnotationSchema)
async def create_host_annotation(
    host_id: int, annotation: AnnotationCreate, db: AsyncSession = Depends(get_db)
):
    # Verify host exists
    host_query = select(Host).where(Host.id == host_id)
    host_result = await db.execute(host_query)
    if not host_result.scalar_one_or_none():
        raise HTTPException(status_code=404, detail="Host not found")

    db_annotation = Annotation(host_id=host_id, note=annotation.note)
    db.add(db_annotation)
    await db.commit()
    await db.refresh(db_annotation)
    return db_annotation
