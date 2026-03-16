import { useState, useEffect } from 'react'
import { Badge } from '../components/ui/Badge'
import { Btn } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { notificationsApi } from '../api/notifications'
import { workspacesApi } from '../api/workspaces'
import { useToast } from '../contexts/ToastContext'
import type { Notification, NotifType } from '../types'

const typeColor: Record<NotifType, 'red' | 'yellow' | 'green' | 'blue'> = {
  critical: 'red', warning: 'yellow', resolved: 'green', info: 'blue',
}

const dotColor: Record<NotifType, string> = {
  critical: 'var(--red)', warning: 'var(--yellow)', resolved: 'var(--green)', info: 'var(--accent)',
}

type Filter = 'all' | NotifType

const FILTERS: { key: Filter; label: string }[] = [
  { key: 'all',      label: 'All' },
  { key: 'critical', label: 'Critical' },
  { key: 'warning',  label: 'Warning' },
  { key: 'resolved', label: 'Resolved' },
  { key: 'info',     label: 'Info' },
]

const Spinner = () => (
  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 160 }}>
    <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
  </div>
)

export function NotificationsPage() {
  const { toast } = useToast()
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [loading,       setLoading]       = useState(true)
  const [filter,        setFilter]        = useState<Filter>('all')

  useEffect(() => {
    const orgId = localStorage.getItem('org_id') ?? ''
    if (!orgId) { setLoading(false); return }
    workspacesApi.list(orgId)
      .then(r => {
        const wsId = r.data[0]?.id
        if (!wsId) { setLoading(false); return }
        return notificationsApi.list({ workspace_id: wsId })
          .then(nr => setNotifications(nr.data))
          .finally(() => setLoading(false))
      })
      .catch(() => {
        toast('Failed to load notifications', 'error')
        setLoading(false)
      })
  }, [])

  const unreadCount = notifications.filter(n => !n.read).length

  const visible = notifications.filter(n => filter === 'all' || n.type === filter)

  function markRead(id: string) {
    notificationsApi.markRead(id)
      .then(() => setNotifications(prev => prev.map(n => n.id === id ? { ...n, read: true } : n)))
      .catch(() => toast('Failed to mark as read', 'error'))
  }

  function markAllRead() {
    notificationsApi.markAllRead()
      .then(() => setNotifications(prev => prev.map(n => ({ ...n, read: true }))))
      .catch(() => toast('Failed to mark all as read', 'error'))
  }

  function formatTime(iso: string) {
    const diff = Date.now() - new Date(iso).getTime()
    const m = Math.floor(diff / 60000)
    if (m < 1) return 'just now'
    if (m < 60) return `${m} min ago`
    const h = Math.floor(m / 60)
    if (h < 24) return `${h} hr ago`
    return `${Math.floor(h / 24)}d ago`
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Notifications</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Alert event history and delivery status</p>
        </div>
        <div style={{ marginLeft: 'auto' }}>
          {unreadCount > 0 && (
            <Btn variant="ghost" size="sm" onClick={markAllRead}>✓ Mark all as read</Btn>
          )}
        </div>
      </div>

      {/* Stats */}
      <div className="rg4" style={{ display: 'grid', gridTemplateColumns: 'repeat(4,1fr)', gap: 12 }}>
        {[
          { label: 'Total',    value: notifications.length,                                        icon: '📋', color: 'var(--text)'   },
          { label: 'Unread',   value: unreadCount,                                                  icon: '🔵', color: 'var(--accent)' },
          { label: 'Critical', value: notifications.filter(n => n.type === 'critical').length,      icon: '🔴', color: 'var(--red)'    },
          { label: 'Warning',  value: notifications.filter(n => n.type === 'warning').length,       icon: '🟡', color: 'var(--yellow)' },
        ].map(s => (
          <div key={s.label} style={{
            background: 'var(--surface)', border: '1px solid var(--border)',
            borderRadius: 'var(--radius-lg)', padding: '12px 16px',
            display: 'flex', alignItems: 'center', gap: 10,
          }}>
            <span style={{ fontSize: 20 }}>{s.icon}</span>
            <div>
              <div style={{ fontSize: 20, fontWeight: 700, lineHeight: 1, color: s.color }}>{s.value}</div>
              <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 2 }}>{s.label}</div>
            </div>
          </div>
        ))}
      </div>

      {/* Filter tabs */}
      <div style={{ display: 'flex', gap: 4, borderBottom: '1px solid var(--border)', paddingBottom: 0 }}>
        {FILTERS.map(f => (
          <button
            key={f.key}
            onClick={() => setFilter(f.key)}
            style={{
              padding: '8px 16px', fontSize: 13, fontWeight: filter === f.key ? 600 : 400,
              color: filter === f.key ? 'var(--accent-lt)' : 'var(--muted)',
              borderBottom: `2px solid ${filter === f.key ? 'var(--accent)' : 'transparent'}`,
              background: 'transparent', cursor: 'pointer', transition: 'all .15s',
              marginBottom: -1,
            }}
          >
            {f.label}
            {f.key === 'all' && unreadCount > 0 && (
              <span style={{
                marginLeft: 6, fontSize: 10, fontWeight: 700, padding: '1px 6px',
                borderRadius: 99, background: 'var(--accent)', color: '#fff',
              }}>{unreadCount}</span>
            )}
          </button>
        ))}
      </div>

      {/* Events list */}
      <Card>
        {loading ? <Spinner /> : visible.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--muted)' }}>
            <div style={{ fontSize: 32, marginBottom: 12 }}>🔕</div>
            <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: 4 }}>No events</div>
            <div style={{ fontSize: 13 }}>No {filter} notifications found.</div>
          </div>
        ) : (
          <div>
            {visible.map((ev, i, arr) => {
              const isUnread = !ev.read
              return (
                <div
                  key={ev.id}
                  onClick={() => markRead(ev.id)}
                  style={{
                    display: 'flex', gap: 14, padding: '12px 0',
                    borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none',
                    cursor: 'pointer', borderRadius: 'var(--radius)',
                    background: isUnread ? 'rgba(37,99,235,.04)' : 'transparent',
                    transition: 'background .15s',
                  }}
                  onMouseEnter={e => { e.currentTarget.style.background = 'var(--surface2)' }}
                  onMouseLeave={e => { e.currentTarget.style.background = isUnread ? 'rgba(37,99,235,.04)' : 'transparent' }}
                >
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', width: 16, flexShrink: 0 }}>
                    <div style={{ position: 'relative', display: 'flex', justifyContent: 'center' }}>
                      <span style={{
                        width: 10, height: 10, borderRadius: '50%',
                        background: dotColor[ev.type], display: 'inline-block', marginTop: 4,
                      }} />
                      {isUnread && (
                        <span style={{
                          position: 'absolute', top: 2, right: -6,
                          width: 6, height: 6, borderRadius: '50%',
                          background: 'var(--accent)', border: '1.5px solid var(--surface)',
                        }} />
                      )}
                    </div>
                    {i < arr.length - 1 && (
                      <div style={{ width: 1, flex: 1, background: 'var(--border2)', margin: '4px 0' }} />
                    )}
                  </div>

                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 13, fontWeight: isUnread ? 700 : 600 }}>{ev.title}</div>
                    <div style={{ fontSize: 12, color: 'var(--muted)' }}>{ev.message}</div>
                  </div>

                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 4, flexShrink: 0 }}>
                    <Badge color={typeColor[ev.type]}>{ev.type}</Badge>
                    <span style={{ fontSize: 11, color: 'var(--muted)', whiteSpace: 'nowrap' }}>{formatTime(ev.created_at)}</span>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </Card>
    </div>
  )
}
