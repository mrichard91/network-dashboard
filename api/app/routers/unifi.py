import logging

from fastapi import APIRouter, HTTPException, Query

from app.unifi import (
    UNIFI_API_KEY,
    UNIFI_HOST,
    UNIFI_USERNAME,
    find_client_by_host,
    find_device_by_host,
)

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/unifi", tags=["unifi"])

_local_configured = bool(UNIFI_HOST and UNIFI_USERNAME)


@router.get("/host-enrichment")
async def host_enrichment(
    ip: str = Query(default=None),
    mac: str = Query(default=None),
):
    if not UNIFI_API_KEY and not _local_configured:
        raise HTTPException(status_code=503, detail="UniFi integration not configured")

    if not ip and not mac:
        raise HTTPException(status_code=400, detail="Provide ip and/or mac parameter")

    # Try cloud device match first
    if UNIFI_API_KEY:
        device = await find_device_by_host(ip, mac)
        if device:
            return {
                "type": "device",
                "name": device.get("name"),
                "model": device.get("model"),
                "shortname": device.get("shortname"),
                "status": device.get("status"),
                "productLine": device.get("productLine"),
                "version": device.get("version"),
                "firmwareStatus": device.get("firmwareStatus"),
                "mac": device.get("mac"),
                "ip": device.get("ip"),
                "isConsole": device.get("isConsole", False),
                "startupTime": device.get("startupTime"),
            }

    # Fall back to local client match
    if _local_configured:
        client = await find_client_by_host(ip, mac)
        if client:
            return {
                "type": "client",
                "name": client.get("name") or client.get("hostname"),
                "mac": client.get("mac"),
                "ip": client.get("ip"),
                "oui": client.get("oui"),
                "network": client.get("network"),
                "lastSeen": client.get("last_seen"),
                "uptime": client.get("uptime"),
                "isWired": client.get("is_wired", False),
                "apName": client.get("ap_name"),
                "essid": client.get("essid"),
                "channel": client.get("channel"),
                "signal": client.get("signal"),
            }

    raise HTTPException(status_code=404, detail="No matching UniFi device or client found")
