import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Chart as ChartJS,
  CategoryScale, LinearScale, PointElement, LineElement,
  BarElement, Title, Tooltip, Legend, Filler,
} from 'chart.js'
import { Line, Bar } from 'react-chartjs-2'
import { MapContainer, TileLayer, CircleMarker, Popup } from 'react-leaflet'
import 'leaflet/dist/leaflet.css'
import { Card, CardTitle, StatCard } from '../components/ui/Card'
import { LiveBadge } from '../components/ui/Badge'
import { Btn } from '../components/ui/Button'
import { RegisterDeviceDrawer } from '../components/ui/RegisterDeviceDrawer'
import { useTheme } from '../hooks/useTheme'
import { queryApi } from '../api/query'
import type { DashboardStats } from '../types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, Title, Tooltip, Legend, Filler)

const labels = ['00:00','02:00','04:00','06:00','08:00','10:00','12:00','14:00','16:00','18:00','20:00','22:00']
const ingestionData = [120,98,88,72,160,210,188,240,280,310,270,248]
const alertDays = ['Mon','Tue','Wed','Thu','Fri','Sat','Sun']
const alertCounts = [2, 1, 4, 1, 3, 0, 2]

const devices = [
  { name: 'Greenhouse Sensor A', lat: 10.776, lng: 106.700, status: 'online' },
  { name: 'Farm Node B',         lat: 10.850, lng: 106.780, status: 'online' },
  { name: 'Air Monitor',         lat: 10.730, lng: 106.650, status: 'online' },
  { name: 'Water Quality',       lat: 10.795, lng: 106.720, status: 'warning' },
  { name: 'R&D Lab Node',        lat: 10.810, lng: 106.660, status: 'offline' },
]

