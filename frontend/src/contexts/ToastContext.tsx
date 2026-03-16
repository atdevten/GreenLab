import { createContext, useContext, useState, useCallback } from 'react'

type ToastType = 'success' | 'error' | 'info'

interface ToastItem {
  id: number
  message: string
  type: ToastType
}

interface ToastCtx {
  toast(message: string, type?: ToastType): void
}

const ToastContext = createContext<ToastCtx>({ toast: () => {} })

let _nextId = 0

const ICON: Record<ToastType, string>   = { success: '✓', error: '✕', info: 'ℹ' }
const COLOR: Record<ToastType, string>  = { success: 'var(--green)', error: 'var(--red)', info: 'var(--accent)' }

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const toast = useCallback((message: string, type: ToastType = 'success') => {
    const id = ++_nextId
    setToasts(prev => [...prev, { id, message, type }])
    setTimeout(() => setToasts(prev => prev.filter(t => t.id !== id)), 3200)
  }, [])

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div style={{
        position: 'fixed', bottom: 24, right: 24, zIndex: 9999,
        display: 'flex', flexDirection: 'column-reverse', gap: 8,
        alignItems: 'flex-end', pointerEvents: 'none',
      }}>
        {toasts.map(t => (
          <div key={t.id} style={{
            display: 'flex', alignItems: 'center', gap: 10,
            background: 'var(--surface)', border: '1px solid var(--border)',
            borderLeft: `3px solid ${COLOR[t.type]}`,
            borderRadius: 'var(--radius-lg)', padding: '10px 16px',
            fontSize: 13, fontWeight: 500,
            boxShadow: '0 4px 20px rgba(0,0,0,.3)',
            animation: 'toast-in 200ms ease forwards',
            pointerEvents: 'auto', maxWidth: 340,
          }}>
            <span style={{ color: COLOR[t.type], fontWeight: 700, fontSize: 15 }}>{ICON[t.type]}</span>
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  return useContext(ToastContext)
}
