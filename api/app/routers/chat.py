import json
import os
import logging
from datetime import datetime, timedelta
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy import select, func, cast
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.dialects.postgresql import INET
from sqlalchemy.orm import selectinload

from app.database import get_db
from app.models import Host, Port, Service, ScanEvent, Annotation
from app.unifi import unifi_cloud_request, get_unifi_host_id, UNIFI_API_KEY

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/chat", tags=["chat"])

OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")

SYSTEM_PROMPT = """You are a helpful network dashboard assistant. You help users understand their network by querying host, port, service, and event data. You can also search the web for information and query the UniFi controller for device, client, and network details.

When answering questions:
- Be concise but informative
- Format data clearly (use tables or lists for multiple items)
- Proactively mention relevant details (e.g. if a host has annotations, mention them)
- If data is empty, say so clearly rather than guessing
- Use IP addresses and hostnames when referring to hosts
- When enriching data, cross-reference scan results with UniFi client/device info when relevant (e.g. match by IP or MAC)
- Use web search to look up information about services, CVEs, or anything else that would help the user"""

TOOLS = [
    {
        "type": "function",
        "name": "get_dashboard_stats",
        "description": "Get overall dashboard statistics: total/active host and port counts, and recent event count (last 24h).",
        "parameters": {"type": "object", "properties": {}, "required": []},
    },
    {
        "type": "function",
        "name": "get_hosts",
        "description": "List hosts on the network. Returns IP, hostname, MAC, active status, first/last seen, port count, and latest annotation.",
        "parameters": {
            "type": "object",
            "properties": {
                "active_only": {
                    "type": "boolean",
                    "description": "If true, only return currently active hosts. Default false.",
                },
                "limit": {
                    "type": "integer",
                    "description": "Max number of hosts to return. Default 100.",
                },
            },
            "required": [],
        },
    },
    {
        "type": "function",
        "name": "get_host_detail",
        "description": "Get detailed info about a specific host including its open ports. Look up by host ID or IP address.",
        "parameters": {
            "type": "object",
            "properties": {
                "host_id": {
                    "type": "integer",
                    "description": "The host's database ID.",
                },
                "ip_address": {
                    "type": "string",
                    "description": "The host's IP address.",
                },
            },
            "required": [],
        },
    },
    {
        "type": "function",
        "name": "get_ports_summary",
        "description": "Get a summary of all port numbers seen across the network, grouped by port number and protocol, with the count of hosts that have each port open.",
        "parameters": {
            "type": "object",
            "properties": {
                "active_only": {
                    "type": "boolean",
                    "description": "If true, only include active ports. Default true.",
                },
            },
            "required": [],
        },
    },
    {
        "type": "function",
        "name": "get_port_hosts",
        "description": "Get all hosts that have a specific port number open, including service info (name, version, banner).",
        "parameters": {
            "type": "object",
            "properties": {
                "port_number": {
                    "type": "integer",
                    "description": "The port number to look up.",
                },
                "protocol": {
                    "type": "string",
                    "description": "Protocol (tcp or udp). Default tcp.",
                },
            },
            "required": ["port_number"],
        },
    },
    {
        "type": "function",
        "name": "get_events",
        "description": "Get scan events (host_discovered, port_opened, port_closed). Returns event type, associated host/port IDs, details, and timestamp.",
        "parameters": {
            "type": "object",
            "properties": {
                "event_type": {
                    "type": "string",
                    "description": "Filter by event type: host_discovered, port_opened, or port_closed.",
                },
                "limit": {
                    "type": "integer",
                    "description": "Max events to return. Default 50.",
                },
            },
            "required": [],
        },
    },
    {
        "type": "function",
        "name": "get_annotations",
        "description": "Get user annotations/notes. Can filter by host_id or port_id.",
        "parameters": {
            "type": "object",
            "properties": {
                "host_id": {
                    "type": "integer",
                    "description": "Filter annotations for a specific host.",
                },
                "port_id": {
                    "type": "integer",
                    "description": "Filter annotations for a specific port.",
                },
            },
            "required": [],
        },
    },
    {
        "type": "function",
        "name": "get_unifi_devices",
        "description": "Get all UniFi-managed devices (APs, switches, gateways, cameras, etc.) from the UniFi cloud. Returns name, model, IP, MAC, status, firmware, product line, and adoption time.",
        "parameters": {
            "type": "object",
            "properties": {
                "status_filter": {
                    "type": "string",
                    "description": "Filter by status: online, offline, updating, etc. Default: return all.",
                },
            },
            "required": [],
        },
    },
    {
        "type": "function",
        "name": "get_unifi_site_stats",
        "description": "Get UniFi site overview with aggregate stats: device counts, client counts (wired/wifi/guest), WAN info, ISP info, gateway details.",
        "parameters": {"type": "object", "properties": {}, "required": []},
    },
    {"type": "web_search_preview"},
]


