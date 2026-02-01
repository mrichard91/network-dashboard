import { useState } from 'react';
import type { Port } from '../types';
import { usePort } from '../hooks/useApi';

interface PortListProps {
  ports: Port[];
}

export default function PortList({ ports }: PortListProps) {
  const [expandedPort, setExpandedPort] = useState<number | null>(null);

  if (ports.length === 0) {
    return (
      <div className="p-4 text-center text-gray-500">
        No open ports found.
      </div>
    );
  }

  return (
    <div className="divide-y divide-gray-200">
      {ports.map((port) => (
        <div key={port.id}>
          <button
            onClick={() => setExpandedPort(expandedPort === port.id ? null : port.id)}
            className="w-full p-4 text-left hover:bg-gray-50 transition-colors"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <span className="font-mono font-medium text-blue-600">
                  {port.port_number}
                </span>
                <span className="text-gray-500">/{port.protocol}</span>
                <span
                  className={`px-2 py-0.5 text-xs rounded ${
                    port.state === 'open'
                      ? 'bg-green-100 text-green-800'
                      : 'bg-gray-100 text-gray-800'
                  }`}
                >
                  {port.state}
                </span>
              </div>
              <span className="text-gray-400">
                {expandedPort === port.id ? '−' : '+'}
              </span>
            </div>
          </button>

          {expandedPort === port.id && <PortDetails portId={port.id} />}
        </div>
      ))}
    </div>
  );
}

function PortDetails({ portId }: { portId: number }) {
  const { port, loading, error } = usePort(portId);

  if (loading) {
    return (
      <div className="px-4 pb-4 text-gray-500">
        Loading service info...
      </div>
    );
  }

  if (error || !port) {
    return (
      <div className="px-4 pb-4 text-red-500">
        Failed to load details
      </div>
    );
  }

  return (
    <div className="px-4 pb-4 bg-gray-50">
      <dl className="grid grid-cols-2 gap-2 text-sm">
        <div>
          <dt className="text-gray-500">First Seen</dt>
          <dd>{new Date(port.first_seen).toLocaleString()}</dd>
        </div>
        <div>
          <dt className="text-gray-500">Last Seen</dt>
          <dd>{new Date(port.last_seen).toLocaleString()}</dd>
        </div>
      </dl>

      {port.services.length > 0 && (
        <div className="mt-4">
          <h4 className="text-sm font-medium text-gray-700 mb-2">
            Detected Services
          </h4>
          <div className="space-y-2">
            {port.services.map((service) => (
              <div
                key={service.id}
                className="p-2 bg-white rounded border border-gray-200"
              >
                <div className="font-medium">
                  {service.service_name ?? 'Unknown Service'}
                  {service.service_version && (
                    <span className="text-gray-500 ml-2">
                      v{service.service_version}
                    </span>
                  )}
                </div>
                {service.banner && (
                  <div className="mt-1 text-xs font-mono text-gray-600 bg-gray-100 p-1 rounded overflow-x-auto">
                    {service.banner}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
