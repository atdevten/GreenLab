import React from 'react'

type Variant = 'primary' | 'ghost' | 'danger'
type Size = 'sm' | 'md'

const variantStyle: Record<Variant, React.CSSProperties> = {
  primary: {
    background: 'var(--accent)', color: '#fff',
    border: '1px solid var(--accent)',
  },
  ghost: {
    background: 'transparent', color: 'var(--muted)',
    border: '1px solid var(--border)',
  },
  danger: {
    background: 'transparent', color: 'var(--red)',
    border: '1px solid rgba(239,68,68,.4)',
  },
}

const sizeStyle: Record<Size, React.CSSProperties> = {
  sm: { padding: '4px 10px', fontSize: 12 },
  md: { padding: '7px 14px', fontSize: 13 },
}

interface Props extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  icon?: boolean
}

export function Btn({ variant = 'ghost', size = 'md', icon, style, ...props }: Props) {
  return (
    <button
      {...props}
      style={{
        display: 'inline-flex', alignItems: 'center', gap: 6,
        borderRadius: 'var(--radius)',
        fontWeight: 500,
        transition: 'all var(--transition)',
        cursor: 'pointer',
        ...variantStyle[variant],
        ...sizeStyle[size],
        ...(icon ? { padding: 7 } : {}),
        ...style,
      }}
    />
  )
}
