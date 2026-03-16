import { useState, useEffect } from 'react'
import { Card, CardTitle } from '../components/ui/Card'
import { Badge, Dot } from '../components/ui/Badge'
import { Btn } from '../components/ui/Button'
import { workspacesApi } from '../api/workspaces'
import { useToast } from '../contexts/ToastContext'
import type { Workspace as ApiWorkspace, WorkspaceMember as ApiMember } from '../types'

// ── Types ──────────────────────────────────────────────────────────────────

interface Member {
  user_id?: string
  initials: string
  gradient: string
  name: string
  email: string
  wss: string[]
  role: 'admin' | 'operator' | 'viewer'
  roleColor: 'purple' | 'blue' | 'muted'
  status: 'active' | 'pending'
  statusColor: 'green' | 'yellow'
}

interface Workspace {
  id: string
  icon: string
  name: string
  slug: string
  desc: string
  plan: string
  planColor: 'purple' | 'blue' | 'muted'
  devices: number
  channels: number
  members: number
  used: string
  quota: string
  pct: number
  barColor?: string
}

interface WsDevice {
  icon: string
  name: string
  id: string
  status: 'Active' | 'Warning' | 'Inactive' | 'Blocked'
  channels: number
  reads: string
  last: string
  workspace: string
}

// ── Seed data ──────────────────────────────────────────────────────────────

const ALL_DEVICES: WsDevice[] = [
  { icon: '🌡️', name: 'Greenhouse Sensor A', id: 'dev_4f2a91c3', status: 'Active',   channels: 4, reads: '42K', last: '12s ago', workspace: 'Default Workspace' },
  { icon: '💧', name: 'Water Quality Probe',  id: 'dev_2a9f10d4', status: 'Warning',  channels: 5, reads: '19K', last: '2m ago',  workspace: 'Default Workspace' },
  { icon: '🌬️', name: 'Air Monitor',          id: 'dev_8b5c44e2', status: 'Active',   channels: 6, reads: '27K', last: '5s ago',  workspace: 'Default Workspace' },
  { icon: '☀️', name: 'Solar Tracker',        id: 'dev_9e2b55a3', status: 'Blocked',  channels: 2, reads: '0',   last: 'Never',   workspace: 'Default Workspace' },
  { icon: '🌾', name: 'Farm Node B',          id: 'dev_7c3e22b1', status: 'Active',   channels: 3, reads: '38K', last: '8s ago',  workspace: 'Farm Project' },
  { icon: '🔬', name: 'R&D Lab Node',         id: 'dev_1d7a88f5', status: 'Inactive', channels: 2, reads: '620', last: '3h ago',  workspace: 'R&D Lab' },
]

const statusColor: Record<string, 'green' | 'yellow' | 'red' | 'muted'> = {
  Active: 'green', Warning: 'yellow', Inactive: 'muted', Blocked: 'red',
}

// ── Shared styles ──────────────────────────────────────────────────────────

const labelStyle: React.CSSProperties = {
  display: 'block', fontSize: 12, fontWeight: 600,
  color: 'var(--muted)', marginBottom: 6,
  textTransform: 'uppercase', letterSpacing: '.05em',
}

const inputStyle: React.CSSProperties = {
  width: '100%', background: 'var(--surface2)',
  border: '1px solid var(--border)', borderRadius: 'var(--radius)',
  padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none',
  boxSizing: 'border-box',
}

// ── Helpers ────────────────────────────────────────────────────────────────

