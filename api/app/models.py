from datetime import datetime
from sqlalchemy import (
    Column, Integer, String, Boolean, Text, DateTime, ForeignKey, UniqueConstraint
)
from sqlalchemy.dialects.postgresql import INET, JSONB, UUID
from sqlalchemy.orm import relationship
from app.database import Base


class Host(Base):
    __tablename__ = "hosts"

    id = Column(Integer, primary_key=True, index=True)
    ip_address = Column(INET, unique=True, nullable=False, index=True)
    hostname = Column(String(255), nullable=True)
    mac_address = Column(String(17), nullable=True)
    first_seen = Column(DateTime, default=datetime.utcnow)
    last_seen = Column(DateTime, default=datetime.utcnow)
    is_active = Column(Boolean, default=True, index=True)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    ports = relationship("Port", back_populates="host", cascade="all, delete-orphan")
    events = relationship("ScanEvent", back_populates="host")
    annotations = relationship("Annotation", back_populates="host", cascade="all, delete-orphan")


class Port(Base):
    __tablename__ = "ports"

    id = Column(Integer, primary_key=True, index=True)
    host_id = Column(Integer, ForeignKey("hosts.id", ondelete="CASCADE"), nullable=False, index=True)
    port_number = Column(Integer, nullable=False)
    protocol = Column(String(10), default="tcp")
    state = Column(String(20), default="open")
    first_seen = Column(DateTime, default=datetime.utcnow)
    last_seen = Column(DateTime, default=datetime.utcnow)
    is_active = Column(Boolean, default=True, index=True)

    __table_args__ = (
        UniqueConstraint("host_id", "port_number", "protocol", name="uq_host_port_protocol"),
    )

    host = relationship("Host", back_populates="ports")
    services = relationship("Service", back_populates="port", cascade="all, delete-orphan")
    events = relationship("ScanEvent", back_populates="port")
    annotations = relationship("Annotation", back_populates="port", cascade="all, delete-orphan")


class Service(Base):
    __tablename__ = "services"

    id = Column(Integer, primary_key=True, index=True)
    port_id = Column(Integer, ForeignKey("ports.id", ondelete="CASCADE"), nullable=False, index=True)
    service_name = Column(String(100), nullable=True)
    service_version = Column(String(100), nullable=True)
    banner = Column(Text, nullable=True)
    fingerprint_data = Column(JSONB, nullable=True)
    detected_at = Column(DateTime, default=datetime.utcnow)

    port = relationship("Port", back_populates="services")


class ScanEvent(Base):
    __tablename__ = "scan_events"

    id = Column(Integer, primary_key=True, index=True)
    scan_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    event_type = Column(String(50), nullable=False)
    host_id = Column(Integer, ForeignKey("hosts.id"), nullable=True, index=True)
    port_id = Column(Integer, ForeignKey("ports.id"), nullable=True)
    details = Column(JSONB, nullable=True)
    created_at = Column(DateTime, default=datetime.utcnow, index=True)

    host = relationship("Host", back_populates="events")
    port = relationship("Port", back_populates="events")


class Annotation(Base):
    __tablename__ = "annotations"

    id = Column(Integer, primary_key=True, index=True)
    host_id = Column(Integer, ForeignKey("hosts.id", ondelete="CASCADE"), nullable=True, index=True)
    port_id = Column(Integer, ForeignKey("ports.id", ondelete="CASCADE"), nullable=True, index=True)
    note = Column(Text, nullable=False)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

    host = relationship("Host", back_populates="annotations")
    port = relationship("Port", back_populates="annotations")
