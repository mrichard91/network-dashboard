import { useParams, Link } from 'react-router-dom';
import { useHost, useEvents, useHostAnnotations, useUnifiEnrichment } from '../hooks/useApi';
import type { UnifiDeviceInfo, UnifiClientInfo } from '../types';
import PortList from './PortList';
import EventHistory from './EventHistory';
import AnnotationForm from './AnnotationForm';

export default function HostDetail() {
  const { id } = useParams<{ id: string }>();
  const hostId = parseInt(id ?? '0', 10);

  const { host, loading: hostLoading, error: hostError } = useHost(hostId);
  const { events, loading: eventsLoading } = useEvents(hostId, 20);
  const { annotations, loading: annotationsLoading, addAnnotation } = useHostAnnotations(hostId);
  const { enrichment } = useUnifiEnrichment(
    host?.ip_address ?? null,
    host?.mac_address ?? null,
  );

  if (hostLoading) {
    return (
      <div className="text-center py-8 text-gray-500">
        Loading host details...
      </div>
    );
  }

  if (hostError || !host) {
    return (
      <div className="text-center py-8 text-red-500">
        Failed to load host: {hostError ?? 'Not found'}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-4">
        <Link
          to="/"
          className="text-blue-600 hover:text-blue-800"
        >
          &larr; Back
        </Link>
        <h1 className="text-2xl font-bold text-gray-900">{host.ip_address}</h1>
        <span
          className={`px-2 py-1 text-xs rounded-full ${
            host.is_active
              ? 'bg-green-100 text-green-800'
              : 'bg-gray-100 text-gray-800'
          }`}
        >
          {host.is_active ? 'Active' : 'Inactive'}
        </span>
      </div>

      {/* Host Info */}
      <div className="bg-white rounded-lg shadow p-4">
        <h2 className="text-lg font-semibold mb-4">Host Information</h2>
        <dl className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div>
            <dt className="text-sm text-gray-500">IP Address</dt>
            <dd className="font-medium">{host.ip_address}</dd>
          </div>
          <div>
            <dt className="text-sm text-gray-500">Hostname</dt>
            <dd className="font-medium">{host.hostname ?? 'Unknown'}</dd>
          </div>
          <div>
            <dt className="text-sm text-gray-500">MAC Address</dt>
            <dd className="font-medium font-mono text-sm">
              {host.mac_address ?? 'Unknown'}
            </dd>
          </div>
          <div>
            <dt className="text-sm text-gray-500">Open Ports</dt>
            <dd className="font-medium">{host.ports.length}</dd>
          </div>
          <div>
            <dt className="text-sm text-gray-500">First Seen</dt>
            <dd className="font-medium">{formatDateTime(host.first_seen)}</dd>
          </div>
          <div>
            <dt className="text-sm text-gray-500">Last Seen</dt>
            <dd className="font-medium">{formatDateTime(host.last_seen)}</dd>
          </div>
        </dl>
      </div>

      {/* UniFi Device Info */}
      {enrichment?.type === 'device' && <DeviceCard device={enrichment} />}

      {/* UniFi Client Info */}
      {enrichment?.type === 'client' && <ClientCard client={enrichment} />}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Ports */}
        <div className="bg-white rounded-lg shadow">
          <div className="px-4 py-3 border-b border-gray-200">
            <h2 className="text-lg font-semibold">Open Ports</h2>
          </div>
          <PortList ports={host.ports} />
        </div>

        {/* Events */}
        <div className="bg-white rounded-lg shadow">
          <div className="px-4 py-3 border-b border-gray-200">
            <h2 className="text-lg font-semibold">Event History</h2>
          </div>
          <EventHistory events={events} loading={eventsLoading} />
        </div>
      </div>

      {/* Annotations */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-4 py-3 border-b border-gray-200">
          <h2 className="text-lg font-semibold">Annotations</h2>
        </div>
        <div className="p-4 space-y-4">
          <AnnotationForm onSubmit={addAnnotation} />

          {annotationsLoading ? (
            <div className="text-gray-500">Loading annotations...</div>
          ) : annotations.length === 0 ? (
            <div className="text-gray-500">No annotations yet.</div>
          ) : (
            <div className="space-y-2">
              {annotations.map((annotation) => (
                <div
                  key={annotation.id}
                  className="p-3 bg-gray-50 rounded border border-gray-200"
                >
                  <p className="text-gray-800">{annotation.note}</p>
                  <p className="text-xs text-gray-500 mt-1">
                    {formatDateTime(annotation.created_at)}
                  </p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function DeviceCard({ device }: { device: UnifiDeviceInfo }) {
  return (
    <div className="bg-white rounded-lg shadow p-4">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">UniFi Device</h2>
        <span
          className={`px-2 py-1 text-xs rounded-full ${
            device.status === 'online'
              ? 'bg-green-100 text-green-800'
              : 'bg-red-100 text-red-800'
          }`}
        >
          {device.status ?? 'unknown'}
        </span>
      </div>
      {device.name && (
        <p className="text-gray-800 font-medium mb-3">{device.name}</p>
      )}
      <dl className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div>
          <dt className="text-sm text-gray-500">Model</dt>
          <dd className="font-medium">{device.model ?? 'N/A'}{device.shortname ? ` (${device.shortname})` : ''}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">Product Line</dt>
          <dd className="font-medium">{device.productLine ?? 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">Firmware</dt>
          <dd className="font-medium">{device.version ?? 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">Firmware Status</dt>
          <dd className="font-medium">{device.firmwareStatus ?? 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">IP</dt>
          <dd className="font-medium">{device.ip ?? 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">MAC</dt>
          <dd className="font-medium font-mono text-sm">{device.mac ?? 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">Uptime</dt>
          <dd className="font-medium">{device.startupTime ? formatUptime(device.startupTime) : 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">Console</dt>
          <dd className="font-medium">{device.isConsole ? 'Yes' : 'No'}</dd>
        </div>
      </dl>
    </div>
  );
}

function ClientCard({ client }: { client: UnifiClientInfo }) {
  return (
    <div className="bg-white rounded-lg shadow p-4">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Network Client</h2>
        <span
          className={`px-2 py-1 text-xs rounded-full ${
            client.isWired
              ? 'bg-blue-100 text-blue-800'
              : 'bg-purple-100 text-purple-800'
          }`}
        >
          {client.isWired ? 'Wired' : 'WiFi'}
        </span>
      </div>
      {client.name && (
        <p className="text-gray-800 font-medium mb-3">{client.name}</p>
      )}
      <dl className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div>
          <dt className="text-sm text-gray-500">Manufacturer</dt>
          <dd className="font-medium">{client.oui ?? 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">Network</dt>
          <dd className="font-medium">{client.network ?? 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">IP</dt>
          <dd className="font-medium">{client.ip ?? 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">MAC</dt>
          <dd className="font-medium font-mono text-sm">{client.mac ?? 'N/A'}</dd>
        </div>
        {!client.isWired && (
          <>
            <div>
              <dt className="text-sm text-gray-500">AP Name</dt>
              <dd className="font-medium">{client.apName ?? 'N/A'}</dd>
            </div>
            <div>
              <dt className="text-sm text-gray-500">SSID</dt>
              <dd className="font-medium">{client.essid ?? 'N/A'}</dd>
            </div>
            <div>
              <dt className="text-sm text-gray-500">Channel</dt>
              <dd className="font-medium">{client.channel ?? 'N/A'}</dd>
            </div>
            <div>
              <dt className="text-sm text-gray-500">Signal</dt>
              <dd className="font-medium">{client.signal != null ? `${client.signal} dBm` : 'N/A'}</dd>
            </div>
          </>
        )}
        <div>
          <dt className="text-sm text-gray-500">Uptime</dt>
          <dd className="font-medium">{client.uptime != null ? formatUptimeSeconds(client.uptime) : 'N/A'}</dd>
        </div>
        <div>
          <dt className="text-sm text-gray-500">Last Seen</dt>
          <dd className="font-medium">{client.lastSeen ? formatDateTime(new Date(client.lastSeen * 1000).toISOString()) : 'N/A'}</dd>
        </div>
      </dl>
    </div>
  );
}

function formatDateTime(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

function formatUptime(startupTime: string): string {
  const start = new Date(startupTime).getTime();
  const now = Date.now();
  const diffMs = now - start;
  if (diffMs < 0) return 'N/A';
  const days = Math.floor(diffMs / 86400000);
  const hours = Math.floor((diffMs % 86400000) / 3600000);
  const minutes = Math.floor((diffMs % 3600000) / 60000);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

function formatUptimeSeconds(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}