export function DashboardPage() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const navigate = useNavigate()
  const { theme } = useTheme()

  useEffect(() => {
    queryApi.stats()
      .then(r => setStats(r.data))
      .catch(() => {})
  }, [])

  const isDark = theme === 'dark'
  const gridColor  = isDark ? 'rgba(48,54,61,.6)'  : 'rgba(208,215,222,.8)'
  const tickColor  = isDark ? '#8b949e'             : '#57606a'

  const chartDefaults = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: { legend: { display: false } },
    scales: {
      x: { grid: { color: gridColor }, ticks: { color: tickColor, font: { size: 11 } } },
      y: { grid: { color: gridColor }, ticks: { color: tickColor, font: { size: 11 } } },
    },
  }

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Dashboard</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>
            Your workspace at a glance — real-time and 24h summary
          </p>
        </div>
        <div style={{ display: 'flex', gap: 8, marginLeft: 'auto' }}>
          <Btn variant="ghost" size="sm">⬇ Export</Btn>
          <Btn variant="primary" size="sm" onClick={() => setDrawerOpen(true)}>+ New Device</Btn>
        </div>
      </div>

      {/* Stats */}
      <div className="rg4" style={{
        display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)',
        gap: 16, marginBottom: 24,
      }}>
        {stats ? (
          <>
            <StatCard label="Active Devices"   icon="🔌" value={String(stats.active_devices)}  change="↑ 2 this week"       changeUp />
            <StatCard label="Readings (24h)"   icon="📊" value={stats.readings_24h >= 1000 ? `${(stats.readings_24h / 1000).toFixed(0)}K` : String(stats.readings_24h)} change="↑ 12% vs yesterday" changeUp />
            <StatCard label="Active Alerts"    icon="🔔" value={<span style={{ color: 'var(--red)' }}>{stats.active_alerts}</span>} change="↑ 1 critical" />
            <StatCard label="Channels"         icon="📡" value={String(stats.total_channels)}  change="↑ 4 this month"      changeUp />
          </>
        ) : (
          <>
            <StatCard label="Active Devices"   icon="🔌" value="—" change="" />
            <StatCard label="Readings (24h)"   icon="📊" value="—" change="" />
            <StatCard label="Active Alerts"    icon="🔔" value="—" change="" />
            <StatCard label="Channels"         icon="📡" value="—" change="" />
          </>
        )}
      </div>

      {/* Charts row */}
      <div className="rg-split" style={{ display: 'grid', gridTemplateColumns: '7fr 5fr', gap: 16, marginBottom: 24 }}>
        <Card>
          <CardTitle>
            Ingestion Rate (readings/min)
            <LiveBadge />
          </CardTitle>
          <div style={{ position: 'relative', height: 220 }}>
            <Line
              data={{
                labels,
                datasets: [{
                  data: ingestionData,
                  borderColor: '#2563eb',
                  backgroundColor: 'rgba(37,99,235,.12)',
                  fill: true,
                  tension: 0.4,
                  pointRadius: 3,
                  pointBackgroundColor: '#2563eb',
                }],
              }}
              options={chartDefaults as any}
            />
          </div>
        </Card>
        <Card>
          <CardTitle>Alert Activity (7d)</CardTitle>
          <div style={{ position: 'relative', height: 220 }}>
            <Bar
              data={{
                labels: alertDays,
                datasets: [{
                  data: alertCounts,
                  backgroundColor: 'rgba(239,68,68,.6)',
                  borderColor: 'var(--red)',
                  borderWidth: 1,
                  borderRadius: 4,
                }],
              }}
              options={chartDefaults as any}
            />
          </div>
        </Card>
      </div>

      {/* Bottom row */}
      <div className="rg2" style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
        {/* Recent Alerts */}
        <Card>
          <CardTitle>
            Recent Alert Events
            <a onClick={() => navigate('/notifications')} style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--accent-lt)', cursor: 'pointer' }}>
              View all →
            </a>
          </CardTitle>
          {[
            { color: 'var(--red)',    title: '🔴 Temperature exceeded 85°C', sub: 'Greenhouse Sensor A · field1', time: '2 min ago' },
            { color: 'var(--yellow)', title: '🟡 Humidity below 30%',        sub: 'Farm Node B · field2',        time: '18 min ago' },
            { color: 'var(--green)',  title: '🟢 CO₂ level back to normal',  sub: 'Air Monitor · field3',        time: '1 hr ago' },
          ].map((ev, i, arr) => (
            <div key={i} style={{
              display: 'flex', gap: 14, padding: '12px 0',
              borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none',
            }}>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                <span style={{
                  width: 10, height: 10, borderRadius: '50',
                  background: ev.color, flexShrink: 0, marginTop: 4,
                  display: 'inline-block',
                }} />
                {i < arr.length - 1 && (
                  <div style={{ width: 1, flex: 1, background: 'var(--border2)', margin: '4px 0' }} />
                )}
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 13, fontWeight: 600 }}>{ev.title}</div>
                <div style={{ fontSize: 12, color: 'var(--muted)' }}>{ev.sub}</div>
              </div>
              <div style={{ fontSize: 11, color: 'var(--muted)', whiteSpace: 'nowrap' }}>{ev.time}</div>
            </div>
          ))}
        </Card>

        {/* Top Channels */}
        <Card>
          <CardTitle>Top Channels by Volume</CardTitle>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Channel','Readings (24h)','Trend'].map(h => (
                  <th key={h} style={{
                    fontSize: 11, fontWeight: 600, textTransform: 'uppercase',
                    letterSpacing: '.06em', color: 'var(--muted)', textAlign: 'left',
                    padding: '10px 12px', borderBottom: '1px solid var(--border)',
                  }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {[
                { name: 'Greenhouse A', fields: 4, reads: '42,110', trend: '↑ 14%', up: true },
                { name: 'Farm Node B',  fields: 3, reads: '38,520', trend: '↑ 5%',  up: true },
                { name: 'Air Monitor',  fields: 6, reads: '27,094', trend: '↓ 3%',  up: false },
                { name: 'Water Quality',fields: 5, reads: '19,882', trend: '↑ 21%', up: true },
              ].map((row, i, arr) => (
                <tr key={i}>
                  <td style={{ padding: '12px 12px', borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none' }}>
                    <div style={{ fontWeight: 600 }}>{row.name}</div>
                    <div style={{ color: 'var(--muted)', fontSize: 11 }}>{row.fields} fields</div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none' }}>{row.reads}</td>
                  <td style={{ padding: '12px 12px', borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none', color: row.up ? 'var(--green)' : 'var(--red)' }}>{row.trend}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>

      {/* Map */}
      <Card style={{ marginTop: 24 }}>
        <CardTitle>
          Device Locations
          <span style={{ marginLeft: 'auto' }}>
            <LiveBadge />
          </span>
        </CardTitle>
        <div style={{ height: 320, borderRadius: 'var(--radius)', overflow: 'hidden' }}>
          <MapContainer
            center={[10.776, 106.700]}
            zoom={11}
            style={{ height: '100%', width: '100%' }}
            zoomControl={true}
          >
            <TileLayer url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png" />
            {devices.map((d, i) => (
              <CircleMarker
                key={i}
                center={[d.lat, d.lng]}
                radius={8}
                color={d.status === 'online' ? '#22c55e' : d.status === 'warning' ? '#f59e0b' : '#ef4444'}
                fillColor={d.status === 'online' ? '#22c55e' : d.status === 'warning' ? '#f59e0b' : '#ef4444'}
                fillOpacity={0.8}
              >
                <Popup>
                  <strong>{d.name}</strong>
                  <span style={{ display: 'block', fontSize: 12 }}>{d.status}</span>
                </Popup>
              </CircleMarker>
            ))}
          </MapContainer>
        </div>
      </Card>

      <RegisterDeviceDrawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        onRegister={() => {}}
      />
    </div>
  )
}