# --- Tool handler functions ---

async def handle_get_dashboard_stats(db: AsyncSession, **kwargs) -> dict:
    total_hosts = (await db.execute(select(func.count(Host.id)))).scalar() or 0
    active_hosts = (await db.execute(
        select(func.count(Host.id)).where(Host.is_active == True)
    )).scalar() or 0
    total_ports = (await db.execute(select(func.count(Port.id)))).scalar() or 0
    active_ports = (await db.execute(
        select(func.count(Port.id)).where(Port.is_active == True)
    )).scalar() or 0
    recent_cutoff = datetime.utcnow() - timedelta(hours=24)
    recent_events = (await db.execute(
        select(func.count(ScanEvent.id)).where(ScanEvent.created_at >= recent_cutoff)
    )).scalar() or 0

    return {
        "total_hosts": total_hosts,
        "active_hosts": active_hosts,
        "total_ports": total_ports,
        "active_ports": active_ports,
        "recent_events_count": recent_events,
    }


async def handle_get_hosts(db: AsyncSession, active_only: bool = False, limit: int = 100, **kwargs) -> list:
    query = select(Host)
    if active_only:
        query = query.where(Host.is_active == True)
    query = query.order_by(Host.last_seen.desc()).limit(limit)

    result = await db.execute(query)
    hosts = result.scalars().all()

    host_ids = [h.id for h in hosts]
    port_counts = {}
    latest_annotations = {}

    if host_ids:
        pc_result = await db.execute(
            select(Port.host_id, func.count(Port.id).label("count"))
            .where(Port.host_id.in_(host_ids), Port.is_active == True)
            .group_by(Port.host_id)
        )
        port_counts = {row.host_id: row.count for row in pc_result}

        subquery = (
            select(
                Annotation.host_id,
                func.max(Annotation.created_at).label("max_created"),
            )
            .where(Annotation.host_id.in_(host_ids))
            .group_by(Annotation.host_id)
            .subquery()
        )
        ann_result = await db.execute(
            select(Annotation).join(
                subquery,
                (Annotation.host_id == subquery.c.host_id)
                & (Annotation.created_at == subquery.c.max_created),
            )
        )
        latest_annotations = {a.host_id: a.note for a in ann_result.scalars()}

    return [
        {
            "id": h.id,
            "ip_address": str(h.ip_address),
            "hostname": h.hostname,
            "mac_address": h.mac_address,
            "is_active": h.is_active,
            "first_seen": h.first_seen.isoformat(),
            "last_seen": h.last_seen.isoformat(),
            "port_count": port_counts.get(h.id, 0),
            "latest_annotation": latest_annotations.get(h.id),
        }
        for h in hosts
    ]


