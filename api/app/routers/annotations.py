from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.database import get_db
from app.models import Annotation
from app.schemas import (
    Annotation as AnnotationSchema,
    AnnotationUpdate,
)

router = APIRouter(prefix="/annotations", tags=["annotations"])


@router.get("", response_model=list[AnnotationSchema])
async def list_annotations(
    skip: int = 0,
    limit: int = 100,
    db: AsyncSession = Depends(get_db),
):
    query = (
        select(Annotation)
        .order_by(Annotation.created_at.desc())
        .offset(skip)
        .limit(limit)
    )
    result = await db.execute(query)
    return result.scalars().all()


@router.get("/{annotation_id}", response_model=AnnotationSchema)
async def get_annotation(annotation_id: int, db: AsyncSession = Depends(get_db)):
    query = select(Annotation).where(Annotation.id == annotation_id)
    result = await db.execute(query)
    annotation = result.scalar_one_or_none()

    if not annotation:
        raise HTTPException(status_code=404, detail="Annotation not found")

    return annotation


@router.patch("/{annotation_id}", response_model=AnnotationSchema)
async def update_annotation(
    annotation_id: int,
    annotation_update: AnnotationUpdate,
    db: AsyncSession = Depends(get_db),
):
    query = select(Annotation).where(Annotation.id == annotation_id)
    result = await db.execute(query)
    annotation = result.scalar_one_or_none()

    if not annotation:
        raise HTTPException(status_code=404, detail="Annotation not found")

    annotation.note = annotation_update.note
    await db.commit()
    await db.refresh(annotation)
    return annotation


@router.delete("/{annotation_id}")
async def delete_annotation(annotation_id: int, db: AsyncSession = Depends(get_db)):
    query = select(Annotation).where(Annotation.id == annotation_id)
    result = await db.execute(query)
    annotation = result.scalar_one_or_none()

    if not annotation:
        raise HTTPException(status_code=404, detail="Annotation not found")

    await db.delete(annotation)
    await db.commit()
    return {"status": "deleted"}
