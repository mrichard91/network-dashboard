from fastapi import APIRouter, Depends, Query
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.database import get_db
from app.models import ScanEvent
from app.schemas import ScanEvent as ScanEventSchema

router = APIRouter(prefix="/events", tags=["events"])


@router.get("", response_model=list[ScanEventSchema])
async def list_events(
    skip: int = 0,
    limit: int = 100,
    event_type: str | None = None,
    event_types: list[str] | None = Query(None),
    db: AsyncSession = Depends(get_db),
):
    query = select(ScanEvent)
    if event_types:
        query = query.where(ScanEvent.event_type.in_(event_types))
    elif event_type:
        query = query.where(ScanEvent.event_type == event_type)
    query = query.order_by(ScanEvent.created_at.desc()).offset(skip).limit(limit)

    result = await db.execute(query)
    return result.scalars().all()