async def handle_get_host_detail(db: AsyncSession, host_id: Optional[int] = None, ip_address: Optional[str] = None, **kwargs) -> dict:
    if not host_id and not ip_address:
        return {"error": "Provide either host_id or ip_address"}

    query = select(Host).options(selectinload(Host.ports))
    if host_id:
        query = query.where(Host.id == host_id)
    else:
        query = query.where(Host.ip_address == cast(ip_address, INET))

    result = await db.execute(query)
    host = result.scalar_one_or_none()
    if not host:
        return {"error": "Host not found"}

    active_ports = [p for p in host.ports if p.is_active]

    # Get services for active ports
    ports_data = []
    for p in active_ports:
        svc_result = await db.execute(
            select(Service).where(Service.port_id == p.id).order_by(Service.detected_at.desc()).limit(1)
        )
        svc = svc_result.scalar_one_or_none()
        ports_data.append({
            "id": p.id,
            "port_number": p.port_number,
            "protocol": p.protocol,
            "state": p.state,
            "first_seen": p.first_seen.isoformat(),
            "last_seen": p.last_seen.isoformat(),
            "service": {
                "name": svc.service_name,
                "version": svc.service_version,
                "banner": svc.banner,
            } if svc else None,
        })

    # Get annotations
    ann_result = await db.execute(
        select(Annotation).where(Annotation.host_id == host.id).order_by(Annotation.created_at.desc())
    )
    annotations = [
        {"id": a.id, "note": a.note, "created_at": a.created_at.isoformat()}
        for a in ann_result.scalars()
    ]

    return {
        "id": host.id,
        "ip_address": str(host.ip_address),
        "hostname": host.hostname,
        "mac_address": host.mac_address,
        "is_active": host.is_active,
        "first_seen": host.first_seen.isoformat(),
        "last_seen": host.last_seen.isoformat(),
        "ports": ports_data,
        "annotations": annotations,
    }


async def handle_get_ports_summary(db: AsyncSession, active_only: bool = True, **kwargs) -> list:
    query = select(
        Port.port_number,
        Port.protocol,
        func.count(Port.id).label("host_count"),
    )
    if active_only:
        query = query.where(Port.is_active == True)
    query = query.group_by(Port.port_number, Port.protocol).order_by(Port.port_number)

    result = await db.execute(query)
    return [
        {"port_number": r.port_number, "protocol": r.protocol, "host_count": r.host_count}
        for r in result.all()
    ]


async def handle_get_port_hosts(db: AsyncSession, port_number: int, protocol: str = "tcp", **kwargs) -> dict:
    query = (
        select(Port, Host)
        .join(Host, Port.host_id == Host.id)
        .where(Port.port_number == port_number, Port.protocol == protocol, Port.is_active == True)
        .order_by(Host.ip_address)
    )
    result = await db.execute(query)
    rows = result.all()

    hosts_data = []
    for port, host in rows:
        svc_result = await db.execute(
            select(Service).where(Service.port_id == port.id).order_by(Service.detected_at.desc()).limit(1)
        )
        svc = svc_result.scalar_one_or_none()
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
                "first_seen": port.first_seen.isoformat(),
                "last_seen": port.last_seen.isoformat(),
            },
            "service": {
                "name": svc.service_name,
                "version": svc.service_version,
                "banner": svc.banner,
            } if svc else None,
        })

    return {
        "port_number": port_number,
        "protocol": protocol,
        "host_count": len(hosts_data),
        "hosts": hosts_data,
    }


async def handle_get_events(db: AsyncSession, event_type: Optional[str] = None, limit: int = 50, **kwargs) -> list:
    query = select(ScanEvent)
    if event_type:
        query = query.where(ScanEvent.event_type == event_type)
    query = query.order_by(ScanEvent.created_at.desc()).limit(limit)

    result = await db.execute(query)
    return [
        {
            "id": e.id,
            "scan_id": str(e.scan_id),
            "event_type": e.event_type,
            "host_id": e.host_id,
            "port_id": e.port_id,
            "details": e.details,
            "created_at": e.created_at.isoformat(),
        }
        for e in result.scalars()
    ]


async def handle_get_annotations(db: AsyncSession, host_id: Optional[int] = None, port_id: Optional[int] = None, **kwargs) -> list:
    query = select(Annotation)
    if host_id:
        query = query.where(Annotation.host_id == host_id)
    if port_id:
        query = query.where(Annotation.port_id == port_id)
    query = query.order_by(Annotation.created_at.desc()).limit(100)

    result = await db.execute(query)
    return [
        {
            "id": a.id,
            "host_id": a.host_id,
            "port_id": a.port_id,
            "note": a.note,
            "created_at": a.created_at.isoformat(),
        }
        for a in result.scalars()
    ]


