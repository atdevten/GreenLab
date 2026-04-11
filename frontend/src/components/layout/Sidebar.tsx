import { useNavigate } from 'react-router-dom'
import type { Page } from '../../hooks/useNav'
import { useAuth } from '../../contexts/AuthContext'

interface NavItem {
  icon: string
  label: string
  page: Page
  badge?: number
  badgeColor?: 'green' | 'red'
}

const sections: { label: string; items: NavItem[] }[] = [
  {
    label: 'Overview',
    items: [
      { icon: '◈', label: 'Dashboard', page: 'dashboard' },
    ],
  },
  {
    label: 'Infrastructure',
    items: [
      { icon: '🗂️', label: 'Workspaces', page: 'workspaces' },
      { icon: '🔌', label: 'Devices', page: 'devices' },
      { icon: '📊', label: 'Channels', page: 'channels' },
    ],
  },
  {
    label: 'Data',
    items: [
      { icon: '📈', label: 'Live Data', page: 'realtime' },
      { icon: '🔍', label: 'Query', page: 'query' },
    ],
  },
  {
    label: 'Alerts',
    items: [
      { icon: '🔔', label: 'Alert Rules', page: 'alerts' },
      { icon: '📬', label: 'Notifications', page: 'notifications' },
    ],
  },
  {
    label: 'System',
    items: [
      { icon: '📋', label: 'Audit Log', page: 'audit' },
      { icon: '⚙️', label: 'Settings', page: 'settings' },
    ],
  },
]

interface Props {
  current: Page
  onNav: (p: Page) => void
  open: boolean
}

