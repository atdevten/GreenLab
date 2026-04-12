import { useState, type ReactNode } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useNav } from './hooks/useNav'
import { Sidebar } from './components/layout/Sidebar'
import { Topbar } from './components/layout/Topbar'
import { DashboardPage }     from './pages/DashboardPage'
import { WorkspacesPage }    from './pages/WorkspacesPage'
import { DevicesPage }       from './pages/DevicesPage'
import { ChannelsPage }      from './pages/ChannelsPage'
import { LiveDataPage }      from './pages/LiveDataPage'
import { QueryPage }         from './pages/QueryPage'
import { AlertsPage }        from './pages/AlertsPage'
import { NotificationsPage } from './pages/NotificationsPage'
import { AuditLogPage }      from './pages/AuditLogPage'
import { SettingsPage }      from './pages/SettingsPage'
import LoginPage             from './pages/LoginPage'
import SignupPage            from './pages/SignupPage'
import ResetPasswordPage     from './pages/ResetPasswordPage'
import VerifyEmailPage       from './pages/VerifyEmailPage'
import { useAuth }           from './contexts/AuthContext'

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated, loading } = useAuth()
  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
        <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
      </div>
    )
  }
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

function AppShell() {
  const { page, setPage } = useNav()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <div style={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      {sidebarOpen && (
        <div
          onClick={() => setSidebarOpen(false)}
          className="mobile-overlay"
          style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', zIndex: 99, backdropFilter: 'blur(2px)' }}
        />
      )}

      <Sidebar
        current={page}
        onNav={p => { setPage(p); setSidebarOpen(false) }}
        open={sidebarOpen}
      />

      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
        <Topbar page={page} onMenuClick={() => setSidebarOpen(o => !o)} />
        <div className="main-pad" style={{ flex: 1, overflowY: 'auto', overflowX: 'hidden', padding: 24 }}>
          <Routes>
            <Route path="/"              element={<DashboardPage />} />
            <Route path="/workspaces"    element={<WorkspacesPage />} />
            <Route path="/devices"       element={<DevicesPage />} />
            <Route path="/channels"      element={<ChannelsPage />} />
            <Route path="/realtime"      element={<LiveDataPage />} />
            <Route path="/query"         element={<QueryPage />} />
            <Route path="/alerts"        element={<AlertsPage />} />
            <Route path="/notifications" element={<NotificationsPage />} />
            <Route path="/audit"         element={<AuditLogPage />} />
            <Route path="/settings"      element={<SettingsPage />} />
            <Route path="*"              element={<Navigate to="/" replace />} />
          </Routes>
        </div>
      </div>
    </div>
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/signup" element={<SignupPage />} />
      <Route path="/reset-password" element={<ResetPasswordPage />} />
      <Route path="/verify-email" element={<VerifyEmailPage />} />
      <Route path="/*" element={
        <ProtectedRoute>
          <AppShell />
        </ProtectedRoute>
      } />
    </Routes>
  )
}
