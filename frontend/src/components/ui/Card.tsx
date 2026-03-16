import React from 'react'

export function Card({ children, style, className }: {
  children: React.ReactNode
  style?: React.CSSProperties
  className?: string
}) {
  return (
    <div className={className} style={{
      background: 'var(--surface)',
      border: '1px solid var(--border)',
      borderRadius: 'var(--radius-lg)',
      padding: 20,
      ...style,
    }}>
      {children}
    </div>
  )
}

export function CardTitle({ children, style }: { children: React.ReactNode; style?: React.CSSProperties }) {
  return (
    <div style={{
      fontSize: 13, fontWeight: 600,
      marginBottom: 16,
      color: 'var(--muted)',
      textTransform: 'uppercase',
      letterSpacing: '.06em',
      display: 'flex', alignItems: 'center', gap: 8,
      ...style,
    }}>
      {children}
    </div>
  )
}

export function StatCard({ label, value, icon, change, changeUp }: {
  label: string
  value: React.ReactNode
  icon: string
  change?: string
  changeUp?: boolean
}) {
  return (
    <div style={{
      background: 'var(--surface)', border: '1px solid var(--border)',
      borderRadius: 'var(--radius-lg)', padding: 20,
      display: 'flex', flexDirection: 'column', gap: 8,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span style={{ fontSize: 12, color: 'var(--muted)', fontWeight: 500 }}>{label}</span>
        <span style={{ fontSize: 18, opacity: .7 }}>{icon}</span>
      </div>
      <div style={{ fontSize: 28, fontWeight: 700 }}>{value}</div>
      {change && (
        <div style={{
          fontSize: 12, display: 'flex', alignItems: 'center', gap: 4,
          color: changeUp ? 'var(--green)' : 'var(--red)',
        }}>
          {change}
        </div>
      )}
    </div>
  )
}
