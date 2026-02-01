-- Hosts discovered on the network
CREATE TABLE hosts (
    id SERIAL PRIMARY KEY,
    ip_address INET UNIQUE NOT NULL,
    hostname VARCHAR(255),
    mac_address VARCHAR(17),
    first_seen TIMESTAMP DEFAULT NOW(),
    last_seen TIMESTAMP DEFAULT NOW(),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Open ports on hosts
CREATE TABLE ports (
    id SERIAL PRIMARY KEY,
    host_id INTEGER REFERENCES hosts(id) ON DELETE CASCADE,
    port_number INTEGER NOT NULL,
    protocol VARCHAR(10) DEFAULT 'tcp',
    state VARCHAR(20) DEFAULT 'open',
    first_seen TIMESTAMP DEFAULT NOW(),
    last_seen TIMESTAMP DEFAULT NOW(),
    is_active BOOLEAN DEFAULT TRUE,
    UNIQUE(host_id, port_number, protocol)
);

-- Service fingerprints
CREATE TABLE services (
    id SERIAL PRIMARY KEY,
    port_id INTEGER REFERENCES ports(id) ON DELETE CASCADE,
    service_name VARCHAR(100),
    service_version VARCHAR(100),
    banner TEXT,
    fingerprint_data JSONB,
    detected_at TIMESTAMP DEFAULT NOW()
);

-- Scan event history
CREATE TABLE scan_events (
    id SERIAL PRIMARY KEY,
    scan_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    host_id INTEGER REFERENCES hosts(id),
    port_id INTEGER REFERENCES ports(id),
    details JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- User annotations
CREATE TABLE annotations (
    id SERIAL PRIMARY KEY,
    host_id INTEGER REFERENCES hosts(id) ON DELETE CASCADE,
    port_id INTEGER REFERENCES ports(id) ON DELETE CASCADE,
    note TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_hosts_ip ON hosts(ip_address);
CREATE INDEX idx_hosts_active ON hosts(is_active);
CREATE INDEX idx_ports_host ON ports(host_id);
CREATE INDEX idx_ports_active ON ports(is_active);
CREATE INDEX idx_services_port ON services(port_id);
CREATE INDEX idx_events_host ON scan_events(host_id);
CREATE INDEX idx_events_created ON scan_events(created_at);
CREATE INDEX idx_events_scan_id ON scan_events(scan_id);
CREATE INDEX idx_annotations_host ON annotations(host_id);
CREATE INDEX idx_annotations_port ON annotations(port_id);