function slugify(name: string) {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

// ── View Devices Drawer ────────────────────────────────────────────────────

function ViewDevicesDrawer({ workspace, onClose }: { workspace: Workspace | null; onClose(): void }) {
  const [devices, setDevices] = useState<WsDevice[]>([])
  useEffect(() => {
    if (!workspace) return
    workspacesApi.listDevices(workspace.id)
      .then(r => {
        const apiDevices = (r.data as any[]) ?? []
        setDevices(apiDevices.map((d: any) => ({
          icon: d.icon ?? '📡',
          name: d.name,
          id: d.id,
          status: d.status === 'active' ? 'Active' : d.status === 'blocked' ? 'Blocked' : d.status === 'inactive' ? 'Inactive' : 'Inactive',
          channels: d.channel_count ?? 0,
          reads: String(d.reads_24h ?? 0),
          last: d.last_seen ? new Date(d.last_seen).toLocaleString() : 'Never',
          workspace: workspace.name,
        })))
      })
      .catch(() => setDevices([]))
  }, [workspace?.id])
  if (!workspace) return null

  return (
    <>
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.55)', backdropFilter: 'blur(3px)', zIndex: 200 }} />
      <div style={{
        position: 'fixed', top: 0, right: 0, bottom: 0, width: 500,
        background: 'var(--surface)', borderLeft: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column', zIndex: 201, overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>{workspace.icon}</span>
          <div style={{ flex: 1 }}>
            <h2 style={{ fontSize: 15, fontWeight: 700 }}>{workspace.name}</h2>
            <div style={{ fontSize: 11, color: 'var(--muted)', fontFamily: 'monospace' }}>{workspace.slug}</div>
          </div>
          <button onClick={onClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        {/* Stats row */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 10, padding: '16px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          {[
            { label: 'Devices',  value: workspace.devices },
            { label: 'Channels', value: workspace.channels },
            { label: 'Members',  value: workspace.members },
          ].map(s => (
            <div key={s.label} style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '10px 14px' }}>
              <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 2 }}>{s.label}</div>
              <div style={{ fontSize: 18, fontWeight: 700 }}>{s.value}</div>
            </div>
          ))}
        </div>

        {/* Device list */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '16px 20px' }}>
          {devices.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--muted)' }}>
              <div style={{ fontSize: 32, marginBottom: 12 }}>📭</div>
              <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: 4 }}>No devices yet</div>
              <div style={{ fontSize: 13 }}>Register a device and assign it to this workspace.</div>
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {devices.map(d => (
                <div key={d.id} style={{
                  display: 'flex', alignItems: 'center', gap: 12,
                  background: 'var(--surface2)', border: '1px solid var(--border)',
                  borderRadius: 'var(--radius-lg)', padding: '14px 16px',
                }}>
                  <span style={{ fontSize: 22, flexShrink: 0 }}>{d.icon}</span>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 600, fontSize: 14 }}>{d.name}</div>
                    <div style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--muted)' }}>{d.id}</div>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 6 }}>
                      <span style={{ fontSize: 11, color: 'var(--muted)' }}>
                        <strong style={{ color: 'var(--text)' }}>{d.channels}</strong> channels
                      </span>
                      <span style={{ fontSize: 11, color: 'var(--muted)' }}>
                        <strong style={{ color: 'var(--text)' }}>{d.reads}</strong> reads/24h
                      </span>
                      <span style={{ fontSize: 11, color: 'var(--muted)' }}>
                        Last: <strong style={{ color: 'var(--text)' }}>{d.last}</strong>
                      </span>
                    </div>
                  </div>
                  <Badge color={statusColor[d.status]}>
                    {d.status === 'Active' && <Dot color="green" />}
                    {d.status}
                  </Badge>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </>
  )
}

// ── Edit Workspace Drawer ──────────────────────────────────────────────────

const roleColor: Record<string, 'purple' | 'blue' | 'muted'> = { admin: 'purple', operator: 'blue', viewer: 'muted' }

const initialMembers: Member[] = [
  { initials: 'AH', gradient: 'linear-gradient(135deg,var(--accent),var(--purple))', name: 'Anh Hoang', email: 'anh.hoang@greenlab.io', wss: ['Default','Farm','R&D'], role: 'admin',    roleColor: 'purple', status: 'active',  statusColor: 'green'  },
  { initials: 'TN', gradient: 'linear-gradient(135deg,#06b6d4,#22c55e)',             name: 'Tran Nam',  email: 'tran.nam@greenlab.io',  wss: ['Default','Farm'],       role: 'operator', roleColor: 'blue',   status: 'active',  statusColor: 'green'  },
  { initials: 'LV', gradient: 'linear-gradient(135deg,#f59e0b,#ef4444)',             name: 'Le Van',    email: 'le.van@greenlab.io',    wss: ['Default'],              role: 'viewer',   roleColor: 'muted',  status: 'pending', statusColor: 'yellow' },
]

const INVITE_GRADIENTS = [
  'linear-gradient(135deg,#a855f7,#06b6d4)',
  'linear-gradient(135deg,#f59e0b,#22c55e)',
  'linear-gradient(135deg,#ef4444,#a855f7)',
  'linear-gradient(135deg,#3b82f6,#06b6d4)',
]

function wsLabel(slug: string) {
  if (slug.includes('farm')) return 'Farm'
  if (slug.includes('rd')) return 'R&D'
  // for any other workspace, use the part after the last '/'
  return slug.split('/').pop()?.replace(/-/g, ' ').replace(/\b\w/g, c => c.toUpperCase()) ?? slug
}

function emailInitials(email: string) {
  const local = email.split('@')[0]
  const parts = local.split(/[._-]/)
  return parts.length >= 2
    ? (parts[0][0] + parts[1][0]).toUpperCase()
    : local.slice(0, 2).toUpperCase()
}

