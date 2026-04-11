import { useEffect, useState } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { authApi } from '../api/auth'

export default function VerifyEmailPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const token = searchParams.get('token') ?? ''

  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    if (!token) {
      setStatus('error')
      setErrorMsg('Invalid or missing verification token.')
      return
    }
    authApi.verifyEmail(token)
      .then(() => setStatus('success'))
      .catch((err: unknown) => {
        const errData = (err as { response?: { data?: { error?: { message?: string }; message?: string } } })?.response?.data
        setErrorMsg(errData?.error?.message ?? errData?.message ?? 'Email verification failed.')
        setStatus('error')
      })
  }, [token])

  const cardStyle: React.CSSProperties = {
    width: '100%', maxWidth: 360, padding: 32,
    borderRadius: 16, border: '1px solid var(--border)',
    background: 'var(--surface)', boxShadow: '0 8px 32px rgba(0,0,0,.3)',
    textAlign: 'center',
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg-primary, var(--surface))' }}>
      <div style={cardStyle}>
        <div style={{ marginBottom: 24 }}>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--accent-lt)', marginBottom: 4 }}>GreenLab IoT</div>
          <p style={{ fontSize: 13, color: 'var(--muted)' }}>Email Verification</p>
        </div>

        {status === 'loading' && (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12 }}>
            <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
            <p style={{ fontSize: 13, color: 'var(--muted)' }}>Verifying your email…</p>
          </div>
        )}

        {status === 'success' && (
          <div>
            <p style={{ fontSize: 13, color: 'var(--green)', marginBottom: 16 }}>Your email has been verified successfully.</p>
            <button
              type="button"
              onClick={() => navigate('/login')}
              style={{ display: 'inline-block', padding: '9px 24px', borderRadius: 8, background: 'var(--accent)', color: '#fff', fontSize: 13, fontWeight: 600, border: 'none', cursor: 'pointer' }}
            >
              Go to Login
            </button>
          </div>
        )}

        {status === 'error' && (
          <div>
            <p style={{ fontSize: 13, color: 'var(--red)', marginBottom: 16 }}>{errorMsg}</p>
            <button
              type="button"
              onClick={() => navigate('/login')}
              style={{ display: 'inline-block', padding: '9px 24px', borderRadius: 8, background: 'var(--accent)', color: '#fff', fontSize: 13, fontWeight: 600, border: 'none', cursor: 'pointer' }}
            >
              Back to Login
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
