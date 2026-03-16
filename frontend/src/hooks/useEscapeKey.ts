import { useEffect } from 'react'

export function useEscapeKey(onEscape: () => void, active = true) {
  useEffect(() => {
    if (!active) return
    function handler(e: KeyboardEvent) {
      if (e.key === 'Escape') onEscape()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onEscape, active])
}
