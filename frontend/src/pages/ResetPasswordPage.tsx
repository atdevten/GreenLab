import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { authApi } from '../api/auth'

export default function ResetPasswordPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') ?? ''

  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match')
      return
    }
    if (newPassword.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    setLoading(true)
    try {
      await authApi.resetPassword({ token, new_password: newPassword })
      setSuccess(true)
      setTimeout(() => navigate('/login'), 2000)
    } catch (err: unknown) {
      const errData = (err as { response?: { data?: { error?: { message?: string }; message?: string } } })?.response?.data
      setError(errData?.error?.message ?? errData?.message ?? 'Failed to reset password')
    } finally {
      setLoading(false)
    }
  }

  const cardStyle: React.CSSProperties = {
    width: '100%', maxWidth: 360, padding: 32,
    borderRadius: 16, border: '1px solid var(--border)',
    background: 'var(--surface)', boxShadow: '0 8px 32px rgba(0,0,0,.3)',
  }

  const inputStyle: React.CSSProperties = {
    width: '100%', padding: '8px 12px', borderRadius: 8,
    background: 'var(--surface2)', border: '1px solid var(--border)',
    color: 'var(--text)', fontSize: 13, outline: 'none', boxSizing: 'border-box',
  }

  const labelStyle: React.CSSProperties = {
    display: 'block', fontSize: 11, fontWeight: 600,
    color: 'var(--muted)', marginBottom: 5,
    textTransform: 'uppercase', letterSpacing: '.05em',
  }

  if (!token) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg-primary, var(--surface))' }}>
        <div style={cardStyle}>
          <p style={{ fontSize: 13, color: 'var(--red)', textAlign: 'center' }}>Invalid or missing reset token.</p>
          <button
            type="button"
            onClick={() => navigate('/login')}
            style={{ display: 'block', width: '100%', marginTop: 16, padding: '9px 0', borderRadius: 8, background: 'var(--accent)', color: '#fff', fontSize: 13, fontWeight: 600, border: 'none', cursor: 'pointer' }}
          >
            Back to Login
          </button>
        </div>
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg-primary, var(--surface))' }}>
      <div style={cardStyle}>
        <div style={{ marginBottom: 32, textAlign: 'center' }}>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--accent-lt)', marginBottom: 4 }}>GreenLab IoT</div>
          <p style={{ fontSize: 13, color: 'var(--muted)' }}>Set a new password</p>
        </div>

        {success ? (
          <div style={{ textAlign: 'center' }}>
            <p style={{ fontSize: 13, color: 'var(--green)', marginBottom: 8 }}>Password reset successfully.</p>
            <p style={{ fontSize: 12, color: 'var(--muted)' }}>Redirecting to login…</p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div>
              <label style={labelStyle}>New Password</label>
              <input
                type="password"
                required
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                style={inputStyle}
                placeholder="••••••••"
              />
            </div>
            <div>
              <label style={labelStyle}>Confirm Password</label>
              <input
                type="password"
                required
                value={confirmPassword}
                onChange={e => setConfirmPassword(e.target.value)}
                style={inputStyle}
                placeholder="••••••••"
              />
            </div>
            {error && <p style={{ fontSize: 12, color: 'var(--red)', margin: 0 }}>{error}</p>}
            <button
              type="submit"
              disabled={loading}
              style={{ width: '100%', padding: '9px 0', borderRadius: 8, background: 'var(--accent)', color: '#fff', fontSize: 13, fontWeight: 600, border: 'none', cursor: loading ? 'default' : 'pointer', opacity: loading ? 0.6 : 1, transition: 'opacity .15s', marginTop: 4 }}
            >
              {loading ? 'Resetting…' : 'Reset Password'}
            </button>
            <p style={{ textAlign: 'center', fontSize: 12, color: 'var(--muted)', margin: 0 }}>
              <button type="button" style={{ color: 'var(--accent-lt)', background: 'none', border: 'none', cursor: 'pointer', fontSize: 12, padding: 0 }} onClick={() => navigate('/login')}>
                Back to login
              </button>
            </p>
          </form>
        )}
      </div>
    </div>
  )
}
