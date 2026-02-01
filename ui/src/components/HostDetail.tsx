import { useParams, Link } from 'react-router-dom';
import { useHost, useEvents, useHostAnnotations } from '../hooks/useApi';
import PortList from './PortList';
import EventHistory from './EventHistory';
import AnnotationForm from './AnnotationForm';

export default function HostDetail() {
  const { id } = useParams<{ id: string }>();
  const hostId = parseInt(id ?? '0', 10);

  const { host, loading: hostLoading, error: hostError } = useHost(hostId);
  const { events, loading: eventsLoading } = useEvents(hostId, 20);
  const { annotations, loading: annotationsLoading, addAnnotation } = useHostAnnotations(hostId);

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

function formatDateTime(dateString: string): string {
  return new Date(dateString).toLocaleString();
}
