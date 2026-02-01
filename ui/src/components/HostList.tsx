import { Link } from 'react-router-dom';
import type { Host } from '../types';
import { formatRelativeTime } from '../utils/time';

interface HostListProps {
  hosts: Host[];
  loading: boolean;
}

export default function HostList({ hosts, loading }: HostListProps) {
  if (loading) {
    return (
      <div className="p-4 text-center text-gray-500">
        Loading hosts...
      </div>
    );
  }

  if (hosts.length === 0) {
    return (
      <div className="p-4 text-center text-gray-500">
        No hosts discovered yet. Scanner will find them shortly.
      </div>
    );
  }

  return (
    <div className="divide-y divide-gray-200">
      {hosts.map((host) => (
        <Link
          key={host.id}
          to={`/hosts/${host.id}`}
          className="block p-4 hover:bg-gray-50 transition-colors"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div
                className={`w-3 h-3 rounded-full ${
                  host.is_active ? 'bg-green-500' : 'bg-gray-400'
                }`}
              />
              <div>
                <div className="font-medium text-gray-900">
                  {host.ip_address}
                </div>
                {host.hostname && (
                  <div className="text-sm text-gray-500">{host.hostname}</div>
                )}
                {host.latest_annotation && (
                  <div className="text-xs text-gray-500 truncate max-w-[200px]">
                    {host.latest_annotation}
                  </div>
                )}
              </div>
            </div>
            <div className="text-right">
              <div className="text-sm font-medium text-gray-900">
                {host.port_count ?? 0} ports
              </div>
              <div className="text-xs text-gray-500">
                Last seen: {formatRelativeTime(host.last_seen)}
              </div>
            </div>
          </div>
        </Link>
      ))}
    </div>
  );
}
