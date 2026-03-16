import { useState, useEffect } from 'react'
import { Badge } from '../components/ui/Badge'
import { Btn } from '../components/ui/Button'
import { Card, CardTitle } from '../components/ui/Card'
import { auditApi } from '../api/audit'
import { workspacesApi } from '../api/workspaces'
import { useToast } from '../contexts/ToastContext'
import { useDebounce } from '../hooks/useDebounce'
import type { AuditEvent } from '../types'

const actionColor: Record<string, 'green' | 'blue' | 'yellow' | 'red' | 'muted'> = {
  'device.create':    'green',
  'apikey.rotate':    'yellow',
  'channel.update':   'blue',
  'alert.create':     'yellow',
  'user.login':       'muted',
  'workspace.update': 'blue',
  'device.block':     'red',
  'user.invite':      'green',
}

function avatarColor(name: string) {
  const colors = ['#2563eb', '#22c55e', '#a855f7', '#f59e0b', '#ef4444']
  let hash = 0
  for (const c of name) hash = (hash * 31 + c.charCodeAt(0)) & 0xffffffff
  return colors[Math.abs(hash) % colors.length]
}

function initials(name: string) {
  return name.split(' ').map(p => p[0]).join('').slice(0, 2).toUpperCase()
}

type Category = 'all' | 'device' | 'channel' | 'user' | 'api' | 'alert' | 'workspace'

const CATEGORIES: { key: Category; label: string }[] = [
  { key: 'all',       label: 'All'       },
  { key: 'device',    label: 'Device'    },
  { key: 'channel',   label: 'Channel'   },
  { key: 'user',      label: 'User'      },
  { key: 'api',       label: 'API Key'   },
  { key: 'alert',     label: 'Alert'     },
  { key: 'workspace', label: 'Workspace' },
]

const Spinner = () => (
  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 160 }}>
    <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
  </div>
)

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url; a.download = filename; a.click()
  URL.revokeObjectURL(url)
}

export function AuditLogPage() {
  const { toast } = useToast()
  const [logs,     setLogs]     = useState<AuditEvent[]>([])
  const [loading,  setLoading]  = useState(true)
  const [search,   setSearch]   = useState('')
  const [category, setCategory] = useState<Category>('all')
  const [wsId,     setWsId]     = useState('')

  const debouncedSearch = useDebounce(search, 300)

  // Resolve workspace once on mount
  useEffect(() => {
    const orgId = localStorage.getItem('org_id') ?? ''
    if (!orgId) { setLoading(false); return }
    workspacesApi.list(orgId)
      .then(r => {
        const id = r.data[0]?.id
        if (!id) { setLoading(false); return }
        setWsId(id)
      })
      .catch(() => { toast('Failed to load workspace', 'error'); setLoading(false) })
  }, [])

  // Fetch audit events whenever workspace, category, or search changes
  useEffect(() => {
    if (!wsId) return
    setLoading(true)
    auditApi.list({
      resource_type: category !== 'all' ? category : undefined,
      search: debouncedSearch || undefined,
    })
      .then(r => setLogs(r.data))
      .catch(() => toast('Failed to load audit log', 'error'))
      .finally(() => setLoading(false))
  }, [wsId, category, debouncedSearch])

  function handleExport() {
    if (!wsId) return
    auditApi.export(category !== 'all' ? { resource_type: category } : undefined)
      .then(r => downloadBlob(r.data as Blob, 'audit-log.csv'))
      .catch(() => toast('Export failed', 'error'))
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Audit Log</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Track all administrative actions across your organization</p>
        </div>
        <div style={{ marginLeft: 'auto' }}>
          <Btn variant="ghost" size="sm" onClick={handleExport}>⬇ Export</Btn>
        </div>
      </div>

      {/* Search + filter */}
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
        <input
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder="Search user, action, target, IP…"
          style={{
            flex: '1 1 220px', background: 'var(--surface)', border: '1px solid var(--border)',
            borderRadius: 'var(--radius)', padding: '7px 12px', color: 'var(--text)',
            fontSize: 13, outline: 'none',
          }}
        />
        <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
          {CATEGORIES.map(c => (
            <button
              key={c.key}
              onClick={() => setCategory(c.key)}
              style={{
                padding: '6px 12px', fontSize: 12, borderRadius: 'var(--radius)',
                fontWeight: category === c.key ? 600 : 400,
                color: category === c.key ? 'var(--accent-lt)' : 'var(--muted)',
                background: category === c.key ? 'rgba(37,99,235,.15)' : 'var(--surface)',
                border: `1px solid ${category === c.key ? 'rgba(37,99,235,.4)' : 'var(--border)'}`,
                cursor: 'pointer', transition: 'all .15s',
              }}
            >{c.label}</button>
          ))}
        </div>
      </div>

      {/* Table */}
      <Card>
        <CardTitle>
          Activity
          <span style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--muted)', fontWeight: 400 }}>
            {logs.length} entries
          </span>
        </CardTitle>
        {loading ? <Spinner /> : (
          <div style={{ overflowX: 'auto' }}>
            {logs.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--muted)' }}>
                <div style={{ fontSize: 28, marginBottom: 8 }}>🔍</div>
                <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: 4 }}>No results</div>
                <div style={{ fontSize: 13 }}>Try a different search or filter.</div>
              </div>
            ) : (
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr>
                    {['User', 'Action', 'Target', 'IP Address', 'Time'].map(h => (
                      <th key={h} style={{
                        fontSize: 11, fontWeight: 600, textTransform: 'uppercase',
                        letterSpacing: '.06em', color: 'var(--muted)', textAlign: 'left',
                        padding: '10px 12px', borderBottom: '1px solid var(--border)',
                      }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {logs.map(log => (
                    <tr
                      key={log.id}
                      onMouseEnter={e => (e.currentTarget.style.background = 'var(--surface2)')}
                      onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                      style={{ transition: 'background .1s' }}
                    >
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <div style={{
                            width: 28, height: 28, borderRadius: '50%', flexShrink: 0,
                            background: avatarColor(log.user_name),
                            display: 'grid', placeItems: 'center',
                            fontSize: 10, fontWeight: 700, color: '#fff',
                          }}>{initials(log.user_name)}</div>
                          <span style={{ fontWeight: 600, fontSize: 13 }}>{log.user_name}</span>
                        </div>
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)' }}>
                        <Badge color={actionColor[log.action] ?? 'muted'}>
                          {log.action}
                        </Badge>
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)', color: 'var(--muted)', fontSize: 13 }}>{log.target}</td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)', fontFamily: 'monospace', fontSize: 12, color: 'var(--muted)' }}>{log.ip}</td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--border2)', fontFamily: 'monospace', fontSize: 12, color: 'var(--muted)', whiteSpace: 'nowrap' }}>{log.created_at}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}
      </Card>
    </div>
  )
}
