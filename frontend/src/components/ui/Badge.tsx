import React from 'react'

type Color = 'green' | 'yellow' | 'red' | 'blue' | 'muted' | 'purple' | 'cyan'

const colorMap: Record<Color, React.CSSProperties> = {
  green:  { background: 'rgba(34,197,94,.15)',   color: 'var(--green)' },
  yellow: { background: 'rgba(245,158,11,.15)',  color: 'var(--yellow)' },
  red:    { background: 'rgba(239,68,68,.15)',   color: 'var(--red)' },
  blue:   { background: 'rgba(37,99,235,.15)',   color: 'var(--accent-lt)' },
  muted:  { background: 'rgba(139,148,158,.12)', color: 'var(--muted)' },
  purple: { background: 'rgba(168,85,247,.15)',  color: 'var(--purple)' },
  cyan:   { background: 'rgba(6,182,212,.15)',   color: 'var(--cyan)' },
}

export function Badge({ color, children }: { color: Color; children: React.ReactNode }) {
  return (
    <span style={{
      ...colorMap[color],
      display: 'inline-flex',
      alignItems: 'center',
      gap: 4,
      fontSize: 11,
      fontWeight: 600,
      padding: '2px 8px',
      borderRadius: 99,
    }}>
      {children}
    </span>
  )
}

export function Dot({ color }: { color: 'green' | 'yellow' | 'red' | 'muted' }) {
  const bg: Record<string, string> = {
    green:  'var(--green)',
    yellow: 'var(--yellow)',
    red:    'var(--red)',
    muted:  'var(--muted)',
  }
  const shadow = color === 'green' ? '0 0 6px var(--green)' : undefined
  return (
    <span style={{
      width: 7, height: 7, borderRadius: '50%',
      background: bg[color],
      display: 'inline-block',
      flexShrink: 0,
      boxShadow: shadow,
    }} />
  )
}

export function LiveBadge() {
  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', gap: 6,
      background: 'rgba(34,197,94,.15)', color: 'var(--green)',
      fontSize: 11, fontWeight: 600,
      padding: '3px 9px', borderRadius: 99,
      animation: 'pulse 2s infinite',
    }}>
      <Dot color="green" /> LIVE
    </span>
  )
}
