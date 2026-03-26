import os
import logging
import re
from typing import Optional

import httpx

logger = logging.getLogger(__name__)

UNIFI_API_KEY = os.getenv("UNIFI_API_KEY")
UNIFI_CLOUD_BASE = "https://api.ui.com"

UNIFI_HOST = os.getenv("UNIFI_HOST")
UNIFI_USERNAME = os.getenv("UNIFI_USERNAME")
UNIFI_PASSWORD = os.getenv("UNIFI_PASSWORD")

_unifi_host_id: Optional[str] = None

# Persistent client for local controller (cookie-based auth)
_local_client = httpx.AsyncClient(verify=False, timeout=15.0)


def _normalize_mac(mac: Optional[str]) -> str:
    """Normalize a MAC address to uppercase hex-only (e.g. '1C6A1B811B37')."""
    if not mac:
        return ""
    return re.sub(r"[^0-9A-Fa-f]", "", mac).upper()


async def unifi_cloud_request(path: str, params: Optional[dict] = None) -> dict:
    """Make an authenticated request to the UniFi Cloud Site Manager API."""
    if not UNIFI_API_KEY:
        return {"error": "UniFi integration not configured (set UNIFI_API_KEY)"}

    url = f"{UNIFI_CLOUD_BASE}{path}"
    headers = {"X-API-Key": UNIFI_API_KEY, "Accept": "application/json"}

    async with httpx.AsyncClient(timeout=15.0) as client:
        resp = await client.get(url, headers=headers, params=params)
        resp.raise_for_status()
        return resp.json()


async def get_unifi_host_id() -> Optional[str]:
    """Get the first host (console) ID from UniFi cloud (cached)."""
    global _unifi_host_id
    if _unifi_host_id:
        return _unifi_host_id
    try:
        data = await unifi_cloud_request("/ea/hosts")
        hosts = data.get("data", [])
        if hosts:
            _unifi_host_id = hosts[0]["id"]
            return _unifi_host_id
    except Exception as e:
        logger.warning(f"Failed to get UniFi host ID: {e}")
    return None


async def get_all_devices() -> list[dict]:
    """Fetch all devices for our console."""
    host_id = await get_unifi_host_id()
    if not host_id:
        return []
    try:
        data = await unifi_cloud_request("/ea/devices", {"hosts": host_id})
        devices = []
        for host_entry in data.get("data", []):
            if host_entry.get("hostId") != host_id:
                continue
            devices.extend(host_entry.get("devices", []))
        return devices
    except Exception as e:
        logger.warning(f"Failed to fetch UniFi devices: {e}")
        return []


async def find_device_by_host(ip: Optional[str], mac: Optional[str]) -> Optional[dict]:
    """Match a host against UniFi devices by IP, falling back to MAC."""
    devices = await get_all_devices()
    if not devices:
        return None

    # Try IP match first
    if ip:
        for d in devices:
            if d.get("ip") == ip:
                return d

    # Fallback to MAC match
    if mac:
        normalized = _normalize_mac(mac)
        if normalized:
            for d in devices:
                if _normalize_mac(d.get("mac")) == normalized:
                    return d

    return None


# --- Local controller (client data) ---


async def _unifi_local_login() -> bool:
    """Authenticate to the local UniFi controller. Returns True on success."""
    if not UNIFI_HOST or not UNIFI_USERNAME or not UNIFI_PASSWORD:
        return False
    try:
        resp = await _local_client.post(
            f"{UNIFI_HOST}/api/auth/login",
            json={"username": UNIFI_USERNAME, "password": UNIFI_PASSWORD},
        )
        resp.raise_for_status()
        return True
    except Exception as e:
        logger.warning(f"UniFi local login failed: {e}")
        return False


async def get_all_clients() -> list[dict]:
    """Fetch active clients from the local UniFi controller."""
    if not UNIFI_HOST or not UNIFI_USERNAME or not UNIFI_PASSWORD:
        return []
    url = f"{UNIFI_HOST}/proxy/network/api/s/default/stat/sta"
    try:
        resp = await _local_client.get(url)
        if resp.status_code == 401:
            if not await _unifi_local_login():
                return []
            resp = await _local_client.get(url)
        resp.raise_for_status()
        return resp.json().get("data", [])
    except Exception as e:
        logger.warning(f"Failed to fetch UniFi clients: {e}")
        return []


async def find_client_by_host(ip: Optional[str], mac: Optional[str]) -> Optional[dict]:
    """Match a host against UniFi clients by IP, falling back to MAC."""
    clients = await get_all_clients()
    if not clients:
        return None

    if ip:
        for c in clients:
            if c.get("ip") == ip:
                return c

    if mac:
        normalized = _normalize_mac(mac)
        if normalized:
            for c in clients:
                if _normalize_mac(c.get("mac")) == normalized:
                    return c

    return None
