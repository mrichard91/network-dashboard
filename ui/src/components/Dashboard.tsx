import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useStats, useHosts, useEvents, usePortsSummary, useScanStatus } from '../hooks/useApi';
import HostList from './HostList';
import EventHistory from './EventHistory';
import { getPortName } from '../utils/ports';
import { formatRelativeTime } from '../utils/time';

const DISCOVERY_EVENTS = ['host_discovered', 'port_opened'];

export default function Dashboard() {
  const { stats, loading: statsLoading } = useStats();
  const { hosts, loading: hostsLoading, refresh: refreshHosts } = useHosts(false);
  const { events, loading: eventsLoading, refresh: refreshEvents } = useEvents(undefined, 25, DISCOVERY_EVENTS);
  const { ports, loading: portsLoading, refresh: refreshPorts } = usePortsSummary();
  const { status: scanStatus, triggerScan, refresh: refreshScanStatus } = useScanStatus();

  // Refresh data when scan completes
  useEffect(() => {
    if (scanStatus && !scanStatus.is_scanning) {
      refreshHosts();
      refreshEvents();
      refreshPorts();
    }
  }, [scanStatus?.is_scanning]);

  const handleRefresh = () => {
    refreshHosts();
    refreshEvents();
    refreshPorts();
    refreshScanStatus();
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Network Dashboard</h1>
          {scanStatus && (
            <div className="text-sm text-gray-500 mt-1">
              {scanStatus.is_scanning ? (
                <span className="text-blue-600">Scanning...</span>
              ) : scanStatus.last_scan_time ? (
                <span>Last scan: {formatRelativeTime(scanStatus.last_scan_time)}</span>
              ) : (
                <span>No scans completed yet</span>
              )}
            </div>
          )}
        </div>
        <div className="flex gap-2">
          <button
            onClick={triggerScan}
            disabled={scanStatus?.is_scanning}
            className={`px-4 py-2 rounded-lg transition-colors ${
              scanStatus?.is_scanning
                ? 'bg-gray-400 text-white cursor-not-allowed'
                : 'bg-green-600 text-white hover:bg-green-700'
            }`}
          >
            {scanStatus?.is_scanning ? 'Scanning...' : 'Scan Now'}
          </button>
          <button
            onClick={handleRefresh}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
          >
            Refresh
          </button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
        <StatCard
          title="Total Hosts"
          value={statsLoading ? '...' : stats?.total_hosts ?? 0}
          color="blue"
        />
        <StatCard
          title="Active Hosts"
          value={statsLoading ? '...' : stats?.active_hosts ?? 0}
          color="green"
        />
        <StatCard
          title="Total Ports"
          value={statsLoading ? '...' : stats?.total_ports ?? 0}
          color="purple"
        />
        <StatCard
          title="Active Ports"
          value={statsLoading ? '...' : stats?.active_ports ?? 0}
          color="indigo"
        />
        <StatCard
          title="Recent Events"
          value={statsLoading ? '...' : stats?.recent_events_count ?? 0}
          subtitle="Last 24h"
          color="orange"
        />
      </div>

      {/* Main Content - Three Columns */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Discovered Hosts */}
        <div>
          <div className="bg-white rounded-lg shadow">
            <div className="px-4 py-3 border-b border-gray-200">
              <h2 className="text-lg font-semibold text-gray-900">Discovered Hosts</h2>
            </div>
            <HostList hosts={hosts} loading={hostsLoading} />
          </div>
        </div>

        {/* Discovered Ports */}
        <div>
          <div className="bg-white rounded-lg shadow">
            <div className="px-4 py-3 border-b border-gray-200 flex justify-between items-center">
              <h2 className="text-lg font-semibold text-gray-900">Discovered Ports</h2>
              <Link
                to="/ports"
                className="text-sm text-blue-600 hover:text-blue-800"
              >
                View all &rarr;
              </Link>
            </div>
            {portsLoading ? (
              <div className="p-4 text-center text-gray-500">Loading...</div>
            ) : ports.length === 0 ? (
              <div className="p-4 text-center text-gray-500">No ports discovered</div>
            ) : (
              <div className="divide-y divide-gray-200">
                {ports.slice(0, 25).map((port) => (
                  <Link
                    key={`${port.port_number}-${port.protocol}`}
                    to={`/ports?selected=${port.port_number}`}
                    className="block p-3 hover:bg-gray-50 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="font-mono font-medium text-blue-600">
                          {port.port_number}
                        </span>
                        <span className="text-gray-500 text-sm">/{port.protocol}</span>
                        {getPortName(port.port_number) && (
                          <span className="ml-2 text-xs text-gray-500">
                            {getPortName(port.port_number)}
                          </span>
                        )}
                      </div>
                      <span className="text-sm text-gray-600">
                        {port.host_count} host{port.host_count !== 1 ? 's' : ''}
                      </span>
                    </div>
                  </Link>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Recent Events */}
        <div>
          <div className="bg-white rounded-lg shadow">
            <div className="px-4 py-3 border-b border-gray-200">
              <h2 className="text-lg font-semibold text-gray-900">Recent Events</h2>
            </div>
            <EventHistory events={events} loading={eventsLoading} />
          </div>
        </div>
      </div>
    </div>
  );
}

interface StatCardProps {
  title: string;
  value: number | string;
  subtitle?: string;
  color: 'blue' | 'green' | 'purple' | 'indigo' | 'orange';
}

function StatCard({ title, value, subtitle, color }: StatCardProps) {
  const colorClasses = {
    blue: 'bg-blue-50 text-blue-700 border-blue-200',
    green: 'bg-green-50 text-green-700 border-green-200',
    purple: 'bg-purple-50 text-purple-700 border-purple-200',
    indigo: 'bg-indigo-50 text-indigo-700 border-indigo-200',
    orange: 'bg-orange-50 text-orange-700 border-orange-200',
  };

  return (
    <div className={`rounded-lg border p-4 ${colorClasses[color]}`}>
      <div className="text-sm font-medium">{title}</div>
      <div className="text-2xl font-bold mt-1">{value}</div>
      {subtitle && <div className="text-xs mt-1 opacity-75">{subtitle}</div>}
    </div>
  );
}