export function Sidebar({ current, onNav, open }: Props) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const initials = user ? ((user.first_name?.[0] ?? '') + (user.last_name?.[0] ?? '')).toUpperCase() || '??' : '??'

  async function handleLogout() {
    await logout()
    navigate('/login')
  }

  return (
    <aside
      className={`sidebar ${open ? 'sidebar-open' : 'sidebar-closed'}`}
      style={{
        width: 'var(--sidebar-w)',
        flexShrink: 0,
        background: 'var(--surface)',
        borderRight: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column',
        overflowY: 'auto',
      }}
    >
      {/* Logo */}
      <div style={{
        height: 'var(--header-h)',
        display: 'flex', alignItems: 'center', gap: 10,
        padding: '0 16px',
        borderBottom: '1px solid var(--border)',
        flexShrink: 0,
      }}>
        <svg width="28" height="28" viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg" style={{ flexShrink: 0 }}>
          <path d="M32 8 C20 14 12 22 12 34 C12 46 20 54 32 56 C44 50 52 42 52 30 C52 18 44 10 32 8 Z" fill="#052e16"/>
          <path d="M32 8 C20 14 12 22 12 34 C12 46 20 54 32 56 C44 50 52 42 52 30 C52 18 44 10 32 8 Z" fill="none" stroke="url(#sl-grad)" strokeWidth="1.5"/>
          <defs>
            <linearGradient id="sl-grad" x1="10" y1="8" x2="54" y2="56" gradientUnits="userSpaceOnUse">
              <stop offset="0%" stopColor="#4ade80"/>
              <stop offset="100%" stopColor="#15803d"/>
            </linearGradient>
          </defs>
          <line x1="32" y1="10" x2="32" y2="54" stroke="#22c55e" strokeWidth="1.5"/>
          <polyline points="32,22 22,22 22,17" stroke="#22c55e" strokeWidth="1.2" fill="none"/>
          <polyline points="32,32 19,32 19,27" stroke="#22c55e" strokeWidth="1.2" fill="none"/>
          <polyline points="32,42 22,42 22,47" stroke="#22c55e" strokeWidth="1.2" fill="none"/>
          <polyline points="32,22 42,22 42,17" stroke="#22c55e" strokeWidth="1.2" fill="none"/>
          <polyline points="32,32 45,32 45,27" stroke="#22c55e" strokeWidth="1.2" fill="none"/>
          <polyline points="32,42 42,42 42,47" stroke="#22c55e" strokeWidth="1.2" fill="none"/>
          <rect x="19.5" y="14.5" width="5" height="5" rx="1" fill="#22c55e"/>
          <rect x="16.5" y="24.5" width="5" height="5" rx="1" fill="#22c55e"/>
          <rect x="19.5" y="44.5" width="5" height="5" rx="1" fill="#22c55e"/>
          <rect x="39.5" y="14.5" width="5" height="5" rx="1" fill="#22c55e"/>
          <rect x="42.5" y="24.5" width="5" height="5" rx="1" fill="#22c55e"/>
          <rect x="39.5" y="44.5" width="5" height="5" rx="1" fill="#22c55e"/>
          <circle cx="32" cy="22" r="2" fill="#4ade80"/>
          <circle cx="32" cy="32" r="2" fill="#4ade80"/>
          <circle cx="32" cy="42" r="2" fill="#4ade80"/>
        </svg>
        <div>
          <div style={{ fontWeight: 700, fontSize: 15, letterSpacing: '-.3px' }}><span style={{ color: '#22c55e' }}>Green</span>Lab</div>
          <div style={{ fontSize: 10, color: 'var(--muted)' }}>IoT Platform</div>
        </div>
      </div>

      {/* Nav sections */}
      {sections.map(sec => (
        <div key={sec.label} style={{ padding: '12px 8px 4px' }}>
          <div style={{
            fontSize: 10, fontWeight: 600, letterSpacing: '.08em',
            textTransform: 'uppercase', color: 'var(--muted)',
            padding: '0 8px 6px',
          }}>{sec.label}</div>
          {sec.items.map(item => {
            const active = item.page === current
            return (
              <div
                key={item.page}
                onClick={() => onNav(item.page)}
                className={`nav-item${active ? ' nav-active' : ''}`}
              >
                <span style={{ fontSize: 15, width: 18, textAlign: 'center' }}>{item.icon}</span>
                <span style={{ flex: 1, fontSize: 13, fontWeight: active ? 600 : 500 }}>{item.label}</span>
                {item.badge != null && (
                  <span style={{
                    background: item.badgeColor === 'green' ? 'var(--green)' : 'var(--red)',
                    color: '#fff', fontSize: 10, fontWeight: 700,
                    padding: '1px 5px', borderRadius: 99, minWidth: 18, textAlign: 'center',
                  }}>{item.badge}</span>
                )}
              </div>
            )
          })}
        </div>
      ))}

      {/* User card */}
      <div style={{ marginTop: 'auto', borderTop: '1px solid var(--border)', padding: '12px 8px' }}>
        <div style={{
          display: 'flex', alignItems: 'center', gap: 10,
          padding: '6px 8px', borderRadius: 'var(--radius)',
        }}>
          <div style={{
            width: 28, height: 28, borderRadius: '50%',
            background: 'linear-gradient(135deg, var(--accent), var(--purple))',
            display: 'grid', placeItems: 'center',
            fontSize: 11, fontWeight: 700, color: '#fff', flexShrink: 0,
          }}>{initials}</div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 12, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{user ? `${user.first_name} ${user.last_name}`.trim() || user.email : '—'}</div>
            <div style={{ fontSize: 10, color: 'var(--muted)' }}>{user?.roles?.[0] ?? ''}</div>
          </div>
          <button
            onClick={handleLogout}
            title="Sign out"
            style={{ background: 'transparent', border: 'none', color: 'var(--muted)', cursor: 'pointer', fontSize: 14, padding: '2px 4px', lineHeight: 1, borderRadius: 'var(--radius)', transition: 'color .15s' }}
            onMouseEnter={e => (e.currentTarget.style.color = 'var(--red)')}
            onMouseLeave={e => (e.currentTarget.style.color = 'var(--muted)')}
          >⏻</button>
        </div>
      </div>
    </aside>
  )
}
