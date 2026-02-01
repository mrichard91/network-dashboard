import os
import httpx
from datetime import datetime
from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy import select, func, cast
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.dialects.postgresql import insert, INET

from app.database import get_db
from app.models import Host, Port, Service, ScanEvent
from app.schemas import ScanResults, DashboardStats

router = APIRouter(tags=["scan"])

SCANNER_URL = os.getenv("SCANNER_URL", "http://scanner:8081")


@router.post("/scan/results")
async def receive_scan_results(results: ScanResults, db: AsyncSession = Depends(get_db)):
    """Receive scan results from the scanner service."""
    scan_id = results.scan_id
    events_created = 0
    hosts_processed = 0
    ports_processed = 0

    for host_data in results.hosts:
        # Upsert host
        host_stmt = insert(Host).values(
            ip_address=host_data.ip_address,
            hostname=host_data.hostname,
            mac_address=host_data.mac_address,
            last_seen=datetime.utcnow(),
            is_active=True,
        )
        host_stmt = host_stmt.on_conflict_do_update(
            index_elements=["ip_address"],
            set_={
                "hostname": host_data.hostname or Host.hostname,
                "mac_address": host_data.mac_address or Host.mac_address,
                "last_seen": datetime.utcnow(),
                "is_active": True,
                "updated_at": datetime.utcnow(),
            },
        )
        await db.execute(host_stmt)

        # Get the host ID
        host_query = select(Host).where(Host.ip_address == cast(host_data.ip_address, INET))
        host_result = await db.execute(host_query)
        host = host_result.scalar_one()

        # Check if this is a newly discovered host
        if host.first_seen == host.last_seen:
            event = ScanEvent(
                scan_id=scan_id,
                event_type="host_discovered",
                host_id=host.id,
                details={"ip_address": host_data.ip_address},
            )
            db.add(event)
            events_created += 1

        hosts_processed += 1

        # Process ports for this host
        seen_ports = set()
        for port_data in host_data.ports:
            seen_ports.add((port_data.port_number, port_data.protocol))

            # Check if port already exists
            port_query = select(Port).where(
                Port.host_id == host.id,
                Port.port_number == port_data.port_number,
                Port.protocol == port_data.protocol,
            )
            port_result = await db.execute(port_query)
            existing_port = port_result.scalar_one_or_none()

            if existing_port:
                # Update existing port
                was_inactive = not existing_port.is_active
                existing_port.last_seen = datetime.utcnow()
                existing_port.state = port_data.state
                existing_port.is_active = True

                if was_inactive:
                    event = ScanEvent(
                        scan_id=scan_id,
                        event_type="port_opened",
                        host_id=host.id,
                        port_id=existing_port.id,
                        details={
                            "port_number": port_data.port_number,
                            "protocol": port_data.protocol,
                        },
                    )
                    db.add(event)
                    events_created += 1

                port = existing_port
            else:
                # Create new port
                port = Port(
                    host_id=host.id,
                    port_number=port_data.port_number,
                    protocol=port_data.protocol,
                    state=port_data.state,
                    is_active=True,
                )
                db.add(port)
                await db.flush()

                event = ScanEvent(
                    scan_id=scan_id,
                    event_type="port_opened",
                    host_id=host.id,
                    port_id=port.id,
                    details={
                        "port_number": port_data.port_number,
                        "protocol": port_data.protocol,
                    },
                )
                db.add(event)
                events_created += 1

            ports_processed += 1

            # Upsert service info if available
            if port_data.service_name or port_data.banner:
                # Check for existing service on this port
                service_query = select(Service).where(Service.port_id == port.id)
                service_result = await db.execute(service_query)
                existing_service = service_result.scalar_one_or_none()

                if existing_service:
                    # Update existing service if data changed
                    if (existing_service.service_name != port_data.service_name or
                        existing_service.service_version != port_data.service_version or
                        existing_service.banner != port_data.banner):
                        existing_service.service_name = port_data.service_name
                        existing_service.service_version = port_data.service_version
                        existing_service.banner = port_data.banner
                        existing_service.fingerprint_data = port_data.fingerprint_data
                        existing_service.detected_at = datetime.utcnow()
                else:
                    # Create new service
                    service = Service(
                        port_id=port.id,
                        service_name=port_data.service_name,
                        service_version=port_data.service_version,
                        banner=port_data.banner,
                        fingerprint_data=port_data.fingerprint_data,
                    )
                    db.add(service)

        # Mark ports not seen in this scan as inactive
        existing_ports_query = select(Port).where(
            Port.host_id == host.id, Port.is_active == True
        )
        existing_ports_result = await db.execute(existing_ports_query)
        for existing_port in existing_ports_result.scalars():
            port_key = (existing_port.port_number, existing_port.protocol)
            if port_key not in seen_ports:
                existing_port.is_active = False
                event = ScanEvent(
                    scan_id=scan_id,
                    event_type="port_closed",
                    host_id=host.id,
                    port_id=existing_port.id,
                    details={
                        "port_number": existing_port.port_number,
                        "protocol": existing_port.protocol,
                    },
                )
                db.add(event)
                events_created += 1

    await db.commit()

    return {
        "status": "success",
        "scan_id": str(scan_id),
        "hosts_processed": hosts_processed,
        "ports_processed": ports_processed,
        "events_created": events_created,
    }


@router.post("/scan/trigger")
async def trigger_scan():
    """Trigger a manual network scan."""
    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            response = await client.post(f"{SCANNER_URL}/trigger")
            return response.json()
    except httpx.RequestError as e:
        raise HTTPException(status_code=503, detail=f"Scanner unavailable: {str(e)}")


@router.get("/scan/status")
async def get_scan_status():
    """Get current scanner status."""
    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            response = await client.get(f"{SCANNER_URL}/status")
            return response.json()
    except httpx.RequestError as e:
        raise HTTPException(status_code=503, detail=f"Scanner unavailable: {str(e)}")


@router.get("/stats", response_model=DashboardStats)
async def get_stats(db: AsyncSession = Depends(get_db)):
    """Get dashboard statistics."""
    # Total and active hosts
    total_hosts_query = select(func.count(Host.id))
    total_hosts_result = await db.execute(total_hosts_query)
    total_hosts = total_hosts_result.scalar() or 0

    active_hosts_query = select(func.count(Host.id)).where(Host.is_active == True)
    active_hosts_result = await db.execute(active_hosts_query)
    active_hosts = active_hosts_result.scalar() or 0

    # Total and active ports
    total_ports_query = select(func.count(Port.id))
    total_ports_result = await db.execute(total_ports_query)
    total_ports = total_ports_result.scalar() or 0

    active_ports_query = select(func.count(Port.id)).where(Port.is_active == True)
    active_ports_result = await db.execute(active_ports_query)
    active_ports = active_ports_result.scalar() or 0

    # Recent events (last 24 hours)
    from datetime import timedelta

    recent_cutoff = datetime.utcnow() - timedelta(hours=24)
    recent_events_query = select(func.count(ScanEvent.id)).where(
        ScanEvent.created_at >= recent_cutoff
    )
    recent_events_result = await db.execute(recent_events_query)
    recent_events = recent_events_result.scalar() or 0

    return DashboardStats(
        total_hosts=total_hosts,
        active_hosts=active_hosts,
        total_ports=total_ports,
        active_ports=active_ports,
        recent_events_count=recent_events,
    )