# --- UniFi Cloud API helpers (delegated to app.unifi) ---


async def handle_get_unifi_devices(db: AsyncSession, status_filter: Optional[str] = None, **kwargs) -> dict:
    host_id = await get_unifi_host_id()
    if not host_id:
        return {"error": "Could not connect to UniFi cloud. Check UNIFI_API_KEY."}
    try:
        params: dict = {"hosts": host_id}
        data = await unifi_cloud_request("/ea/devices", params)

        all_devices = []
        for host_entry in data.get("data", []):
            if host_entry.get("hostId") != host_id:
                continue
            for d in host_entry.get("devices", []):
                if status_filter and status_filter.lower() not in ("all", "") and d.get("status", "").lower() != status_filter.lower():
                    continue
                all_devices.append({
                    "name": d.get("name"),
                    "model": d.get("model"),
                    "shortname": d.get("shortname"),
                    "ip": d.get("ip"),
                    "mac": d.get("mac"),
                    "status": d.get("status"),
                    "productLine": d.get("productLine"),
                    "version": d.get("version"),
                    "firmwareStatus": d.get("firmwareStatus"),
                    "isConsole": d.get("isConsole", False),
                    "startupTime": d.get("startupTime"),
                })

        return {"total": len(all_devices), "devices": all_devices}
    except Exception as e:
        return {"error": f"UniFi API error: {e}"}


async def handle_get_unifi_site_stats(db: AsyncSession, **kwargs) -> dict:
    try:
        data = await unifi_cloud_request("/ea/sites")
        sites = data.get("data", [])
        if not sites:
            return {"error": "No sites found"}

        results = []
        for site in sites:
            stats = site.get("statistics", {})
            counts = stats.get("counts", {})
            isp = stats.get("ispInfo", {})
            wans = stats.get("wans", {})
            gw = stats.get("gateway", {})

            wan_details = {}
            for wan_name, wan_data in wans.items():
                wan_details[wan_name] = {
                    "externalIp": wan_data.get("externalIp"),
                    "uptime": wan_data.get("wanUptime"),
                    "isp": wan_data.get("ispInfo", {}).get("name"),
                }

            results.append({
                "name": site.get("meta", {}).get("desc", "Unknown"),
                "timezone": site.get("meta", {}).get("timezone"),
                "gateway": gw.get("shortname"),
                "isp": isp.get("name"),
                "ispOrg": isp.get("organization"),
                "wans": wan_details,
                "counts": {
                    "totalDevices": counts.get("totalDevice", 0),
                    "offlineDevices": counts.get("offlineDevice", 0),
                    "wifiClients": counts.get("wifiClient", 0),
                    "wiredClients": counts.get("wiredClient", 0),
                    "guestClients": counts.get("guestClient", 0),
                    "wifiDevices": counts.get("wifiDevice", 0),
                    "wiredDevices": counts.get("wiredDevice", 0),
                    "lanConfigurations": counts.get("lanConfiguration", 0),
                    "wanConfigurations": counts.get("wanConfiguration", 0),
                },
            })

        return {"sites": results}
    except Exception as e:
        return {"error": f"UniFi API error: {e}"}


TOOL_HANDLERS = {
    "get_dashboard_stats": handle_get_dashboard_stats,
    "get_hosts": handle_get_hosts,
    "get_host_detail": handle_get_host_detail,
    "get_ports_summary": handle_get_ports_summary,
    "get_port_hosts": handle_get_port_hosts,
    "get_events": handle_get_events,
    "get_annotations": handle_get_annotations,
    "get_unifi_devices": handle_get_unifi_devices,
    "get_unifi_site_stats": handle_get_unifi_site_stats,
}


# --- Models endpoint ---

