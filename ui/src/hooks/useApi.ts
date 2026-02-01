import { useState, useEffect, useCallback } from 'react';
import type {
  Host,
  HostWithPorts,
  Port,
  PortWithServices,
  ScanEvent,
  Annotation,
  DashboardStats,
  PortSummary,
  PortDetail,
} from '../types';

const API_BASE = '/api';

async function fetchApi<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${url}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
  return response.json();
}

export function useHosts(activeOnly = false) {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchApi<Host[]>(`/hosts?active_only=${activeOnly}`);
      setHosts(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [activeOnly]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { hosts, loading, error, refresh };
}

export function useHost(hostId: number) {
  const [host, setHost] = useState<HostWithPorts | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchApi<HostWithPorts>(`/hosts/${hostId}`);
      setHost(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [hostId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { host, loading, error, refresh };
}

export function usePorts(hostId?: number) {
  const [ports, setPorts] = useState<Port[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const url = hostId ? `/ports?host_id=${hostId}` : '/ports';
      const data = await fetchApi<Port[]>(url);
      setPorts(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [hostId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { ports, loading, error, refresh };
}

export function usePort(portId: number) {
  const [port, setPort] = useState<PortWithServices | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        setLoading(true);
        const data = await fetchApi<PortWithServices>(`/ports/${portId}`);
        setPort(data);
        setError(null);
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Unknown error');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [portId]);

  return { port, loading, error };
}

export function useEvents(hostId?: number, limit = 50, eventTypes?: string[]) {
  const [events, setEvents] = useState<ScanEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      let url: string;
      if (hostId) {
        url = `/hosts/${hostId}/events?limit=${limit}`;
      } else {
        const params = new URLSearchParams({ limit: String(limit) });
        if (eventTypes) {
          eventTypes.forEach((t) => params.append('event_types', t));
        }
        url = `/events?${params.toString()}`;
      }
      const data = await fetchApi<ScanEvent[]>(url);
      setEvents(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [hostId, limit, eventTypes]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { events, loading, error, refresh };
}

export function useHostAnnotations(hostId: number) {
  const [annotations, setAnnotations] = useState<Annotation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchApi<Annotation[]>(`/hosts/${hostId}/annotations`);
      setAnnotations(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [hostId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const addAnnotation = async (note: string) => {
    await fetchApi(`/hosts/${hostId}/annotations`, {
      method: 'POST',
      body: JSON.stringify({ note }),
    });
    await refresh();
  };

  return { annotations, loading, error, refresh, addAnnotation };
}

export function useStats() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchApi<DashboardStats>('/stats');
      setStats(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { stats, loading, error, refresh };
}

export function usePortsSummary() {
  const [ports, setPorts] = useState<PortSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchApi<PortSummary[]>('/ports/summary');
      setPorts(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { ports, loading, error, refresh };
}

export function usePortDetail(portNumber: number, protocol = 'tcp') {
  const [portDetail, setPortDetail] = useState<PortDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchApi<PortDetail>(`/ports/by-number/${portNumber}?protocol=${protocol}`);
      setPortDetail(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [portNumber, protocol]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { portDetail, loading, error, refresh };
}

export interface ScanStatus {
  is_scanning: boolean;
  last_scan_time?: string;
}

export function useScanStatus() {
  const [status, setStatus] = useState<ScanStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchApi<ScanStatus>('/scan/status');
      setStatus(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const triggerScan = async () => {
    try {
      await fetchApi('/scan/trigger', { method: 'POST' });
      // Poll for status updates
      const pollInterval = setInterval(async () => {
        const data = await fetchApi<ScanStatus>('/scan/status');
        setStatus(data);
        if (!data.is_scanning) {
          clearInterval(pollInterval);
        }
      }, 2000);
      // Initial refresh
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    }
  };

  return { status, loading, error, refresh, triggerScan };
}
