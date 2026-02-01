from datetime import datetime
from typing import Optional, Any
from uuid import UUID
from pydantic import BaseModel, ConfigDict


# Host schemas
class HostBase(BaseModel):
    ip_address: str
    hostname: Optional[str] = None
    mac_address: Optional[str] = None


class HostCreate(HostBase):
    pass


class HostUpdate(BaseModel):
    hostname: Optional[str] = None
    mac_address: Optional[str] = None
    is_active: Optional[bool] = None


class PortSummary(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: int
    port_number: int
    protocol: str
    state: str
    is_active: bool


class Host(HostBase):
    model_config = ConfigDict(from_attributes=True)

    id: int
    first_seen: datetime
    last_seen: datetime
    is_active: bool
    created_at: datetime
    updated_at: datetime


class HostWithPorts(Host):
    ports: list[PortSummary] = []
    port_count: int = 0


class HostList(Host):
    port_count: int = 0
    latest_annotation: Optional[str] = None


# Port schemas
class PortBase(BaseModel):
    port_number: int
    protocol: str = "tcp"
    state: str = "open"


class PortCreate(PortBase):
    host_id: int


class Port(PortBase):
    model_config = ConfigDict(from_attributes=True)

    id: int
    host_id: int
    first_seen: datetime
    last_seen: datetime
    is_active: bool


# Service schemas
class ServiceBase(BaseModel):
    service_name: Optional[str] = None
    service_version: Optional[str] = None
    banner: Optional[str] = None
    fingerprint_data: Optional[dict[str, Any]] = None


class ServiceCreate(ServiceBase):
    port_id: int


class Service(ServiceBase):
    model_config = ConfigDict(from_attributes=True)

    id: int
    port_id: int
    detected_at: datetime


class PortWithServices(Port):
    services: list[Service] = []


# Event schemas
class ScanEventBase(BaseModel):
    scan_id: UUID
    event_type: str
    host_id: Optional[int] = None
    port_id: Optional[int] = None
    details: Optional[dict[str, Any]] = None


class ScanEventCreate(ScanEventBase):
    pass


class ScanEvent(ScanEventBase):
    model_config = ConfigDict(from_attributes=True)

    id: int
    created_at: datetime


# Annotation schemas
class AnnotationBase(BaseModel):
    note: str


class AnnotationCreate(AnnotationBase):
    host_id: Optional[int] = None
    port_id: Optional[int] = None


class AnnotationUpdate(BaseModel):
    note: str


class Annotation(AnnotationBase):
    model_config = ConfigDict(from_attributes=True)

    id: int
    host_id: Optional[int]
    port_id: Optional[int]
    created_at: datetime
    updated_at: datetime


# Scan results schema (from scanner)
class ScanResultPort(BaseModel):
    port_number: int
    protocol: str = "tcp"
    state: str = "open"
    service_name: Optional[str] = None
    service_version: Optional[str] = None
    banner: Optional[str] = None
    fingerprint_data: Optional[dict[str, Any]] = None


class ScanResultHost(BaseModel):
    ip_address: str
    hostname: Optional[str] = None
    mac_address: Optional[str] = None
    ports: list[ScanResultPort] = []


class ScanResults(BaseModel):
    scan_id: UUID
    hosts: list[ScanResultHost]


# Stats schema
class DashboardStats(BaseModel):
    total_hosts: int
    active_hosts: int
    total_ports: int
    active_ports: int
    recent_events_count: int
