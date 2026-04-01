import { useState, useEffect, useRef } from 'react'
import { Line } from 'react-chartjs-2'
import {
  Chart as ChartJS, CategoryScale, LinearScale,
  PointElement, LineElement, Filler, Tooltip, Legend,
} from 'chart.js'
import { Card, CardTitle } from '../components/ui/Card'
import { LiveBadge } from '../components/ui/Badge'
import { useTheme } from '../hooks/useTheme'
import { channelsApi } from '../api/channels'
import { fieldsApi } from '../api/fields'
import { workspacesApi } from '../api/workspaces'
import { devicesApi } from '../api/devices'
import type { Channel } from '../types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Filler, Tooltip, Legend)

const FIELD_COLORS = [
  '#3b82f6', '#22c55e', '#eab308', '#a855f7',
  '#06b6d4', '#ef4444', '#f97316', '#a78bfa',
]

type WsStatus = 'connecting' | 'connected' | 'disconnected'
// PushMessage is what the backend hub broadcasts
type WsPush = { channel_id: string; device_id: string; fields: Record<string, number>; timestamp: string; type: string }
type FieldDef = { key: string; name: string; unit: string; color: string }
type ChannelEx = Channel & { fieldDefs: FieldDef[] }
type FieldVal = { value: number; trend: string }
type HistEntry = { time: string; value: number }

const HISTORY_LEN = 20
const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

