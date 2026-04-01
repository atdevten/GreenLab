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
import { devicesApi } from '../api/devices'
import { channelsApi } from '../api/channels'
import { notificationsApi } from '../api/notifications'
import { workspacesApi } from '../api/workspaces'
import type { Device, Channel, Notification } from '../types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, Title, Tooltip, Legend, Filler)

const labels = ['00:00','02:00','04:00','06:00','08:00','10:00','12:00','14:00','16:00','18:00','20:00','22:00']
const ingestionData = [120,98,88,72,160,210,188,240,280,310,270,248]
const alertDays = ['Mon','Tue','Wed','Thu','Fri','Sat','Sun']
const alertCounts = [2, 1, 4, 1, 3, 0, 2]

function timeAgo(isoDate: string): string {
  const ts = new Date(isoDate).getTime()
  if (Number.isNaN(ts)) return 'unknown'
  const diff = Date.now() - ts
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins} min ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs} hr ago`
  return `${Math.floor(hrs / 24)} days ago`
}

function notifColor(type: string): string {
  if (type === 'critical') return 'var(--red)'
  if (type === 'warning') return 'var(--yellow)'
  return 'var(--green)'
}

function notifIcon(type: string): string {
  if (type === 'critical') return '🔴'
  if (type === 'warning') return '🟡'
  return '🟢'
}

export function DashboardPage() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [loading, setLoading] = useState(true)
  const [devices, setDevices] = useState<Device[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [notifications, setNotifications] = useState<Notification[]>([])
  const navigate = useNavigate()
  const { theme } = useTheme()

  useEffect(() => {
    const orgId = localStorage.getItem('org_id') ?? ''
    if (!orgId) { setLoading(false); return }

    workspacesApi.list(orgId).then(wsRes => {
      const wsId = wsRes.data?.[0]?.id
      if (!wsId) { setLoading(false); return }

      Promise.allSettled([
        devicesApi.list({ workspace_id: wsId }),
        channelsApi.list({ workspace_id: wsId }),
        notificationsApi.list(),
      ]).then(([devRes, chanRes, notifRes]) => {
        if (devRes.status === 'fulfilled') setDevices(devRes.value.data ?? [])
        if (chanRes.status === 'fulfilled') setChannels(chanRes.value.data ?? [])
        if (notifRes.status === 'fulfilled') setNotifications(notifRes.value.data ?? [])
      }).finally(() => setLoading(false))
    }).catch(() => setLoading(false))
  }, [])

  const activeDevices = devices.filter(d => d.status === 'active').length
  const readings24h = channels.reduce((sum, c) => sum + (c.reads_24h ?? 0), 0)
  const activeAlerts = notifications.filter(n => !n.read && (n.type === 'critical' || n.type === 'warning')).length
  const totalChannels = channels.length

  const topChannels = [...channels]
    .sort((a, b) => (b.reads_24h ?? 0) - (a.reads_24h ?? 0))
    .slice(0, 4)

  const recentAlerts = notifications
    .filter(n => n.type === 'critical' || n.type === 'warning')
    .slice(0, 3)

  const mapDevices = devices.filter(d => d.lat != null && d.lng != null)

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
        <StatCard label="Active Devices" icon="🔌" value={loading ? '—' : String(activeDevices)} change="" />
        <StatCard label="Readings (24h)" icon="📊" value={loading ? '—' : (readings24h >= 1000 ? `${(readings24h / 1000).toFixed(0)}K` : String(readings24h))} change="" />
        <StatCard label="Active Alerts"  icon="🔔" value={loading ? '—' : <span style={{ color: 'var(--red)' }}>{activeAlerts}</span>} change="" />
        <StatCard label="Channels"       icon="📡" value={loading ? '—' : String(totalChannels)} change="" />
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
          {loading ? (
            <div style={{ fontSize: 13, color: 'var(--muted)', padding: '12px 0' }}>Loading…</div>
          ) : recentAlerts.length === 0 ? (
            <div style={{ fontSize: 13, color: 'var(--muted)', padding: '12px 0' }}>No recent alerts</div>
          ) : recentAlerts.map((ev, i, arr) => (
            <div key={ev.id} style={{
              display: 'flex', gap: 14, padding: '12px 0',
              borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none',
            }}>
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
                <span style={{
                  width: 10, height: 10, borderRadius: '50%',
                  background: notifColor(ev.type), flexShrink: 0, marginTop: 4,
                  display: 'inline-block',
                }} />
                {i < arr.length - 1 && (
                  <div style={{ width: 1, flex: 1, background: 'var(--border2)', margin: '4px 0' }} />
                )}
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 13, fontWeight: 600 }}>{notifIcon(ev.type)} {ev.title}</div>
                <div style={{ fontSize: 12, color: 'var(--muted)' }}>{ev.message}</div>
              </div>
              <div style={{ fontSize: 11, color: 'var(--muted)', whiteSpace: 'nowrap' }}>{timeAgo(ev.created_at)}</div>
            </div>
          ))}
        </Card>

        {/* Top Channels */}
        <Card>
          <CardTitle>Top Channels by Volume</CardTitle>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Channel', 'Readings (24h)', 'Fields'].map(h => (
                  <th key={h} style={{
                    fontSize: 11, fontWeight: 600, textTransform: 'uppercase',
                    letterSpacing: '.06em', color: 'var(--muted)', textAlign: 'left',
                    padding: '10px 12px', borderBottom: '1px solid var(--border)',
                  }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={3} style={{ padding: '12px 12px', color: 'var(--muted)', fontSize: 13 }}>Loading…</td></tr>
              ) : topChannels.length === 0 ? (
                <tr><td colSpan={3} style={{ padding: '12px 12px', color: 'var(--muted)', fontSize: 13 }}>No channels found</td></tr>
              ) : topChannels.map((row, i, arr) => (
                <tr key={row.id}>
                  <td style={{ padding: '12px 12px', borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none' }}>
                    <div style={{ fontWeight: 600 }}>{row.name}</div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none' }}>
                    {(row.reads_24h ?? 0) >= 1000 ? `${((row.reads_24h ?? 0) / 1000).toFixed(1)}K` : (row.reads_24h ?? 0)}
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none', color: 'var(--muted)' }}>
                    {row.fields?.length ?? 0}
                  </td>
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
            {mapDevices.map((d, i) => (
              <CircleMarker
                key={d.id ?? i}
                center={[d.lat!, d.lng!]}
                radius={8}
                color={d.status === 'active' ? '#22c55e' : d.status === 'inactive' ? '#f59e0b' : '#ef4444'}
                fillColor={d.status === 'active' ? '#22c55e' : d.status === 'inactive' ? '#f59e0b' : '#ef4444'}
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