function EditWorkspaceDrawer({ workspace, wsMembers, onClose, onSave, onRemoveMember }: {
  workspace: Workspace | null
  wsMembers: Member[]
  onClose(): void
  onSave(slug: string, patch: Partial<Workspace>, invitedEmails: string[]): void
  onRemoveMember(email: string, wsSlug: string): void
}) {
  const [name,         setName]         = useState(workspace?.name ?? '')
  const [slug,         setSlug]         = useState(workspace?.slug.split('/')[1] ?? '')
  const [desc,         setDesc]         = useState(workspace?.desc ?? '')
  const [invite,       setInvite]       = useState('')
  const [invited,      setInvited]      = useState<string[]>([])
  const [slugEdited,   setSlugEdited]   = useState(false)
  const [removeTarget, setRemoveTarget] = useState<string | null>(null)

  if (!workspace) return null

  function handleNameChange(v: string) {
    setName(v)
    if (!slugEdited) setSlug(slugify(v))
  }

  function handleSlugChange(v: string) {
    setSlugEdited(true)
    setSlug(v.toLowerCase().replace(/[^a-z0-9-]/g, ''))
  }

  function handleAddInvite() {
    const emails = invite.split(',').map(e => e.trim()).filter(e => e && !invited.includes(e))
    if (emails.length === 0) return
    setInvited(prev => [...prev, ...emails])
    setInvite('')
  }

  function removeInvite(email: string) {
    setInvited(prev => prev.filter(e => e !== email))
  }

  function handleSave() {
    if (!name.trim() || !slug.trim()) return
    onSave(
      workspace!.slug,
      { name: name.trim(), slug: `greenlab/${slug.trim()}`, desc: desc.trim(), members: (workspace?.members ?? 0) + invited.length },
      invited,
    )
    handleClose()
  }

  function handleClose() {
    onClose()
    setTimeout(() => {
      setName(workspace?.name ?? '')
      setSlug(workspace?.slug.split('/')[1] ?? '')
      setDesc(workspace?.desc ?? '')
      setInvite('')
      setInvited([])
      setSlugEdited(false)
    }, 300)
  }

  const valid = name.trim().length > 0 && slug.trim().length > 0

  return (
    <>
      <div onClick={handleClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.55)', backdropFilter: 'blur(3px)', zIndex: 200 }} />
      <div style={{
        position: 'fixed', top: 0, right: 0, bottom: 0, width: 440,
        background: 'var(--surface)', borderLeft: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column', zIndex: 201, overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>{workspace.icon}</span>
          <h2 style={{ fontSize: 16, fontWeight: 700, flex: 1 }}>Edit Workspace</h2>
          <button onClick={handleClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        {/* Body */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '24px 20px' }}>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 20 }}>
            Update the details for <strong style={{ color: 'var(--text)' }}>{workspace.name}</strong>.
          </p>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Workspace Name <span style={{ color: 'var(--red)' }}>*</span></label>
            <input value={name} onChange={e => handleNameChange(e.target.value)} placeholder="e.g. Farm Project Alpha" style={inputStyle} autoFocus />
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Slug</label>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <span style={{
                background: 'var(--surface2)', border: '1px solid var(--border)',
                borderRight: 'none', borderRadius: 'var(--radius) 0 0 var(--radius)',
                padding: '8px 10px', fontSize: 12, color: 'var(--muted)', whiteSpace: 'nowrap',
              }}>greenlab /</span>
              <input
                value={slug}
                onChange={e => handleSlugChange(e.target.value)}
                placeholder="workspace-slug"
                style={{ ...inputStyle, borderRadius: '0 var(--radius) var(--radius) 0', fontFamily: 'monospace', fontSize: 12 }}
              />
            </div>
            <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>
              Used in API URLs. Lowercase letters, numbers, hyphens only.
            </div>
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Description</label>
            <input value={desc} onChange={e => setDesc(e.target.value)} placeholder="What is this workspace for?" style={inputStyle} />
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Organisation</label>
            <input value="GreenLab Technologies" disabled style={{ ...inputStyle, opacity: .6, cursor: 'not-allowed' }} />
            <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>Workspaces always belong to the current organisation.</div>
          </div>

          {/* Plan badge (read-only) */}
          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Plan</label>
            <div style={{
              display: 'flex', alignItems: 'center', gap: 10,
              background: 'var(--surface2)', border: '1px solid var(--border)',
              borderRadius: 'var(--radius)', padding: '8px 12px',
            }}>
              <Badge color={workspace.planColor}>{workspace.plan}</Badge>
              <span style={{ fontSize: 12, color: 'var(--muted)' }}>To change plan, go to Billing settings.</span>
            </div>
          </div>

          {/* Quota bar */}
          <div style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', padding: '14px 16px', marginBottom: 20 }}>
            <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em', marginBottom: 10 }}>
              Usage Today
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, marginBottom: 6 }}>
              <span style={{ color: 'var(--muted)' }}>Readings</span>
              <span style={{ fontWeight: 600 }}>{workspace.used} / {workspace.quota}</span>
            </div>
            <div style={{ height: 6, background: 'var(--border)', borderRadius: 99, overflow: 'hidden' }}>
              <div style={{ height: '100%', width: `${workspace.pct}%`, background: workspace.barColor || 'var(--accent)', borderRadius: 99 }} />
            </div>
          </div>

          {/* Current Members */}
          <div style={{ marginBottom: 20 }}>
            <label style={labelStyle}>Current Members</label>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {wsMembers.length === 0 ? (
                <div style={{ fontSize: 13, color: 'var(--muted)', padding: '10px 0' }}>No members yet.</div>
              ) : wsMembers.map(m => (
                <div key={m.email} style={{ background: 'var(--surface2)', border: `1px solid ${removeTarget === m.email ? 'var(--red)' : 'var(--border)'}`, borderRadius: 'var(--radius)', overflow: 'hidden', transition: 'border-color .15s' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px' }}>
                    <div style={{ width: 26, height: 26, borderRadius: '50%', background: m.gradient, display: 'grid', placeItems: 'center', fontSize: 10, fontWeight: 700, color: '#fff', flexShrink: 0 }}>{m.initials}</div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: 13, fontWeight: 600 }}>{m.name}</div>
                      <div style={{ fontSize: 11, color: 'var(--muted)' }}>{m.email}</div>
                    </div>
                    <Badge color={roleColor[m.role] ?? 'muted'}>{m.role}</Badge>
                    {m.role !== 'admin' && (
                      <button
                        onClick={() => setRemoveTarget(removeTarget === m.email ? null : m.email)}
                        title="Remove from workspace"
                        style={{ background: 'transparent', border: 'none', color: removeTarget === m.email ? 'var(--red)' : 'var(--muted)', cursor: 'pointer', fontSize: 14, padding: '0 2px', lineHeight: 1, marginLeft: 4, transition: 'color .15s' }}
                      >✕</button>
                    )}
                  </div>
                  {removeTarget === m.email && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 12px', background: 'rgba(239,68,68,.06)', borderTop: '1px solid rgba(239,68,68,.2)' }}>
                      <span style={{ fontSize: 12, color: 'var(--red)', flex: 1 }}>Remove <strong>{m.name}</strong> from this workspace?</span>
                      <button
                        onClick={() => setRemoveTarget(null)}
                        style={{ fontSize: 12, padding: '3px 10px', borderRadius: 'var(--radius)', border: '1px solid var(--border)', background: 'var(--surface)', color: 'var(--text)', cursor: 'pointer' }}
                      >Cancel</button>
                      <button
                        onClick={() => { onRemoveMember(m.email, workspace!.slug); setRemoveTarget(null) }}
                        style={{ fontSize: 12, padding: '3px 10px', borderRadius: 'var(--radius)', border: 'none', background: 'var(--red)', color: '#fff', cursor: 'pointer', fontWeight: 600 }}
                      >Remove</button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Invite Members */}
          <div style={{ marginBottom: 8 }}>
            <label style={{ ...labelStyle, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span>Invite Members</span>
              <span style={{ fontSize: 11, color: 'var(--muted)', fontWeight: 400, textTransform: 'none', letterSpacing: 0 }}>viewer access by default</span>
            </label>
            <div style={{ display: 'flex', gap: 8 }}>
              <input
                value={invite}
                onChange={e => setInvite(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && handleAddInvite()}
                placeholder="email@example.com, another@example.com"
                style={{ ...inputStyle, flex: 1 }}
              />
              <Btn variant="ghost" size="sm" onClick={handleAddInvite} disabled={!invite.trim()}>
                Add
              </Btn>
            </div>
            <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>
              Comma-separated emails. Press Enter or click Add.
            </div>
          </div>

          {/* Pending invites */}
          {invited.length > 0 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 10 }}>
              {invited.map(email => (
                <div key={email} style={{ display: 'flex', alignItems: 'center', gap: 10, background: 'rgba(37,99,235,.08)', border: '1px solid rgba(37,99,235,.2)', borderRadius: 'var(--radius)', padding: '7px 12px' }}>
                  <div style={{ width: 26, height: 26, borderRadius: '50%', background: 'var(--surface2)', border: '1px dashed var(--border)', display: 'grid', placeItems: 'center', fontSize: 13, flexShrink: 0 }}>✉️</div>
                  <div style={{ flex: 1, fontSize: 13, color: 'var(--accent-lt)' }}>{email}</div>
                  <span style={{ fontSize: 11, color: 'var(--muted)' }}>pending</span>
                  <button onClick={() => removeInvite(email)} style={{ background: 'transparent', border: 'none', color: 'var(--muted)', cursor: 'pointer', fontSize: 14, padding: '0 2px', lineHeight: 1 }}>✕</button>
                </div>
              ))}
              <div style={{ fontSize: 11, color: 'var(--accent-lt)', marginTop: 2 }}>
                {invited.length} invite{invited.length > 1 ? 's' : ''} will be sent when you save.
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 20px', borderTop: '1px solid var(--border)', flexShrink: 0 }}>
          <div style={{ flex: 1 }} />
          <Btn variant="ghost" onClick={handleClose}>Cancel</Btn>
          <Btn variant="primary" onClick={handleSave} disabled={!valid} style={{ opacity: valid ? 1 : .5 }}>
            Save Changes
          </Btn>
        </div>
      </div>
    </>
  )
}

// ── Create Workspace Drawer ────────────────────────────────────────────────

function CreateWorkspaceDrawer({ open, onClose, onCreate }: {
  open: boolean
  onClose: () => void
  onCreate: (ws: Workspace) => void
}) {
  const [name,       setName]       = useState('')
  const [slug,       setSlug]       = useState('')
  const [desc,       setDesc]       = useState('')
  const [invite,     setInvite]     = useState('')
  const [slugEdited, setSlugEdited] = useState(false)

  function handleNameChange(v: string) {
    setName(v)
    if (!slugEdited) setSlug(slugify(v))
  }

  function handleSlugChange(v: string) {
    setSlugEdited(true)
    setSlug(v.toLowerCase().replace(/[^a-z0-9-]/g, ''))
  }

  function handleCreate() {
    if (!name.trim() || !slug.trim()) return
    onCreate({
      id: '', icon: '🗂️', name: name.trim(), slug: `greenlab/${slug.trim()}`,
      desc: desc.trim(), plan: 'Free', planColor: 'muted',
      devices: 0, channels: 0,
      members: invite ? invite.split(',').length + 1 : 1,
      used: '0', quota: '10K', pct: 0,
    })
    setName(''); setSlug(''); setDesc(''); setInvite(''); setSlugEdited(false)
    onClose()
  }

  const valid = name.trim().length > 0 && slug.trim().length > 0

  return (
    <>
      {open && (
        <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.55)', backdropFilter: 'blur(3px)', zIndex: 200 }} />
      )}
      <div style={{
        position: 'fixed', top: 0, right: 0, bottom: 0, width: 440,
        background: 'var(--surface)', borderLeft: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column',
        transform: open ? 'translateX(0)' : 'translateX(100%)',
        transition: 'transform 300ms cubic-bezier(.4,0,.2,1)',
        zIndex: 201, overflow: 'hidden',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>🗂️</span>
          <h2 style={{ fontSize: 16, fontWeight: 700, flex: 1 }}>Create Workspace</h2>
          <button onClick={onClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        <div style={{ flex: 1, overflowY: 'auto', padding: '24px 20px' }}>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 20 }}>
            Workspaces let you group devices and channels by project, team, or location — all under the same organisation.
          </p>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Workspace Name <span style={{ color: 'var(--red)' }}>*</span></label>
            <input value={name} onChange={e => handleNameChange(e.target.value)} placeholder="e.g. Farm Project Alpha" style={inputStyle} />
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Slug</label>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <span style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRight: 'none', borderRadius: 'var(--radius) 0 0 var(--radius)', padding: '8px 10px', fontSize: 12, color: 'var(--muted)', whiteSpace: 'nowrap' }}>greenlab /</span>
              <input value={slug} onChange={e => handleSlugChange(e.target.value)} placeholder="farm-project-alpha" style={{ ...inputStyle, borderRadius: '0 var(--radius) var(--radius) 0', fontFamily: 'monospace', fontSize: 12 }} />
            </div>
            <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>Used in API URLs. Auto-generated from name.</div>
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Description</label>
            <input value={desc} onChange={e => setDesc(e.target.value)} placeholder="What is this workspace for?" style={inputStyle} />
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Organisation</label>
            <input value="GreenLab Technologies" disabled style={{ ...inputStyle, opacity: .6, cursor: 'not-allowed' }} />
            <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>Workspaces always belong to the current organisation.</div>
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={{ ...labelStyle, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span>Invite Members</span>
              <span style={{ fontSize: 11, color: 'var(--muted)', fontWeight: 400, textTransform: 'none', letterSpacing: 0 }}>Optional</span>
            </label>
            <input value={invite} onChange={e => setInvite(e.target.value)} placeholder="email@example.com, another@example.com" style={inputStyle} />
            <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>
              Comma-separated emails. Invited users get <strong style={{ color: 'var(--text)' }}>viewer</strong> access by default.
            </div>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 20px', borderTop: '1px solid var(--border)', flexShrink: 0 }}>
          <div style={{ flex: 1 }} />
          <Btn variant="ghost" onClick={onClose}>Cancel</Btn>
          <Btn variant="primary" onClick={handleCreate} disabled={!valid} style={{ opacity: valid ? 1 : .5 }}>
            Create Workspace
          </Btn>
        </div>
      </div>
    </>
  )
}

// ── Workspace Card ─────────────────────────────────────────────────────────

function WorkspaceCard({ ws, selected, onViewDevices, onEdit, onSelect }: {
  ws: Workspace
  selected: boolean
  onViewDevices(w: Workspace): void
  onEdit(w: Workspace): void
  onSelect(w: Workspace): void
}) {
  return (
    <div
      onClick={() => onSelect(ws)}
      style={{
        background: 'var(--surface)',
        border: selected ? '1px solid var(--accent)' : '1px solid var(--border)',
        boxShadow: selected ? '0 0 0 2px rgba(37,99,235,.25)' : 'none',
        borderRadius: 'var(--radius-lg)', padding: 20,
        display: 'flex', flexDirection: 'column', gap: 12,
        transition: 'border-color var(--transition), box-shadow var(--transition)',
        cursor: 'pointer',
      }}
      onMouseEnter={e => { if (!selected) { e.currentTarget.style.borderColor = 'var(--accent)'; e.currentTarget.style.boxShadow = '0 0 0 1px rgba(37,99,235,.15)' } }}
      onMouseLeave={e => { if (!selected) { e.currentTarget.style.borderColor = 'var(--border)'; e.currentTarget.style.boxShadow = 'none' } }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: 22 }}>{ws.icon}</div>
          <div style={{ fontSize: 14, fontWeight: 600, marginTop: 6 }}>{ws.name}</div>
          <div style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--muted)' }}>{ws.slug}</div>
        </div>
        <Badge color={ws.planColor}>{ws.plan}</Badge>
      </div>

      {ws.desc && (
        <div style={{ fontSize: 12, color: 'var(--muted)', lineHeight: 1.5 }}>{ws.desc}</div>
      )}

      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
        {[`${ws.devices} devices`, `${ws.channels} channels`, `${ws.members} members`].map(s => (
          <span key={s} style={{ fontSize: 12, color: 'var(--muted)' }}>
            <strong style={{ color: 'var(--text)' }}>{s.split(' ')[0]}</strong> {s.split(' ').slice(1).join(' ')}
          </span>
        ))}
      </div>

      <div style={{ fontSize: 12, color: 'var(--muted)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
          <span>Readings today</span>
          <span style={{ color: 'var(--text)', fontWeight: 600 }}>{ws.used} / {ws.quota}</span>
        </div>
        <div style={{ height: 4, background: 'var(--border)', borderRadius: 99, overflow: 'hidden' }}>
          <div style={{ height: '100%', width: `${ws.pct}%`, background: ws.barColor || 'var(--accent)', borderRadius: 99 }} />
        </div>
      </div>

      <div style={{ display: 'flex', gap: 8 }} onClick={e => e.stopPropagation()}>
        <Btn variant="ghost" size="sm" style={{ flex: 1 }} onClick={() => onViewDevices(ws)}>View Devices</Btn>
        <Btn variant="ghost" size="sm" onClick={() => onEdit(ws)}>Edit</Btn>
      </div>
    </div>
  )
}

// ── Page ───────────────────────────────────────────────────────────────────

function apiWorkspaceToLocal(w: ApiWorkspace): Workspace {
  const planColorMap: Record<string, 'purple' | 'blue' | 'muted'> = { Pro: 'purple', Starter: 'blue', Free: 'muted' }
  const plan = w.plan ?? 'Free'
  return {
    id: w.id,
    icon: '🗂️',
    name: w.name,
    slug: w.slug,
    desc: w.description ?? '',
    plan,
    planColor: planColorMap[plan] ?? 'muted',
    devices: w.device_count ?? 0,
    channels: w.channel_count ?? 0,
    members: w.member_count ?? 0,
    used: '—', quota: '—', pct: 0,
  }
}

function apiMemberToLocal(m: ApiMember, wsName: string): Member {
  const roleColorMap: Record<string, 'purple' | 'blue' | 'muted'> = { admin: 'purple', operator: 'blue', viewer: 'muted' }
  return {
    user_id: m.user_id,
    initials: emailInitials(m.email),
    gradient: INVITE_GRADIENTS[0],
    name: m.name,
    email: m.email,
    wss: [wsName],
    role: m.role as Member['role'],
    roleColor: roleColorMap[m.role] ?? 'muted',
    status: 'active',
    statusColor: 'green',
  }
}

const initial: Workspace[] = [
  { id: '', icon: '🏢', name: 'Default Workspace', slug: 'greenlab/default',      desc: 'Primary workspace for all production devices.',        plan: 'Pro',     planColor: 'purple', devices: 8, channels: 18, members: 3, used: '148K', quota: '1M',   pct: 15 },
  { id: '', icon: '🌾', name: 'Farm Project',      slug: 'greenlab/farm-project', desc: 'Soil and crop monitoring for the farm deployment.',     plan: 'Starter', planColor: 'blue',   devices: 3, channels: 5,  members: 2, used: '38K',  quota: '100K', pct: 38, barColor: 'var(--accent-lt)' },
  { id: '', icon: '🔬', name: 'R&D Lab',           slug: 'greenlab/rd-lab',       desc: 'Experimental devices for internal R&D projects only.', plan: 'Free',    planColor: 'muted',  devices: 1, channels: 2,  members: 1, used: '620',  quota: '10K',  pct: 6,  barColor: 'var(--muted)' },
]

export function WorkspacesPage() {
  const { toast } = useToast()
  const [workspaces,   setWorkspaces]   = useState<Workspace[]>([])
  const [loading,      setLoading]      = useState(true)
  const [members,      setMembers]      = useState<Member[]>([])
  const [createOpen,   setCreateOpen]   = useState(false)
  const [viewTarget,   setViewTarget]   = useState<Workspace | null>(null)
  const [editTarget,   setEditTarget]   = useState<Workspace | null>(null)
  const [selectedWs,   setSelectedWs]   = useState<string | null>(null)

  const orgId = localStorage.getItem('org_id') ?? '1'

  useEffect(() => {
    workspacesApi.list(orgId)
      .then(async r => {
        const wsList = r.data.map(apiWorkspaceToLocal)
        setWorkspaces(wsList)
        const allMembersArrays = await Promise.all(
          wsList.map(ws =>
            workspacesApi.listMembers(ws.id)
              .then(mr => mr.data.map(m => apiMemberToLocal(m, wsLabel(ws.slug))))
              .catch(() => [] as Member[])
          )
        )
        const merged: Member[] = []
        allMembersArrays.flat().forEach(m => {
          const existing = merged.find(e => e.email === m.email)
          if (existing) {
            m.wss.forEach(w => { if (!existing.wss.includes(w)) existing.wss.push(w) })
          } else {
            merged.push(m)
          }
        })
        setMembers(merged)
      })
      .catch(() => toast('Failed to load workspaces', 'error'))
      .finally(() => setLoading(false))
  }, [])

  const visibleMembers = selectedWs
    ? members.filter(m => m.wss.includes(wsLabel(selectedWs)))
    : members

  function handleCreate(ws: Workspace) {
    workspacesApi.create({ org_id: orgId, name: ws.name, slug: ws.slug.split('/').pop() ?? ws.slug, description: ws.desc })
      .then(r => setWorkspaces(prev => [...prev, apiWorkspaceToLocal(r.data)]))
      .catch(() => setWorkspaces(prev => [...prev, ws]))
  }

  function handleRemoveMember(email: string, wsSlug: string) {
    const ws = workspaces.find(w => w.slug === wsSlug)
    const member = members.find(m => m.email === email)
    if (ws && member) workspacesApi.removeMember(ws.id, member.user_id ?? '').catch(() => {})
    const label = wsLabel(wsSlug)
    setMembers(prev =>
      prev
        .map(m => m.email === email ? { ...m, wss: m.wss.filter(w => w !== label) } : m)
        .filter(m => m.wss.length > 0)
    )
    setWorkspaces(prev => prev.map(w => w.slug === wsSlug ? { ...w, members: Math.max(0, w.members - 1) } : w))
  }

  function handleSaveEdit(slug: string, patch: Partial<Workspace>, invitedEmails: string[]) {
    const ws = workspaces.find(w => w.slug === slug)
    if (ws) workspacesApi.update(ws.id, { name: patch.name, slug: patch.slug?.split('/').pop(), description: patch.desc }).catch(() => {})
    setWorkspaces(prev => prev.map(w => w.slug === slug ? { ...w, ...patch } : w))
    if (invitedEmails.length === 0) return
    const label = wsLabel(slug)
    // invite by email not yet supported (requires user_id lookup)
    invitedEmails.forEach(_email => {})
    setMembers(prev => {
      const next = [...prev]
      invitedEmails.forEach((email, idx) => {
        const existing = next.find(m => m.email === email)
        if (existing) {
          if (!existing.wss.includes(label)) existing.wss = [...existing.wss, label]
        } else {
          next.push({
            initials: emailInitials(email),
            gradient: INVITE_GRADIENTS[(next.length + idx) % INVITE_GRADIENTS.length],
            name: email,
            email,
            wss: [label],
            role: 'viewer', roleColor: 'muted',
            status: 'pending', statusColor: 'yellow',
          })
        }
      })
      return next
    })
  }

  function loadMembers(ws: Workspace) {
    setEditTarget(ws)
    workspacesApi.listMembers(ws.id)
      .then(r => {
        const apiMembers = r.data.map(m => apiMemberToLocal(m, wsLabel(ws.slug)))
        setMembers(prev => {
          const wsName = wsLabel(ws.slug)
          const withoutWs = prev.filter(m => !m.wss.includes(wsName))
          const merged = [...withoutWs]
          apiMembers.forEach(am => {
            const existing = merged.find(m => m.email === am.email)
            if (existing) {
              if (!existing.wss.includes(wsName)) existing.wss = [...existing.wss, wsName]
            } else {
              merged.push(am)
            }
          })
          return merged
        })
      })
      .catch(() => {})
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Workspaces</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>
            Group your devices and channels by project, team, or location
          </p>
        </div>
        <div style={{ marginLeft: 'auto' }}>
          <Btn variant="primary" size="sm" onClick={() => setCreateOpen(true)}>+ New Workspace</Btn>
        </div>
      </div>

      {loading && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 160 }}>
          <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
        </div>
      )}

      <div className="rg3" style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 24 }}>
        {workspaces.map(ws => (
          <WorkspaceCard
            key={ws.slug}
            ws={ws}
            selected={selectedWs === ws.slug}
            onViewDevices={setViewTarget}
            onEdit={loadMembers}
            onSelect={w => setSelectedWs(prev => prev === w.slug ? null : w.slug)}
          />
        ))}

        {/* Add new placeholder */}
        <div
          onClick={() => setCreateOpen(true)}
          style={{
            background: 'transparent', border: '1px dashed var(--border)',
            borderRadius: 'var(--radius-lg)', padding: 20,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            minHeight: 200, cursor: 'pointer', transition: 'border-color var(--transition)',
          }}
          onMouseEnter={e => (e.currentTarget.style.borderColor = 'var(--accent)')}
          onMouseLeave={e => (e.currentTarget.style.borderColor = 'var(--border)')}
        >
          <div style={{ textAlign: 'center', color: 'var(--muted)', padding: '48px 20px' }}>
            <div style={{ fontSize: 36, marginBottom: 12 }}>➕</div>
            <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--text)', marginBottom: 4 }}>New Workspace</div>
            <div style={{ fontSize: 13, marginBottom: 16 }}>Separate project, team, or environment</div>
            <Btn variant="primary" size="sm" onClick={e => { e.stopPropagation(); setCreateOpen(true) }}>
              + Create Workspace
            </Btn>
          </div>
        </div>
      </div>

      {/* Members */}
      <Card>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
          <CardTitle style={{ margin: 0 }}>Members &amp; Access</CardTitle>
          {selectedWs ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <Badge color="blue">{wsLabel(selectedWs)}</Badge>
              <button
                onClick={() => setSelectedWs(null)}
                style={{ background: 'transparent', border: 'none', color: 'var(--muted)', cursor: 'pointer', fontSize: 12, padding: '0 2px' }}
                title="Show all workspaces"
              >✕ all</button>
            </div>
          ) : (
            <span style={{ fontSize: 12, color: 'var(--muted)' }}>All Workspaces</span>
          )}
        </div>
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Member', 'Workspaces', 'Role', 'Status', ''].map(h => (
                  <th key={h} style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '.06em', color: 'var(--muted)', textAlign: 'left', padding: '10px 12px', borderBottom: '1px solid var(--border)' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {loading && members.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ padding: '24px 12px', textAlign: 'center', color: 'var(--muted)', fontSize: 13 }}>
                    Loading members…
                  </td>
                </tr>
              )}
              {!loading && visibleMembers.length === 0 && (
                <tr>
                  <td colSpan={5} style={{ padding: '24px 12px', textAlign: 'center', color: 'var(--muted)', fontSize: 13 }}>
                    No members found.
                  </td>
                </tr>
              )}
              {visibleMembers.map((m, i) => (
                <tr key={i}>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <div style={{ width: 28, height: 28, borderRadius: '50%', background: m.gradient, display: 'grid', placeItems: 'center', fontSize: 11, fontWeight: 700, color: '#fff', flexShrink: 0 }}>{m.initials}</div>
                      <div>
                        <div style={{ fontWeight: 600 }}>{m.name}</div>
                        {m.name !== m.email && <div style={{ color: 'var(--muted)', fontSize: 11 }}>{m.email}</div>}
                      </div>
                    </div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}>
                    <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                      {m.wss.map(w => <Badge key={w} color="muted">{w}</Badge>)}
                    </div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}><Badge color={m.roleColor}>{m.role}</Badge></td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}><Badge color={m.statusColor}>{m.status}</Badge></td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}>
                    {m.role !== 'admin' && <Btn variant="ghost" size="sm">Manage</Btn>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      {/* Drawers */}
      <CreateWorkspaceDrawer open={createOpen} onClose={() => setCreateOpen(false)} onCreate={handleCreate} />
      <ViewDevicesDrawer workspace={viewTarget} onClose={() => setViewTarget(null)} />
      <EditWorkspaceDrawer
        key={editTarget?.slug}
        workspace={editTarget}
        wsMembers={editTarget ? members.filter(m => m.wss.includes(wsLabel(editTarget.slug))) : []}
        onClose={() => setEditTarget(null)}
        onSave={handleSaveEdit}
        onRemoveMember={handleRemoveMember}
      />
    </div>
  )
}