export function LiveDataPage() {
  const [active, setActive]             = useState(0)
  const [channels, setChannels]         = useState<ChannelEx[]>([])
  const [loading, setLoading]           = useState(true)
  const [deviceNames, setDeviceNames]   = useState<Record<string, string>>({})
  const [fieldVals, setFieldVals]       = useState<Record<string, FieldVal>>({})
  const [history, setHistory]           = useState<Record<string, HistEntry[]>>({})
  const [wsStatus, setWsStatus]         = useState<WsStatus>('disconnected')
  const wsRef            = useRef<WebSocket | null>(null)
  const subscribedRef    = useRef<string | null>(null)   // currently subscribed channelId
  const activeChannelRef = useRef<number>(0)             // current active index (for onopen closure)
  const { theme } = useTheme()

  const isDark    = theme === 'dark'
  const gridColor = isDark ? 'rgba(48,54,61,.5)'  : 'rgba(208,215,222,.8)'
  const tickColor = isDark ? '#8b949e'             : '#57606a'

  const chartOpts = {
    responsive: true,
    maintainAspectRatio: false,
    animation: false as const,
    plugins: { legend: { display: false } },
    scales: {
      x: { grid: { color: gridColor }, ticks: { color: tickColor, font: { size: 10 }, maxTicksLimit: 6 } },
      y: { grid: { color: gridColor }, ticks: { color: tickColor, font: { size: 10 } } },
    },
  }

  // Load workspace → channels → fields per channel + device names
  useEffect(() => {
    const orgId = localStorage.getItem('org_id') ?? ''
    if (!orgId) { setLoading(false); return }

    let ignore = false

    workspacesApi.list(orgId)
      .then(async r => {
        const wsId = r.data[0]?.id
        if (!wsId) { if (!ignore) setLoading(false); return }

        const [chRes, devRes] = await Promise.all([
          channelsApi.list({ workspace_id: wsId }),
          devicesApi.list({ workspace_id: wsId }).catch(() => ({ data: [] })),
        ])

        const names: Record<string, string> = {}
        ;(devRes.data as any[]).forEach(d => { names[d.id] = d.name })

        const enriched = await Promise.all(
          (chRes.data as Channel[]).map(async (ch, _chIdx) => {
            try {
              const fRes = await fieldsApi.list(ch.id)
              const fieldDefs: FieldDef[] = (fRes.data as any[]).map((f, i) => ({
                key:   f.name,               // Field.Name = machine key (e.g. "temperature")
                name:  f.label || f.name,    // Field.Label = human name (e.g. "Temperature")
                unit:  f.unit ?? '',
                color: FIELD_COLORS[i % FIELD_COLORS.length],
              }))
              return { ...ch, fieldDefs } as ChannelEx
            } catch {
              return { ...ch, fieldDefs: [] } as ChannelEx
            }
          })
        )
        if (!ignore) {
          setDeviceNames(names)
          setChannels(enriched)
        }
      })
      .catch(() => {})
      .finally(() => { if (!ignore) setLoading(false) })

    return () => { ignore = true }
  }, [])

  // Effect 1 — WS lifecycle. One connection shared across all channel switches.
  useEffect(() => {
    if (channels.length === 0) return

    const token = localStorage.getItem('access_token') ?? ''
    const wsUrl = `${BASE_URL.replace(/^http/, 'ws')}/api/v1/ws?token=${token}`

    setWsStatus('connecting')
    let ws: WebSocket
    try {
      ws = new WebSocket(wsUrl)
    } catch {
      setWsStatus('disconnected')
      return
    }
    wsRef.current = ws

    ws.onopen = () => {
      setWsStatus('connected')
      // Subscribe to whichever channel is active at the moment the socket opens.
      const initialChannelId = channels[activeChannelRef.current]?.id
      if (initialChannelId) {
        ws.send(JSON.stringify({ action: 'subscribe', channel_id: initialChannelId }))
        subscribedRef.current = initialChannelId
      }
    }

    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data) as WsPush
        if (!msg.fields || typeof msg.fields !== 'object') return
        const ts = msg.timestamp
          ? new Date(msg.timestamp).toLocaleTimeString()
          : new Date().toLocaleTimeString()

        setFieldVals(prev => {
          const next = { ...prev }
          for (const [key, val] of Object.entries(msg.fields)) {
            const prevVal = prev[key]?.value ?? val
            const diff    = val - prevVal
            next[key] = {
              value: val,
              trend: diff >= 0 ? `+${diff.toFixed(2)}` : diff.toFixed(2),
            }
          }
          return next
        })

        setHistory(prev => {
          const next = { ...prev }
          for (const [key, val] of Object.entries(msg.fields)) {
            const arr = prev[key] ?? []
            next[key] = [...arr.slice(-(HISTORY_LEN - 1)), { time: ts, value: val }]
          }
          return next
        })
      } catch {}
    }

    ws.onerror = () => { setWsStatus('disconnected'); wsRef.current = null; subscribedRef.current = null }
    ws.onclose = () => { setWsStatus('disconnected'); wsRef.current = null; subscribedRef.current = null }

    return () => {
      ws.close()
      wsRef.current = null
      subscribedRef.current = null
    }
  }, [channels])

  // Effect 2 — Subscription management. Runs when the active tab changes.
  useEffect(() => {
    const newChannelId = channels[active]?.id
    if (!newChannelId) return

    // Clear display state for the incoming channel.
    setFieldVals({})
    setHistory({})

    // Keep the ref in sync so Effect 1's onopen closure sees the right value.
    activeChannelRef.current = active

    // If the socket is not yet open, onopen will handle the initial subscribe.
    if (wsRef.current?.readyState !== WebSocket.OPEN) return

    const ws = wsRef.current

    // Unsubscribe from the previous channel if it differs.
    if (subscribedRef.current && subscribedRef.current !== newChannelId) {
      ws.send(JSON.stringify({ action: 'unsubscribe', channel_id: subscribedRef.current }))
    }

    // Subscribe to the new channel.
    ws.send(JSON.stringify({ action: 'subscribe', channel_id: newChannelId }))
    subscribedRef.current = newChannelId
  }, [active, channels])

  const ch        = channels[active]
  const fieldDefs = ch?.fieldDefs ?? []

  if (loading) return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 300 }}>
      <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
    </div>
  )

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Live Data</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Real-time streaming from your channels</p>
        </div>
        <LiveBadge />
        <div style={{
          display: 'flex', alignItems: 'center', gap: 6, marginLeft: 'auto',
          fontSize: 12, padding: '4px 12px', borderRadius: 99,
          background: wsStatus === 'connected'   ? 'rgba(34,197,94,.12)'
                    : wsStatus === 'connecting'   ? 'rgba(234,179,8,.12)'
                    :                              'rgba(239,68,68,.12)',
          border: `1px solid ${
            wsStatus === 'connected'   ? 'rgba(34,197,94,.3)'
          : wsStatus === 'connecting'   ? 'rgba(234,179,8,.3)'
          :                              'rgba(239,68,68,.3)'}`,
          color: wsStatus === 'connected' ? 'var(--green)' : wsStatus === 'connecting' ? 'var(--yellow)' : 'var(--red)',
        }}>
          <div style={{
            width: 6, height: 6, borderRadius: '50%',
            background: wsStatus === 'connected' ? 'var(--green)' : wsStatus === 'connecting' ? 'var(--yellow)' : 'var(--red)',
          }} />
          {wsStatus === 'connected' ? 'Connected' : wsStatus === 'connecting' ? 'Connecting…' : 'Disconnected'}
        </div>
      </div>

      {channels.length === 0 ? (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 300, gap: 12, color: 'var(--muted)' }}>
          <div style={{ fontSize: 40 }}>📡</div>
          <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--text)' }}>No channels yet</div>
          <div style={{ fontSize: 13 }}>Create a device with a channel first, then data will stream here.</div>
        </div>
      ) : (
        <div className="live-layout" style={{ height: 'calc(100vh - 180px)' }}>
          {/* Channel list */}
          <div style={{ overflowY: 'auto' }}>
            {channels.map((c, i) => (
              <div
                key={c.id}
                onClick={() => setActive(i)}
                style={{
                  padding: '12px 16px', borderRadius: 'var(--radius)', cursor: 'pointer',
                  border: `1px solid ${i === active ? 'var(--accent)' : 'var(--border)'}`,
                  background: i === active ? 'rgba(37,99,235,.12)' : 'var(--surface)',
                  marginBottom: 8, transition: 'all var(--transition)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                  <div style={{
                    width: 7, height: 7, borderRadius: '50%', flexShrink: 0,
                    background: i === active ? 'var(--green)' : 'var(--muted)',
                    boxShadow: i === active ? '0 0 6px var(--green)' : 'none',
                  }} />
                  <div style={{ fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{c.name}</div>
                </div>
                {c.device_id && deviceNames[c.device_id] && (
                  <div style={{ fontSize: 11, color: 'var(--muted)', paddingLeft: 15 }}>
                    {deviceNames[c.device_id]}
                  </div>
                )}
                <div style={{ fontSize: 11, color: 'var(--muted)', paddingLeft: 15, marginTop: 2 }}>
                  {c.fieldDefs.length} field{c.fieldDefs.length !== 1 ? 's' : ''}
                </div>
              </div>
            ))}
          </div>

          {/* Charts + tickers */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16, overflowY: 'auto' }}>
            {fieldDefs.length === 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 240, gap: 10, color: 'var(--muted)' }}>
                <div style={{ fontSize: 32 }}>🔢</div>
                <div style={{ fontSize: 14, fontWeight: 600, color: 'var(--text)' }}>No fields configured</div>
                <div style={{ fontSize: 12 }}>Go to Channels → Edit Schema to add field definitions.</div>
              </div>
            ) : (
              <>
                {/* Field tickers */}
                <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                  {fieldDefs.map(f => {
                    const live = fieldVals[f.key]
                    return (
                      <div key={f.key} style={{
                        background: 'var(--surface)', border: '1px solid var(--border)',
                        borderRadius: 'var(--radius-lg)', padding: '14px 18px',
                        flex: 1, minWidth: 120,
                      }}>
                        <div style={{ fontSize: 11, color: 'var(--muted)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '.06em', marginBottom: 4 }}>
                          {f.name}
                        </div>
                        <div style={{ fontSize: 24, fontWeight: 700 }}>
                          {live ? live.value.toFixed(2) : '—'}
                          {f.unit && <span style={{ fontSize: 13, color: 'var(--muted)', marginLeft: 4 }}>{f.unit}</span>}
                        </div>
                        {live && (
                          <div style={{ fontSize: 11, marginTop: 4, color: live.trend.startsWith('+') ? 'var(--green)' : 'var(--red)' }}>
                            {live.trend}
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>

                {/* Charts per field */}
                {fieldDefs.map(f => {
                  const hist   = history[f.key] ?? []
                  const labels = hist.map(h => h.time)
                  const data   = hist.map(h => h.value)
                  return (
                    <Card key={`${active}-${f.key}`}>
                      <CardTitle>{f.name}{f.unit ? ` (${f.unit})` : ''}</CardTitle>
                      <div style={{ position: 'relative', height: 180 }}>
                        {hist.length === 0 ? (
                          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--muted)', fontSize: 12 }}>
                            Waiting for data…
                          </div>
                        ) : (
                          <Line
                            data={{
                              labels,
                              datasets: [{
                                data,
                                borderColor: f.color,
                                backgroundColor: f.color + '20',
                                fill: true,
                                tension: 0.4,
                                pointRadius: 2,
                              }],
                            }}
                            options={chartOpts as any}
                          />
                        )}
                      </div>
                    </Card>
                  )
                })}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
