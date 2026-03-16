import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'
import { useToast } from '../contexts/ToastContext'
import { authApi } from '../api/auth'

export default function LoginPage() {
  const { login } = useAuth()
  const { toast } = useToast()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login(email, password)
      navigate('/')
    } catch (err: unknown) {
      const errData = (err as { response?: { data?: { error?: { message?: string }; message?: string } } })?.response?.data
      const message = errData?.error?.message ?? errData?.message ?? 'Invalid email or password'
      setError(message)
      toast('Login failed', 'error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--bg-primary)]" style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg-primary, var(--surface))' }}>
      <div style={{ width: '100%', maxWidth: 360, padding: 32, borderRadius: 16, border: '1px solid var(--border)', background: 'var(--surface)', boxShadow: '0 8px 32px rgba(0,0,0,.3)' }}>
        <div style={{ marginBottom: 32, textAlign: 'center' }}>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--accent-lt)', marginBottom: 4 }}>GreenLab IoT</div>
          <p style={{ fontSize: 13, color: 'var(--muted)' }}>Sign in to your account</p>
        </div>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div>
            <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--muted)', marginBottom: 5, textTransform: 'uppercase', letterSpacing: '.05em' }}>Email</label>
            <input
              type="email"
              required
              value={email}
              onChange={e => setEmail(e.target.value)}
              style={{ width: '100%', padding: '8px 12px', borderRadius: 8, background: 'var(--surface2)', border: '1px solid var(--border)', color: 'var(--text)', fontSize: 13, outline: 'none', boxSizing: 'border-box' }}
              placeholder="you@example.com"
            />
          </div>
          <div>
            <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--muted)', marginBottom: 5, textTransform: 'uppercase', letterSpacing: '.05em' }}>Password</label>
            <input
              type="password"
              required
              value={password}
              onChange={e => setPassword(e.target.value)}
              style={{ width: '100%', padding: '8px 12px', borderRadius: 8, background: 'var(--surface2)', border: '1px solid var(--border)', color: 'var(--text)', fontSize: 13, outline: 'none', boxSizing: 'border-box' }}
              placeholder="••••••••"
            />
          </div>
          {error && <p style={{ fontSize: 12, color: 'var(--red)', margin: 0 }}>{error}</p>}
          <button
            type="submit"
            disabled={loading}
            style={{ width: '100%', padding: '9px 0', borderRadius: 8, background: 'var(--accent)', color: '#fff', fontSize: 13, fontWeight: 600, border: 'none', cursor: loading ? 'default' : 'pointer', opacity: loading ? 0.6 : 1, transition: 'opacity .15s', marginTop: 4 }}
          >
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
          <p style={{ textAlign: 'center', fontSize: 12, color: 'var(--muted)', margin: 0 }}>
            Forgot password?{' '}
            <button type="button" style={{ color: 'var(--accent-lt)', background: 'none', border: 'none', cursor: 'pointer', fontSize: 12, padding: 0 }} onClick={() => {
              if (!email) { toast('Enter your email first', 'error'); return }
              authApi.forgotPassword(email)
                .then(() => toast('If that email exists, a reset link has been sent', 'info'))
                .catch(() => toast('Failed to send reset email', 'error'))
            }}>
              Reset it
            </button>
          </p>
          <p style={{ textAlign: 'center', fontSize: 12, color: 'var(--muted)', margin: 0 }}>
            Don't have an account?{' '}
            <button type="button" style={{ color: 'var(--accent-lt)', background: 'none', border: 'none', cursor: 'pointer', fontSize: 12, padding: 0 }} onClick={() => navigate('/signup')}>
              Sign up
            </button>
          </p>
        </form>
      </div>
    </div>
  )
}