@router.get("/models")
async def list_models():
    if not OPENAI_API_KEY:
        raise HTTPException(status_code=503, detail="OPENAI_API_KEY not configured")

    try:
        from openai import AsyncOpenAI
        client = AsyncOpenAI(api_key=OPENAI_API_KEY)
        models = await client.models.list()
        return sorted(
            [{"id": m.id, "owned_by": m.owned_by} for m in models.data],
            key=lambda m: m["id"],
        )
    except Exception as e:
        logger.exception("Failed to list models")
        raise HTTPException(status_code=500, detail=f"Failed to list models: {e}")


# --- Request/Response models ---

class ChatRequest(BaseModel):
    message: str
    model: Optional[str] = None
    previous_response_id: Optional[str] = None


class ToolCallInfo(BaseModel):
    name: str
    arguments: dict
    result: Optional[str] = None


class ChatResponse(BaseModel):
    response: str
    response_id: Optional[str] = None
    tool_calls: list[ToolCallInfo] = []


# --- Endpoint ---

@router.post("", response_model=ChatResponse)
async def chat(request: ChatRequest, db: AsyncSession = Depends(get_db)):
    if not OPENAI_API_KEY:
        raise HTTPException(status_code=503, detail="Chat is unavailable: OPENAI_API_KEY not configured")

    try:
        from openai import AsyncOpenAI
        client = AsyncOpenAI(api_key=OPENAI_API_KEY)
    except Exception as e:
        raise HTTPException(status_code=503, detail=f"Failed to initialize OpenAI client: {e}")

    tool_calls_info: list[ToolCallInfo] = []

    try:
        # Build the initial request
        model = request.model or "gpt-4o-mini"  # frontend sends preferred model; this is just a fallback
        api_kwargs: dict = {
            "model": model,
            "instructions": SYSTEM_PROMPT,
            "tools": TOOLS,
            "input": request.message,
        }
        if request.previous_response_id:
            api_kwargs["previous_response_id"] = request.previous_response_id

        response = await client.responses.create(**api_kwargs)

        # Tool-calling loop
        for _ in range(10):
            # Track web search calls for display
            for item in response.output:
                if item.type == "web_search_call":
                    tool_calls_info.append(ToolCallInfo(
                        name="web_search",
                        arguments={"status": getattr(item, "status", "completed")},
                    ))

            # Check if any function tool calls need handling
            tool_call_items = [
                item for item in response.output
                if item.type == "function_call"
            ]

            if not tool_call_items:
                break

            # Process each tool call
            function_results = []
            for tool_call in tool_call_items:
                fn_name = tool_call.name
                try:
                    fn_args = json.loads(tool_call.arguments) if tool_call.arguments else {}
                except json.JSONDecodeError:
                    fn_args = {}

                handler = TOOL_HANDLERS.get(fn_name)
                if handler:
                    result = await handler(db=db, **fn_args)
                    result_str = json.dumps(result, default=str)
                else:
                    result_str = json.dumps({"error": f"Unknown tool: {fn_name}"})

                tool_calls_info.append(ToolCallInfo(
                    name=fn_name,
                    arguments=fn_args,
                    result=result_str,
                ))

                function_results.append({
                    "type": "function_call_output",
                    "call_id": tool_call.call_id,
                    "output": result_str,
                })

            # Send results back and get next response
            response = await client.responses.create(
                model=model,
                instructions=SYSTEM_PROMPT,
                tools=TOOLS,
                previous_response_id=response.id,
                input=function_results,
            )

        # Extract text from final response
        text_parts = [
            item.text for item in response.output
            if item.type == "message" and hasattr(item, "text")
        ]
        if not text_parts:
            # Try extracting from content blocks
            for item in response.output:
                if item.type == "message" and hasattr(item, "content"):
                    for content in item.content:
                        if hasattr(content, "text"):
                            text_parts.append(content.text)

        response_text = "\n".join(text_parts) if text_parts else "I wasn't able to generate a response."

        return ChatResponse(
            response=response_text,
            response_id=response.id,
            tool_calls=tool_calls_info,
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.exception("Chat error")
        raise HTTPException(status_code=500, detail=f"Chat error: {e}")
