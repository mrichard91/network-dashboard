import type { ScanEvent } from '../types';
import { formatRelativeTime } from '../utils/time';

interface EventHistoryProps {
  events: ScanEvent[];
  loading: boolean;
}

export default function EventHistory({ events, loading }: EventHistoryProps) {
  if (loading) {
    return (
      <div className="p-4 text-center text-gray-500">
        Loading events...
      </div>
    );
  }

  if (events.length === 0) {
    return (
      <div className="p-4 text-center text-gray-500">
        No events yet.
      </div>
    );
  }

  return (
    <div className="divide-y divide-gray-200">
      {events.map((event) => (
        <div key={event.id} className="p-3 hover:bg-gray-50">
          <div className="flex items-start space-x-2">
            <EventIcon type={event.event_type} />
            <div className="flex-1 min-w-0">
              <div className="flex items-center space-x-2">
                <span className="font-medium text-sm">
                  {formatEventType(event.event_type)}
                </span>
              </div>
              <EventDetails event={event} />
              <div className="text-xs text-gray-500 mt-1">
                {formatRelativeTime(event.created_at)}
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function EventIcon({ type }: { type: string }) {
  const iconClasses = {
    host_discovered: 'bg-green-100 text-green-600',
    host_lost: 'bg-red-100 text-red-600',
    port_opened: 'bg-blue-100 text-blue-600',
    port_closed: 'bg-orange-100 text-orange-600',
    service_changed: 'bg-purple-100 text-purple-600',
  }[type] ?? 'bg-gray-100 text-gray-600';

  const icon = {
    host_discovered: '+',
    host_lost: '-',
    port_opened: '⬆',
    port_closed: '⬇',
    service_changed: '~',
  }[type] ?? '•';

  return (
    <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold ${iconClasses}`}>
      {icon}
    </div>
  );
}

function formatEventType(type: string): string {
  const labels: Record<string, string> = {
    host_discovered: 'Host Discovered',
    host_lost: 'Host Lost',
    port_opened: 'Port Opened',
    port_closed: 'Port Closed',
    service_changed: 'Service Changed',
  };
  return labels[type] ?? type;
}

function EventDetails({ event }: { event: ScanEvent }) {
  const details = event.details as Record<string, unknown> | null;
  if (!details) return null;

  if (details.ip_address) {
    return (
      <div className="text-sm text-gray-600">
        {String(details.ip_address)}
      </div>
    );
  }

  if (details.port_number) {
    return (
      <div className="text-sm text-gray-600">
        Port {String(details.port_number)}/{String(details.protocol ?? 'tcp')}
      </div>
    );
  }

  return null;
}
