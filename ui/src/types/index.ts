export interface Host {
  id: number;
  ip_address: string;
  hostname: string | null;
  mac_address: string | null;
  first_seen: string;
  last_seen: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  port_count?: number;
  latest_annotation?: string | null;
}

export interface Port {
  id: number;
  host_id: number;
  port_number: number;
  protocol: string;
  state: string;
  first_seen: string;
  last_seen: string;
  is_active: boolean;
}

export interface Service {
  id: number;
  port_id: number;
  service_name: string | null;
  service_version: string | null;
  banner: string | null;
  fingerprint_data: Record<string, unknown> | null;
  detected_at: string;
}

export interface PortWithServices extends Port {
  services: Service[];
}

export interface HostWithPorts extends Host {
  ports: Port[];
}

export interface ScanEvent {
  id: number;
  scan_id: string;
  event_type: string;
  host_id: number | null;
  port_id: number | null;
  details: Record<string, unknown> | null;
  created_at: string;
}

export interface Annotation {
  id: number;
  host_id: number | null;
  port_id: number | null;
  note: string;
  created_at: string;
  updated_at: string;
}

export interface DashboardStats {
  total_hosts: number;
  active_hosts: number;
  total_ports: number;
  active_ports: number;
  recent_events_count: number;
}

export interface PortSummary {
  port_number: number;
  protocol: string;
  host_count: number;
}

export interface HostByPort {
  host: {
    id: number;
    ip_address: string;
    hostname: string | null;
    is_active: boolean;
  };
  port: {
    id: number;
    state: string;
    first_seen: string;
    last_seen: string;
  };
  service: {
    name: string | null;
    version: string | null;
    banner: string | null;
  } | null;
}

export interface PortDetail {
  port_number: number;
  protocol: string;
  host_count: number;
  hosts: HostByPort[];
}
