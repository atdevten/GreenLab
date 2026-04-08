import { useState, useEffect } from 'react'
import { Card } from '../components/ui/Card'
import { Btn } from '../components/ui/Button'
import { useToast } from '../contexts/ToastContext'
import { settingsApi } from '../api/settings'
import type { Org, ApiKey, WorkspaceApiKey } from '../types'
import { workspacesApi } from '../api/workspaces'

const navItems = ['General', 'Notifications', 'API Keys', 'Workspace Keys', 'Security', 'Billing']

function Toggle({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return (
    <div
      onClick={onChange}
      style={{
        position: 'relative', display: 'inline-block', width: 36, height: 20, cursor: 'pointer',
      }}
    >
      <div style={{
        position: 'absolute', inset: 0,
        background: checked ? 'var(--accent)' : 'var(--border)',
        borderRadius: 99, transition: 'var(--transition)',
      }} />
      <div style={{
        position: 'absolute', left: checked ? 18 : 2, top: 2,
        width: 16, height: 16, background: '#fff', borderRadius: '50%',
        transition: 'left var(--transition)',
      }} />
    </div>
  )
}

function SettingsRow({ label, desc, control }: { label: string; desc?: string; control: React.ReactNode }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      padding: '14px 0', borderBottom: '1px solid var(--border2)',
    }}>
      <div>
        <div style={{ fontSize: 13, fontWeight: 500 }}>{label}</div>
        {desc && <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 1 }}>{desc}</div>}
      </div>
      {control}
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  width: '100%', background: 'var(--surface2)', border: '1px solid var(--border)',
  borderRadius: 'var(--radius)', padding: '7px 12px', color: 'var(--text)',
  fontSize: 13, outline: 'none', marginTop: 6, boxSizing: 'border-box',
}

