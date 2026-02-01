import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { usePortsSummary, usePortDetail } from '../hooks/useApi';
import { getPortName } from '../utils/ports';
import { formatRelativeTime } from '../utils/time';

export default function PortsPage() {
  const { ports, loading, refresh } = usePortsSummary();
  const [selectedPort, setSelectedPort] = useState<number | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();

  useEffect(() => {
    const selectedParam = searchParams.get('selected');

    if (!selectedParam) {
      setSelectedPort(null);
      return;
    }

    const parsed = Number(selectedParam);
    if (!Number.isNaN(parsed)) {
      setSelectedPort(parsed);
    }
  }, [searchParams]);

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div className="flex items-center space-x-4">
          <Link to="/" className="text-blue-600 hover:text-blue-800">
            &larr; Back
          </Link>
          <h1 className="text-2xl font-bold text-gray-900">Discovered Ports</h1>
        </div>
        <button
          onClick={refresh}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
        >
          Refresh
        </button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Port List */}
        <div className="lg:col-span-1">
          <div className="bg-white rounded-lg shadow">
            <div className="px-4 py-3 border-b border-gray-200">
              <h2 className="text-lg font-semibold">Open Ports ({ports.length})</h2>
            </div>
            {loading ? (
              <div className="p-4 text-center text-gray-500">Loading...</div>
            ) : ports.length === 0 ? (
              <div className="p-4 text-center text-gray-500">No ports discovered</div>
            ) : (
              <div className="divide-y divide-gray-200">
                {ports.map((port) => (
                  <button
                    key={`${port.port_number}-${port.protocol}`}
                    onClick={() => {
                      setSelectedPort(port.port_number);
                      setSearchParams({ selected: String(port.port_number) });
                    }}
                    className={`w-full p-4 text-left hover:bg-gray-50 transition-colors ${
                      selectedPort === port.port_number ? 'bg-blue-50' : ''
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="font-mono font-medium text-blue-600">
                          {port.port_number}
                        </span>
                        <span className="text-gray-500">/{port.protocol}</span>
                        {getPortName(port.port_number) && (
                          <span className="ml-2 text-sm text-gray-600">
                            ({getPortName(port.port_number)})
                          </span>
                        )}
                      </div>
                      <span className="px-2 py-1 text-sm bg-gray-100 rounded">
                        {port.host_count} host{port.host_count !== 1 ? 's' : ''}
                      </span>
                    </div>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Port Detail */}
        <div className="lg:col-span-2">
          {selectedPort ? (
            <PortDetailView portNumber={selectedPort} />
          ) : (
            <div className="bg-white rounded-lg shadow p-8 text-center text-gray-500">
              Select a port to view hosts
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function PortDetailView({ portNumber }: { portNumber: number }) {
  const { portDetail, loading, error } = usePortDetail(portNumber);

  if (loading) {
    return (
      <div className="bg-white rounded-lg shadow p-8 text-center text-gray-500">
        Loading...
      </div>
    );
  }

  if (error || !portDetail) {
    return (
      <div className="bg-white rounded-lg shadow p-8 text-center text-red-500">
        Failed to load port details
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow">
      <div className="px-4 py-3 border-b border-gray-200">
        <h2 className="text-lg font-semibold">
          Port {portDetail.port_number}/{portDetail.protocol}
          {getPortName(portDetail.port_number) && (
            <span className="ml-2 text-gray-500 font-normal">
              ({getPortName(portDetail.port_number)})
            </span>
          )}
          <span className="ml-4 text-sm font-normal text-gray-500">
            {portDetail.host_count} host{portDetail.host_count !== 1 ? 's' : ''}
          </span>
        </h2>
      </div>
      <div className="divide-y divide-gray-200">
        {portDetail.hosts.map((item) => (
          <div key={item.host.id} className="p-4 hover:bg-gray-50">
            <div className="flex items-start justify-between">
              <div>
                <Link
                  to={`/hosts/${item.host.id}`}
                  className="font-medium text-blue-600 hover:text-blue-800"
                >
                  {item.host.ip_address}
                </Link>
                {item.host.hostname && (
                  <span className="ml-2 text-gray-500">{item.host.hostname}</span>
                )}
                <div
                  className={`inline-block ml-2 w-2 h-2 rounded-full ${
                    item.host.is_active ? 'bg-green-500' : 'bg-gray-400'
                  }`}
                />
              </div>
              <div className="text-sm text-gray-500">
                Last seen: {formatRelativeTime(item.port.last_seen)}
              </div>
            </div>

            {item.service && (
              <div className="mt-2 text-sm">
                {item.service.name && (
                  <span className="inline-block px-2 py-0.5 bg-purple-100 text-purple-800 rounded mr-2">
                    {item.service.name}
                    {item.service.version && ` ${item.service.version}`}
                  </span>
                )}
                {item.service.banner && (
                  <div className="mt-1 p-2 bg-gray-100 rounded font-mono text-xs text-gray-700 overflow-x-auto">
                    {item.service.banner}
                  </div>
                )}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
