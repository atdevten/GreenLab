import { useState, useEffect } from 'react'
import { Line } from 'react-chartjs-2'
import { Card, CardTitle } from '../components/ui/Card'
import { Btn } from '../components/ui/Button'
import { useTheme } from '../hooks/useTheme'
import { queryApi } from '../api/query'
import { channelsApi } from '../api/channels'
import { fieldsApi } from '../api/fields'
import { workspacesApi } from '../api/workspaces'
import { useToast } from '../contexts/ToastContext'
import type { Channel } from '../types'

const presets = ['Last 1h', 'Last 6h', 'Last 24h', 'Last 7d', 'Last 30d', 'Custom']

const AGG_OPTS = ['Raw', 'Mean (5m)', 'Mean (1h)', 'Max', 'Min']


function presetToRange(preset: string): { start: string; end: string } {
  const now = new Date()
  const end = now.toISOString()
  const map: Record<string, number> = {
    'Last 1h': 1, 'Last 6h': 6, 'Last 24h': 24, 'Last 7d': 168, 'Last 30d': 720,
  }
  const hours = map[preset] ?? 24
  const start = new Date(now.getTime() - hours * 3600000).toISOString()
  return { start, end }
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = filename; a.click()
  URL.revokeObjectURL(url)
}

export function QueryPage() {
  const { toast } = useToast()
  const [preset,      setPreset]     = useState('Last 24h')
  const [custom,      setCustom]     = useState(false)
  const [customStart, setCustomStart] = useState('')
  const [customEnd,   setCustomEnd]   = useState('')
  const [channel,     setChannel]    = useState('')
  const [field,       setField]      = useState('')
  const [agg,         setAgg]        = useState('Raw')
  const [queryLoading, setQueryLoading] = useState(false)
  const [apiChartData, setApiChartData] = useState<{ labels: string[]; values: number[] } | null | 'error'>(null)

  const [channels, setChannels] = useState<Channel[]>([])
  const [channelsLoading, setChannelsLoading] = useState(true)
  const [fieldOpts, setFieldOpts] = useState<{ key: string; label: string; unit: string; color: string }[]>([])
  const [fieldsLoading, setFieldsLoading] = useState(false)

  const [result, setResult] = useState({
    channel: '',
    field:   '',
    unit:    '',
    color:   '#3b82f6',
    agg:     'Raw',
    range:   'Last 24h',
  })

  const { theme } = useTheme()
  const isDark    = theme === 'dark'
  const gridColor = isDark ? 'rgba(48,54,61,.5)'  : 'rgba(208,215,222,.8)'
  const tickColor = isDark ? '#8b949e'             : '#57606a'
  const chartOpts = {
    responsive: true, maintainAspectRatio: false,
    plugins: { legend: { display: false } },
    scales: {
      x: { grid: { color: gridColor }, ticks: { color: tickColor, font: { size: 11 } } },
      y: { grid: { color: gridColor }, ticks: { color: tickColor, font: { size: 11 } } },
    },
  }

  useEffect(() => {
    const orgId = localStorage.getItem('org_id') ?? ''
    if (!orgId) { setChannelsLoading(false); return }
    workspacesApi.list(orgId)
      .then(r => {
        const wsId = r.data[0]?.id
        if (!wsId) { setChannelsLoading(false); return }
        return channelsApi.list({ workspace_id: wsId })
          .then(cr => {
            setChannels(cr.data)
            if (cr.data.length > 0) setChannel(cr.data[0].id)
          })
          .finally(() => setChannelsLoading(false))
      })
      .catch(() => setChannelsLoading(false))
  }, [])

  useEffect(() => {
    if (!channel) return
    let cancelled = false
    setFieldsLoading(true)
    fieldsApi.list(channel).then(r => {
      if (cancelled) return
      const opts = r.data.map((f: { name: string; label: string; unit: string }) => ({
        key: f.name,
        label: f.label || f.name,
        unit: f.unit ?? '',
        color: '#3b82f6',
      }))
      setFieldOpts(opts)
      if (opts.length > 0) setField(opts[0].key)
    }).catch(() => { if (!cancelled) setFieldOpts([]) })
      .finally(() => { if (!cancelled) setFieldsLoading(false) })
    return () => { cancelled = true }
  }, [channel])

  const activeChannel = channels.find(c => c.id === channel)

  function handleChannelChange(chId: string) {
    setChannel(chId)
  }

  function runQuery() {
    const meta = fieldOpts.find(f => f.key === field) ?? fieldOpts[0]
    const range = preset === 'Custom'
      ? { start: customStart, end: customEnd }
      : presetToRange(preset)
    let aggValue: string | undefined
    let windowValue: string | undefined
    if (agg !== 'Raw') {
      aggValue = agg.toLowerCase().replace(/\s*\(.*\)/, '').trim()
      const windowMatch = agg.match(/\((\d+[smhd])\)/)
      if (windowMatch) windowValue = windowMatch[1]
    }

    setQueryLoading(true)
    queryApi.query({ channel_id: channel, field, ...range, aggregate: aggValue, window: windowValue })
      .then(r => {
        const data = r.data.data_points ?? []
        setApiChartData({
          labels: data.map((d: { timestamp: string }) => new Date(d.timestamp).toLocaleTimeString()),
          values: data.map((d: { value: number }) => d.value),
        })
        setResult({ channel: activeChannel?.name ?? channel, field, unit: meta?.unit ?? '', color: meta?.color ?? '#3b82f6', agg, range: preset })
      })
      .catch(() => {
        toast('Query failed', 'error')
        setApiChartData('error')
      })
      .finally(() => setQueryLoading(false))
  }

  function handleExportCsv() {
    const range = preset === 'Custom'
      ? { start: customStart, end: customEnd }
      : presetToRange(preset)
    queryApi.export({ channel_id: channel, field, ...range })
      .then(r => downloadBlob(r.data as Blob, 'query-export.csv'))
      .catch(() => toast('Export failed', 'error'))
  }

  const hasResult = apiChartData !== null
  const chartData = apiChartData === 'error' ? null : apiChartData
  const displayLabels = chartData?.labels ?? []
  const displayValues = chartData?.values ?? []
  const tableData = displayValues.slice(0, 8)
  const tableLabels = displayLabels.slice(0, 8)

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 20, fontWeight: 700 }}>Query</h1>
        <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Explore historical data across channels and fields</p>
      </div>

      {/* Date range picker */}
      <div style={{
        display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 6,
        background: 'var(--surface)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius-lg)', padding: '10px 14px', marginBottom: 16,
      }}>
        <span style={{ fontSize: 11, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.06em', color: 'var(--muted)', marginRight: 4 }}>
          RANGE
        </span>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
          {presets.map(p => (
            <button
              key={p}
              onClick={() => { setPreset(p); setCustom(p === 'Custom') }}
              style={{
                padding: '4px 10px', borderRadius: 'var(--radius)',
                fontSize: 12, fontWeight: 500,
                color: preset === p ? 'var(--accent-lt)' : 'var(--muted)',
                border: `1px solid ${preset === p ? 'var(--accent)' : 'var(--border)'}`,
                background: preset === p ? 'rgba(37,99,235,.18)' : 'var(--surface2)',
                cursor: 'pointer',
              }}
            >{p}</button>
          ))}
        </div>
        {custom && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginLeft: 8, flexWrap: 'wrap' }}>
            <input type="datetime-local" value={customStart} onChange={e => setCustomStart(e.target.value)} style={{
              background: 'var(--surface2)', border: '1px solid var(--border)',
              borderRadius: 'var(--radius)', padding: '5px 9px',
              color: 'var(--text)', fontSize: 12, outline: 'none', width: 168,
            }} />
            <span style={{ fontSize: 11, color: 'var(--muted)' }}>to</span>
            <input type="datetime-local" value={customEnd} onChange={e => setCustomEnd(e.target.value)} style={{
              background: 'var(--surface2)', border: '1px solid var(--border)',
              borderRadius: 'var(--radius)', padding: '5px 9px',
              color: 'var(--text)', fontSize: 12, outline: 'none', width: 168,
            }} />
          </div>
        )}
        <span style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--muted)' }}>
          Active: <strong style={{ color: 'var(--text)' }}>{preset}</strong>
        </span>
      </div>

      {/* Filters — controlled selects */}
      <div className="query-filters" style={{ marginBottom: 24 }}>
        <div>
          <label style={{ display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--muted)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '.05em' }}>Channel</label>
          <select
            value={channel}
            onChange={e => handleChannelChange(e.target.value)}
            style={{ width: '100%', background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }}
          >
            {channelsLoading ? (
              <option>Loading channels…</option>
            ) : channels.map(ch => (
              <option key={ch.id} value={ch.id}>{ch.name}</option>
            ))}
          </select>
        </div>
        <div>
          <label style={{ display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--muted)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '.05em' }}>Field</label>
          <select
            value={field}
            onChange={e => setField(e.target.value)}
            disabled={fieldsLoading}
            style={{ width: '100%', background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }}
          >
            {fieldsLoading ? (
              <option>Loading fields…</option>
            ) : fieldOpts.map(f => <option key={f.key} value={f.key}>{f.label}{f.unit ? ` (${f.unit})` : ''}</option>)}
          </select>
        </div>
        <div>
          <label style={{ display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--muted)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '.05em' }}>Aggregate</label>
          <select
            value={agg}
            onChange={e => setAgg(e.target.value)}
            style={{ width: '100%', background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }}
          >
            {AGG_OPTS.map(o => <option key={o}>{o}</option>)}
          </select>
        </div>
        <div style={{ display: 'flex', alignItems: 'flex-end' }}>
          <Btn variant="primary" onClick={runQuery} disabled={queryLoading || !channel || fieldsLoading || fieldOpts.length === 0}>
            {queryLoading ? 'Running…' : 'Run Query'}
          </Btn>
        </div>
      </div>

      {/* Chart — only shown after a query has been run */}
      {!channelsLoading && channels.length === 0 ? (
        <Card style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 280, gap: 8, color: 'var(--muted)' }}>
            <span style={{ fontSize: 32 }}>📡</span>
            <span style={{ fontSize: 14, fontWeight: 500 }}>No channels found</span>
            <span style={{ fontSize: 12 }}>Create a channel first to start querying data.</span>
          </div>
        </Card>
      ) : (
        <Card style={{ marginBottom: 16 }}>
          <CardTitle>
            {hasResult
              ? <>{result.field}{result.unit ? ` (${result.unit})` : ''} — {result.range}{result.agg !== 'Raw' && <span style={{ marginLeft: 8, fontSize: 11, color: 'var(--muted)', fontWeight: 400 }}>· {result.agg}</span>}</>
              : 'Chart'
            }
          </CardTitle>
          <div style={{ position: 'relative', height: 280 }}>
            {queryLoading ? (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 280 }}>
                <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
              </div>
            ) : !hasResult ? (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 280, gap: 8, color: 'var(--muted)' }}>
                <span style={{ fontSize: 32 }}>📊</span>
                <span style={{ fontSize: 14, fontWeight: 500 }}>No data yet</span>
                <span style={{ fontSize: 12 }}>Select a channel and field, then click Run Query.</span>
              </div>
            ) : (
              <Line
                data={{
                  labels: displayLabels,
                  datasets: [{
                    label: result.field,
                    data: displayValues,
                    borderColor: result.color,
                    backgroundColor: result.color + '20',
                    fill: true, tension: 0.4, pointRadius: 3,
                    pointBackgroundColor: result.color,
                  }],
                }}
                options={chartOpts as any}
              />
            )}
          </div>
        </Card>
      )}

      {/* Data table — only shown after a query has been run */}
      {hasResult && (
        <Card>
          <CardTitle>
            Raw Data
            <span style={{ marginLeft: 'auto' }}>
              <Btn variant="ghost" size="sm" onClick={handleExportCsv}>⬇ Export CSV</Btn>
            </span>
          </CardTitle>
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr>
                  {['Timestamp', 'Channel', 'Field', 'Value', 'Unit'].map(h => (
                    <th key={h} style={{
                      fontSize: 11, fontWeight: 600, textTransform: 'uppercase',
                      letterSpacing: '.06em', color: 'var(--muted)', textAlign: 'left',
                      padding: '10px 12px', borderBottom: '1px solid var(--border)',
                    }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {tableData.length === 0 ? (
                  <tr>
                    <td colSpan={5} style={{ padding: '20px 12px', textAlign: 'center', color: 'var(--muted)', fontSize: 13 }}>
                      No data points returned for this query.
                    </td>
                  </tr>
                ) : tableData.map((v, i) => (
                  <tr
                    key={i}
                    onMouseEnter={e => (e.currentTarget.style.background = 'var(--surface2)')}
                    onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                    style={{ transition: 'background .1s' }}
                  >
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)', fontFamily: 'monospace', fontSize: 12, color: 'var(--muted)' }}>
                      {tableLabels[i] ?? '—'}
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)', fontSize: 13 }}>{result.channel}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)', color: result.color, fontFamily: 'monospace', fontSize: 12 }}>{result.field}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)', fontWeight: 600 }}>{v}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)', color: 'var(--muted)' }}>{result.unit || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  )
}