export function SettingsPage() {
  const { toast } = useToast()
  const [active, setActive] = useState('General')
  const [toggles, setToggles] = useState({ email: true, telegram: false, slack: true, twofa: false, publicApi: true })
  const toggle = (k: keyof typeof toggles) => setToggles(t => ({ ...t, [k]: !t[k] }))

  const orgId = localStorage.getItem('org_id') ?? ''

  // General tab state
  const [org, setOrg] = useState<Org | null>(null)
  const [orgName, setOrgName] = useState('')
  const [orgWebsite, setOrgWebsite] = useState('')
  const [orgLoading, setOrgLoading] = useState(false)
  const [orgSaving, setOrgSaving] = useState(false)

  // API Keys tab state
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([])
  const [keysLoading, setKeysLoading] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [creatingKey, setCreatingKey] = useState(false)
  const [showNewKeyForm, setShowNewKeyForm] = useState(false)
  const [newKeyValue, setNewKeyValue] = useState<string | null>(null)

  // Workspace API Keys tab state
  const [wsApiKeys, setWsApiKeys] = useState<WorkspaceApiKey[]>([])
  const [wsKeysLoading, setWsKeysLoading] = useState(false)
  const [newWsKeyName, setNewWsKeyName] = useState('')
  const [newWsKeyScope, setNewWsKeyScope] = useState<'read' | 'write'>('read')
  const [showNewWsKeyForm, setShowNewWsKeyForm] = useState(false)
  const [creatingWsKey, setCreatingWsKey] = useState(false)
  const [newWsKeyValue, setNewWsKeyValue] = useState<string | null>(null)
  const wsId = localStorage.getItem('workspace_id') ?? ''

  useEffect(() => {
    if (active === 'General' && orgId) {
      setOrgLoading(true)
      settingsApi.getOrg(orgId)
        .then(r => {
          setOrg(r.data)
          setOrgName(r.data.name)
          setOrgWebsite(r.data.website ?? '')
        })
        .catch(() => {})
        .finally(() => setOrgLoading(false))
    }
  }, [active, orgId])

  useEffect(() => {
    if (active === 'API Keys') {
      setKeysLoading(true)
      settingsApi.listApiKeys()
        .then(r => setApiKeys(r.data))
        .catch(() => {})
        .finally(() => setKeysLoading(false))
    }
  }, [active])

  useEffect(() => {
    if (active === 'Workspace Keys' && wsId) {
      setWsKeysLoading(true)
      workspacesApi.listApiKeys(wsId)
        .then(r => setWsApiKeys(r.data))
        .catch(() => {})
        .finally(() => setWsKeysLoading(false))
    }
  }, [active, wsId])

  function handleSaveOrg() {
    if (!orgId) return
    setOrgSaving(true)
    settingsApi.updateOrg(orgId, { name: orgName, website: orgWebsite })
      .then(r => {
        setOrg(r.data)
        toast('Organization settings saved')
      })
      .catch(() => toast('Failed to save settings', 'error'))
      .finally(() => setOrgSaving(false))
  }

  function handleCreateKey() {
    if (!newKeyName.trim()) return
    setCreatingKey(true)
    settingsApi.createApiKey({ name: newKeyName.trim(), scopes: [] })
      .then(r => {
        const { key, ...keyData } = r.data as ApiKey & { key: string }
        setApiKeys(prev => [keyData, ...prev])
        setNewKeyValue(key)
        setNewKeyName('')
        setShowNewKeyForm(false)
        toast('API key created')
      })
      .catch(() => toast('Failed to create API key', 'error'))
      .finally(() => setCreatingKey(false))
  }

  function handleRevokeKey(id: string) {
    settingsApi.revokeApiKey(id)
      .then(() => {
        setApiKeys(prev => prev.filter(k => k.id !== id))
        toast('API key revoked')
      })
      .catch(() => toast('Failed to revoke API key', 'error'))
  }

  function handleCreateWsKey() {
    if (!newWsKeyName.trim() || !wsId) return
    setCreatingWsKey(true)
    workspacesApi.createApiKey(wsId, { name: newWsKeyName.trim(), scope: newWsKeyScope })
      .then(r => {
        const { key, ...keyData } = r.data
        setWsApiKeys(prev => [keyData, ...prev])
        setNewWsKeyValue(key)
        setNewWsKeyName('')
        setShowNewWsKeyForm(false)
        toast('Workspace API key created')
      })
      .catch(() => toast('Failed to create workspace API key', 'error'))
      .finally(() => setCreatingWsKey(false))
  }

  function handleRevokeWsKey(keyId: string) {
    if (!wsId) return
    workspacesApi.revokeApiKey(wsId, keyId)
      .then(() => {
        setWsApiKeys(prev => prev.filter(k => k.id !== keyId))
        toast('Workspace API key revoked')
      })
      .catch(() => toast('Failed to revoke workspace API key', 'error'))
  }

  return (
    <div>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 20, fontWeight: 700 }}>Settings</h1>
        <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Manage your account, organization, and preferences</p>
      </div>

      <div className="settings-layout" style={{ display: 'grid', gridTemplateColumns: '180px 1fr', gap: 24, alignItems: 'start' }}>
        {/* Side nav */}
        <div className="settings-nav" style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          {navItems.map(n => (
            <div
              key={n}
              onClick={() => setActive(n)}
              className="settings-nav-item"
              style={{
                padding: '8px 12px',
                fontSize: 13, fontWeight: active === n ? 600 : 500,
                color: active === n ? 'var(--accent-lt)' : 'var(--muted)',
                background: active === n ? 'rgba(37,99,235,.18)' : 'transparent',
                whiteSpace: 'nowrap',
                borderLeft: `2px solid ${active === n ? 'var(--accent)' : 'transparent'}`,
                cursor: 'pointer',
              }}
            >{n}</div>
          ))}
        </div>

        {/* Content */}
        <div>
          {active === 'General' && (
            <>
              <Card style={{ marginBottom: 16 }}>
                <div style={{ marginBottom: 16 }}>
                  <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 4 }}>Organization</div>
                  <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 16 }}>Your organization profile and branding</div>
                </div>
                {orgLoading ? (
                  <div style={{ color: 'var(--muted)', fontSize: 13 }}>Loading…</div>
                ) : (
                  <>
                    <div style={{ marginBottom: 16 }}>
                      <label style={{ display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--muted)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '.05em' }}>Organization Name</label>
                      <input value={orgName} onChange={e => setOrgName(e.target.value)} style={{ width: '100%', background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }} />
                    </div>
                    <div style={{ marginBottom: 16 }}>
                      <label style={{ display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--muted)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '.05em' }}>Slug</label>
                      <input value={org?.slug ?? ''} disabled style={{ width: '100%', background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none', opacity: 0.6, cursor: 'not-allowed' }} />
                    </div>
                    <div style={{ marginBottom: 16 }}>
                      <label style={{ display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--muted)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '.05em' }}>Website</label>
                      <input value={orgWebsite} onChange={e => setOrgWebsite(e.target.value)} style={{ width: '100%', background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }} />
                    </div>
                    <Btn variant="primary" size="sm" onClick={handleSaveOrg} disabled={orgSaving}>{orgSaving ? 'Saving…' : 'Save Changes'}</Btn>
                  </>
                )}
              </Card>

              <Card>
                <div style={{ marginBottom: 16 }}>
                  <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 4 }}>Danger Zone</div>
                  <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 16 }}>Irreversible actions — proceed with caution</div>
                </div>
                <SettingsRow
                  label="Delete Organization"
                  desc="Permanently delete your organization and all data"
                  control={<Btn variant="danger" size="sm">Delete Organization</Btn>}
                />
              </Card>
            </>
          )}

          {active === 'Notifications' && (
            <Card>
              <div style={{ marginBottom: 16 }}>
                <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 4 }}>Notification Channels</div>
                <div style={{ fontSize: 12, color: 'var(--muted)' }}>Choose how you receive alert notifications</div>
              </div>

              <SettingsRow label="Email Alerts" desc="Receive alerts via email" control={<Toggle checked={toggles.email} onChange={() => toggle('email')} />} />
              {toggles.email && (
                <div style={{ padding: '8px 0 12px', borderBottom: '1px solid var(--border2)' }}>
                  <label style={{ fontSize: 11, fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em' }}>Email Address</label>
                  <input defaultValue="anh@greenlab.io" style={inputStyle} />
                </div>
              )}

              <SettingsRow label="Telegram Bot" desc="Send alerts to a Telegram chat" control={<Toggle checked={toggles.telegram} onChange={() => toggle('telegram')} />} />
              {toggles.telegram && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: '8px 0 12px', borderBottom: '1px solid var(--border2)' }}>
                  <div>
                    <label style={{ fontSize: 11, fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em' }}>Bot Token</label>
                    <input placeholder="12345678:AAF..." style={inputStyle} />
                  </div>
                  <div>
                    <label style={{ fontSize: 11, fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em' }}>Chat ID</label>
                    <input placeholder="-100123456789" style={inputStyle} />
                  </div>
                </div>
              )}

              <SettingsRow label="Slack Webhook" desc="Post alerts to a Slack channel" control={<Toggle checked={toggles.slack} onChange={() => toggle('slack')} />} />
              {toggles.slack && (
                <div style={{ padding: '8px 0 12px', borderBottom: '1px solid var(--border2)' }}>
                  <label style={{ fontSize: 11, fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em' }}>Webhook URL</label>
                  <input placeholder="https://hooks.slack.com/services/…" style={inputStyle} />
                </div>
              )}

              <div style={{ marginTop: 16 }}>
                <Btn variant="primary" size="sm" onClick={() => toast('Notification settings saved')}>Save Notification Settings</Btn>
              </div>
            </Card>
          )}

          {active === 'Security' && (
            <Card>
              <div style={{ marginBottom: 16 }}>
                <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 4 }}>Security</div>
                <div style={{ fontSize: 12, color: 'var(--muted)' }}>Authentication and access settings</div>
              </div>
              <SettingsRow label="Two-Factor Authentication" desc="Add an extra layer of security to your account" control={<Toggle checked={toggles.twofa}    onChange={() => toggle('twofa')}    />} />
              <SettingsRow label="Public API Access"         desc="Allow API access without authentication"        control={<Toggle checked={toggles.publicApi} onChange={() => toggle('publicApi')} />} />
              <SettingsRow label="Change Password"           desc="Update your account password"                  control={<Btn variant="ghost" size="sm">Change</Btn>} />
              <SettingsRow label="Active Sessions"           desc="2 active sessions"                             control={<Btn variant="danger" size="sm">Revoke All</Btn>} />
            </Card>
          )}

          {active === 'API Keys' && (
            <Card>
              <div style={{ marginBottom: 16 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div>
                    <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 4 }}>API Keys</div>
                    <div style={{ fontSize: 12, color: 'var(--muted)' }}>Manage personal access tokens</div>
                  </div>
                  <Btn variant="primary" size="sm" onClick={() => { setShowNewKeyForm(v => !v); setNewKeyValue(null) }}>+ New Key</Btn>
                </div>
              </div>

              {showNewKeyForm && (
                <div style={{ display: 'flex', gap: 8, marginBottom: 16, padding: 12, background: 'var(--surface2)', borderRadius: 'var(--radius)', border: '1px solid var(--border)' }}>
                  <input
                    value={newKeyName}
                    onChange={e => setNewKeyName(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && handleCreateKey()}
                    placeholder="Key name (e.g. Dashboard Token)"
                    style={{ flex: 1, background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '7px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }}
                    autoFocus
                  />
                  <Btn variant="primary" size="sm" onClick={handleCreateKey} disabled={creatingKey || !newKeyName.trim()}>
                    {creatingKey ? 'Creating…' : 'Create'}
                  </Btn>
                  <Btn variant="ghost" size="sm" onClick={() => { setShowNewKeyForm(false); setNewKeyName('') }}>Cancel</Btn>
                </div>
              )}

              {newKeyValue && (
                <div style={{ marginBottom: 16, padding: 12, background: 'rgba(34,197,94,.08)', border: '1px solid rgba(34,197,94,.3)', borderRadius: 'var(--radius)' }}>
                  <div style={{ fontSize: 12, color: 'var(--green)', fontWeight: 600, marginBottom: 6 }}>Key created — copy it now, it won't be shown again</div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <code style={{ flex: 1, fontFamily: 'monospace', fontSize: 12, color: 'var(--text)', wordBreak: 'break-all' }}>{newKeyValue}</code>
                    <Btn variant="ghost" size="sm" onClick={() => { navigator.clipboard.writeText(newKeyValue); toast('Copied!') }}>Copy</Btn>
                    <Btn variant="ghost" size="sm" onClick={() => setNewKeyValue(null)}>✕</Btn>
                  </div>
                </div>
              )}

              {keysLoading ? (
                <div style={{ color: 'var(--muted)', fontSize: 13, padding: '12px 0' }}>Loading…</div>
              ) : apiKeys.length === 0 ? (
                <div style={{ color: 'var(--muted)', fontSize: 13, padding: '12px 0' }}>No API keys yet.</div>
              ) : (
                apiKeys.map(k => (
                  <div key={k.id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '14px 0', borderBottom: '1px solid var(--border2)' }}>
                    <div style={{ flex: 1 }}>
                      <div style={{ fontWeight: 600, marginBottom: 2 }}>{k.name}</div>
                      <div style={{ fontSize: 12, color: 'var(--muted)' }}>
                        Prefix: <span style={{ fontFamily: 'monospace', color: 'var(--cyan)' }}>{k.key_prefix}…</span>
                        {k.scopes.length > 0 && <> · Scopes: {k.scopes.join(', ')}</>}
                        {' · '}Created {new Date(k.created_at).toLocaleDateString()}
                        {k.last_used && <> · Last used {new Date(k.last_used).toLocaleDateString()}</>}
                      </div>
                    </div>
                    <Btn variant="danger" size="sm" onClick={() => handleRevokeKey(k.id)}>Revoke</Btn>
                  </div>
                ))
              )}
            </Card>
          )}

          {active === 'Workspace Keys' && (
            <Card>
              <div style={{ marginBottom: 16 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <div>
                    <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 4 }}>Workspace API Keys</div>
                    <div style={{ fontSize: 12, color: 'var(--muted)' }}>Scoped keys for integrations and read-only access</div>
                  </div>
                  <Btn variant="primary" size="sm" onClick={() => { setShowNewWsKeyForm(v => !v); setNewWsKeyValue(null) }}>+ New Key</Btn>
                </div>
              </div>

              {showNewWsKeyForm && (
                <div style={{ display: 'flex', gap: 8, marginBottom: 16, padding: 12, background: 'var(--surface2)', borderRadius: 'var(--radius)', border: '1px solid var(--border)' }}>
                  <input
                    value={newWsKeyName}
                    onChange={e => setNewWsKeyName(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && handleCreateWsKey()}
                    placeholder="Key name (e.g. Integration Token)"
                    style={{ flex: 1, background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '7px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }}
                    autoFocus
                  />
                  <select
                    value={newWsKeyScope}
                    onChange={e => setNewWsKeyScope(e.target.value as 'read' | 'write')}
                    style={{ background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '7px 10px', color: 'var(--text)', fontSize: 13, outline: 'none', cursor: 'pointer' }}
                  >
                    <option value="read">read</option>
                    <option value="write">write</option>
                  </select>
                  <Btn variant="primary" size="sm" onClick={handleCreateWsKey} disabled={creatingWsKey || !newWsKeyName.trim()}>
                    {creatingWsKey ? 'Creating…' : 'Create'}
                  </Btn>
                  <Btn variant="ghost" size="sm" onClick={() => { setShowNewWsKeyForm(false); setNewWsKeyName('') }}>Cancel</Btn>
                </div>
              )}

              {newWsKeyValue && (
                <div style={{ marginBottom: 16, padding: 12, background: 'rgba(34,197,94,.08)', border: '1px solid rgba(34,197,94,.3)', borderRadius: 'var(--radius)' }}>
                  <div style={{ fontSize: 12, color: 'var(--green)', fontWeight: 600, marginBottom: 6 }}>Copy it now — not shown again</div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <code style={{ flex: 1, fontFamily: 'monospace', fontSize: 12, color: 'var(--text)', wordBreak: 'break-all' }}>{newWsKeyValue}</code>
                    <Btn variant="ghost" size="sm" onClick={() => { navigator.clipboard.writeText(newWsKeyValue!); toast('Copied!') }}>Copy</Btn>
                    <Btn variant="ghost" size="sm" onClick={() => setNewWsKeyValue(null)}>✕</Btn>
                  </div>
                </div>
              )}

              {wsKeysLoading ? (
                <div style={{ color: 'var(--muted)', fontSize: 13, padding: '12px 0' }}>Loading…</div>
              ) : wsApiKeys.length === 0 ? (
                <div style={{ color: 'var(--muted)', fontSize: 13, padding: '12px 0' }}>No workspace API keys yet.</div>
              ) : (
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                  <thead>
                    <tr style={{ color: 'var(--muted)', fontWeight: 600, fontSize: 11, textTransform: 'uppercase', letterSpacing: '.05em' }}>
                      <th style={{ textAlign: 'left', padding: '6px 0', borderBottom: '1px solid var(--border)' }}>Name</th>
                      <th style={{ textAlign: 'left', padding: '6px 8px', borderBottom: '1px solid var(--border)' }}>Scope</th>
                      <th style={{ textAlign: 'left', padding: '6px 8px', borderBottom: '1px solid var(--border)' }}>Prefix</th>
                      <th style={{ textAlign: 'left', padding: '6px 8px', borderBottom: '1px solid var(--border)' }}>Last Used</th>
                      <th style={{ textAlign: 'right', padding: '6px 0', borderBottom: '1px solid var(--border)' }}>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {wsApiKeys.map(k => (
                      <tr key={k.id}>
                        <td style={{ padding: '12px 0', borderBottom: '1px solid var(--border2)', fontWeight: 600 }}>{k.name}</td>
                        <td style={{ padding: '12px 8px', borderBottom: '1px solid var(--border2)' }}>
                          <span style={{ fontSize: 11, padding: '2px 8px', borderRadius: 99, fontWeight: 600, background: k.scope === 'write' ? 'rgba(234,179,8,.15)' : 'rgba(34,197,94,.12)', color: k.scope === 'write' ? 'var(--yellow)' : 'var(--green)' }}>{k.scope}</span>
                        </td>
                        <td style={{ padding: '12px 8px', borderBottom: '1px solid var(--border2)', fontFamily: 'monospace', color: 'var(--cyan)', fontSize: 12 }}>{k.key_prefix}…</td>
                        <td style={{ padding: '12px 8px', borderBottom: '1px solid var(--border2)', color: 'var(--muted)', fontSize: 12 }}>{k.last_used_at ? new Date(k.last_used_at).toLocaleDateString() : '—'}</td>
                        <td style={{ padding: '12px 0', borderBottom: '1px solid var(--border2)', textAlign: 'right' }}>
                          <Btn variant="danger" size="sm" onClick={() => handleRevokeWsKey(k.id)}>Revoke</Btn>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </Card>
          )}

          {active === 'Billing' && (
            <Card>
              <div style={{ marginBottom: 20 }}>
                <div style={{ fontSize: 14, fontWeight: 700, marginBottom: 4 }}>Current Plan</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                  <span style={{
                    display: 'inline-flex', alignItems: 'center', gap: 6,
                    background: 'linear-gradient(135deg, rgba(168,85,247,.2), rgba(37,99,235,.2))',
                    border: '1px solid rgba(168,85,247,.3)', color: 'var(--purple)',
                    fontSize: 11, fontWeight: 700, padding: '2px 10px', borderRadius: 99,
                  }}>⭐ Pro Plan</span>
                  <span style={{ fontSize: 13, color: 'var(--muted)' }}>$49/month · Renews Apr 1, 2026</span>
                </div>
              </div>
              {[
                { label: 'Monthly Readings', desc: '1,000,000 included',  used: 186_000,  max: 1_000_000, fmt: (v: number) => v >= 1000 ? `${Math.round(v/1000)}K` : String(v) },
                { label: 'Devices',          desc: '50 devices included', used: 12,       max: 50,        fmt: (v: number) => String(v) },
                { label: 'Workspaces',       desc: '10 included',         used: 3,        max: 10,        fmt: (v: number) => String(v) },
              ].map(row => (
                <SettingsRow
                  key={row.label}
                  label={row.label}
                  desc={row.desc}
                  control={
                    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 4, minWidth: 160 }}>
                      <span style={{ fontSize: 12, fontWeight: 700, color: row.used / row.max > 0.8 ? 'var(--yellow)' : 'var(--text)' }}>
                        {row.fmt(row.used)} / {row.fmt(row.max)}
                      </span>
                      <div style={{ width: 160, height: 5, background: 'var(--border)', borderRadius: 99 }}>
                        <div style={{
                          height: '100%', borderRadius: 99,
                          width: `${Math.min(100, (row.used / row.max) * 100).toFixed(1)}%`,
                          background: row.used / row.max > 0.8 ? 'var(--yellow)' : 'var(--accent)',
                          transition: 'width .4s ease',
                        }} />
                      </div>
                    </div>
                  }
                />
              ))}
              <div style={{ marginTop: 16 }}>
                <Btn variant="primary" size="sm">Upgrade Plan</Btn>
              </div>
            </Card>
          )}
        </div>
      </div>
    </div>
  )
}
