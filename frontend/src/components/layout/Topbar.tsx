import { useNavigate } from 'react-router-dom'
import type { Page } from '../../hooks/useNav'
import { useTheme } from '../../hooks/useTheme'

const titles: Record<Page, string> = {
  dashboard:     'Dashboard',
  workspaces:    'Workspaces',
  devices:       'Devices',
  channels:      'Channels',
  realtime:      'Live Data',
  query:         'Query',
  alerts:        'Alert Rules',
  notifications: 'Notifications',
  audit:         'Audit Log',
  settings:      'Settings',
}

interface Props {
  page: Page
  onMenuClick: () => void
}

export function Topbar({ page, onMenuClick }: Props) {
  const { theme, toggleTheme } = useTheme()
  const navigate = useNavigate()

  return (
    <div style={{
      height: 'var(--header-h)',
      borderBottom: '1px solid var(--border)',
      display: 'flex', alignItems: 'center', gap: 12,
      padding: '0 20px',
      background: 'var(--surface)',
      flexShrink: 0,
    }}>
      {/* Hamburger (always rendered, hidden via CSS on desktop via inline logic) */}
      <button
        onClick={onMenuClick}
        style={{
          display: 'flex', flexDirection: 'column', justifyContent: 'center',
          alignItems: 'center', gap: 5,
          width: 36, height: 36, borderRadius: 'var(--radius)',
          cursor: 'pointer', flexShrink: 0,
        }}
        aria-label="Toggle menu"
      >
        {[0,1,2].map(i => (
          <span key={i} style={{
            display: 'block', width: 18, height: 2,
            background: 'var(--muted)', borderRadius: 99,
          }} />
        ))}
      </button>

      <span style={{ fontSize: 13, color: 'var(--muted)' }}>
        {titles[page]}
      </span>

      <div style={{ flex: 1 }} />

      <div className="hide-sm" style={{
        display: 'flex', alignItems: 'center', gap: 8,
        background: 'var(--surface2)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius)', padding: '6px 12px',
        width: 200, color: 'var(--muted)', fontSize: 13,
      }}>
        🔍 Search...
      </div>

      <button
        onClick={() => navigate('/notifications')}
        title="Notifications"
        style={{
          width: 32, height: 32, borderRadius: 'var(--radius)',
          display: 'grid', placeItems: 'center',
          color: page === 'notifications' ? 'var(--accent-lt)' : 'var(--muted)',
          fontSize: 16, position: 'relative',
          background: page === 'notifications' ? 'rgba(37,99,235,.12)' : 'transparent',
        }}
      >
        🔔
        <span style={{
          position: 'absolute', top: 4, right: 4,
          width: 7, height: 7, borderRadius: '50%',
          background: 'var(--red)', border: '1.5px solid var(--surface)',
        }} />
      </button>

      <button
        onClick={toggleTheme}
        title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
        style={{
          width: 32, height: 32, borderRadius: 'var(--radius)',
          display: 'grid', placeItems: 'center',
          color: 'var(--muted)', fontSize: 16,
        }}
      >
        {theme === 'dark' ? '☀️' : '🌙'}
      </button>

      <button style={{
        width: 32, height: 32, borderRadius: 'var(--radius)',
        display: 'grid', placeItems: 'center',
        color: 'var(--muted)', fontSize: 16,
      }}>
        ❓
      </button>
    </div>
  )
}
