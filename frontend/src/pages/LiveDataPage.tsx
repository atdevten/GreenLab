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
import { workspacesApi } from '../api/workspaces'
import type { Channel } from '../types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Filler, Tooltip, Legend)

type FieldValues = Record<string, { value: number; trend: string }>
type ChartHistory = Record<string, number[]>

const HISTORY_LEN = 20
const labels = Array.from({ length: HISTORY_LEN }, (_, i) => `${i * 3}s`)
const genData = (base: number) => Array.from({ length: HISTORY_LEN }, () => base + (Math.random() - 0.5) * base * 0.1)

const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

export function LiveDataPage() {
  const [active,    setActive]    = useState(0)
  const [fieldVals, setFieldVals] = useState<FieldValues>({})
  const [history,   setHistory]   = useState<ChartHistory>({})
  const wsRef = useRef<WebSocket | null>(null)
  const { theme } = useTheme()

  const [apiChannels, setApiChannels]       = useState<Channel[]>([])
  const [channelsLoading, setChannelsLoading] = useState(true)

  const isDark    = theme === 'dark'
  const gridColor = isDark ? 'rgba(48,54,61,.5)'  : 'rgba(208,215,222,.8)'
  const tickColor = isDark ? '#8b949e'             : '#57606a'

  const chartOpts = {
    responsive: true,
    maintainAspectRatio: false,
    animation: false as const,
    plugins: { legend: { display: false } },
    scales: {
      x: { grid: { color: gridColor }, ticks: { color: tickColor, font: { size: 10 } } },
      y: { grid: { color: gridColor }, ticks: { color: tickColor, font: { size: 10 } } },
    },
  }

  // Fetch workspace, then channels for that workspace
  useEffect(() => {
    const orgId = localStorage.getItem('org_id') ?? ''
    if (!orgId) { setChannelsLoading(false); return }
    workspacesApi.list(orgId)
      .then(r => {
        const wsId = r.data[0]?.id
        if (!wsId) { setChannelsLoading(false); return }
        return channelsApi.list({ workspace_id: wsId })
          .then(cr => setApiChannels(cr.data))
          .finally(() => setChannelsLoading(false))
      })
      .catch(() => setChannelsLoading(false))
  }, [])

  // Connect WebSocket whenever the active channel changes
  useEffect(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }

    const channelId = apiChannels[active]?.id ?? ''
    if (!channelId) return

    const token = localStorage.getItem('access_token') ?? ''
    const wsUrl = `${BASE_URL.replace(/^http/, 'ws')}/api/v1/ws?channel_id=${channelId}&token=${token}`

    try {
      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onmessage = (evt) => {
        try {
          const msg = JSON.parse(evt.data) as Record<string, number>
          setFieldVals(prev => {
            const next = { ...prev }
            for (const [key, val] of Object.entries(msg)) {
              const prevVal = prev[key]?.value ?? val
              const diff = val - prevVal
              next[key] = {
                value: val,
                trend: diff >= 0 ? `+${diff.toFixed(1)}` : diff.toFixed(1),
              }
            }
            return next
          })
          setHistory(prev => {
            const next = { ...prev }
            for (const [key, val] of Object.entries(msg)) {
              const arr = prev[key] ?? genData(val)
              next[key] = [...arr.slice(1), val]
            }
            return next
          })
        } catch {}
      }

      ws.onerror = () => {
        wsRef.current = null
      }
    } catch {
      wsRef.current = null
    }

    return () => {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [active, apiChannels])

  const fieldData = apiChannels[active]?.fields ?? []

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Live Data</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Real-time streaming from your channels</p>
        </div>
        <LiveBadge />
      </div>

      <div className="live-layout" style={{ height: 'calc(100vh - 180px)' }}>
        {/* Channel list */}
        <div style={{ overflowY: 'auto' }}>
          {channelsLoading ? (
            <div style={{ color: 'var(--muted)', fontSize: 13, padding: 12 }}>Loading channels…</div>
          ) : apiChannels.map((ch, i) => (
            <div
              key={ch.id}
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
                  background: 'var(--green)',
                  boxShadow: '0 0 6px var(--green)',
                }} />
                <div style={{ fontSize: 13, fontWeight: 600 }}>{ch.name}</div>
              </div>
              <div style={{ fontSize: 11, color: 'var(--muted)', paddingLeft: 15 }}>{ch.device_id}</div>
              <div style={{ fontSize: 11, color: 'var(--muted)', paddingLeft: 15, marginTop: 4 }}>
                {ch.fields?.length ?? 0} fields
              </div>
            </div>
          ))}
        </div>

        {/* Chart section */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, overflowY: 'auto' }}>
          {/* Field ticker */}
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
            {fieldData.map(f => {
              const live = fieldVals[f.key]
              return (
                <div key={f.key} style={{
                  background: 'var(--surface)', border: '1px solid var(--border)',
                  borderRadius: 'var(--radius-lg)', padding: '14px 18px',
                  flex: 1, minWidth: 120,
                }}>
                  <div style={{ fontSize: 11, color: 'var(--muted)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '.06em', marginBottom: 4 }}>
                    {f.name || f.key}
                  </div>
                  <div style={{ fontSize: 24, fontWeight: 700 }}>
                    {live ? live.value.toFixed(1) : '—'} <span style={{ fontSize: 13, color: 'var(--muted)' }}>{f.unit ?? ''}</span>
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
          {fieldData.map(f => (
            <Card key={`${active}-${f.key}`}>
              <CardTitle>{f.name || f.key}{f.unit ? ` (${f.unit})` : ''}</CardTitle>
              <div style={{ position: 'relative', height: 180 }}>
                <Line
                  data={{
                    labels,
                    datasets: [{
                      data: history[f.key] ?? genData(50),
                      borderColor: f.color ?? '#3b82f6',
                      backgroundColor: (f.color ?? '#3b82f6') + '20',
                      fill: true,
                      tension: 0.4,
                      pointRadius: 2,
                    }],
                  }}
                  options={chartOpts as any}
                />
              </div>
            </Card>
          ))}
        </div>
      </div>
    </div>
  )
}
