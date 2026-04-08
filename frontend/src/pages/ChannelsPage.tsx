import { useState, useEffect } from 'react'
import { Badge, Dot } from '../components/ui/Badge'
import { Btn } from '../components/ui/Button'
import { Card, CardTitle } from '../components/ui/Card'
import { useToast } from '../contexts/ToastContext'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { channelsApi } from '../api/channels'
import { fieldsApi } from '../api/fields'
import { workspacesApi } from '../api/workspaces'
import { devicesApi } from '../api/devices'
import type { Channel as ApiChannel, Workspace } from '../types'

// ── Types ───────────────────────────────────────────────────────────────────

interface FieldDef {
  key: string
  name: string
  unit: string
  type: 'number' | 'boolean' | 'string'
  color: string
  enabled: boolean
}

interface Channel {
  id: string
  name: string
  device: string
  workspace: string
  tags: string[]
  fields: FieldDef[]
  lastReading: string
  readings: string
  updated: string
  public: boolean
}

// ── Constants ────────────────────────────────────────────────────────────────

const FIELD_COLORS = [
  'var(--accent)',
  'var(--green)',
  'var(--yellow)',
  'var(--purple)',
  '#06b6d4',
  'var(--red)',
  '#f97316',
  '#a78bfa',
]

// ── Shared styles ────────────────────────────────────────────────────────────

const labelStyle: React.CSSProperties = {
  display: 'block', fontSize: 11, fontWeight: 600,
  color: 'var(--muted)', marginBottom: 5,
  textTransform: 'uppercase', letterSpacing: '.05em',
}

const inputStyle: React.CSSProperties = {
  width: '100%', background: 'var(--surface2)',
  border: '1px solid var(--border)', borderRadius: 'var(--radius)',
  padding: '7px 10px', color: 'var(--text)', fontSize: 12, outline: 'none',
  boxSizing: 'border-box',
}

const drawerOverlay: React.CSSProperties = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,.55)',
  backdropFilter: 'blur(3px)', zIndex: 200,
}

const drawerBase: React.CSSProperties = {
  position: 'fixed', top: 0, right: 0, bottom: 0,
  background: 'var(--surface)', borderLeft: '1px solid var(--border)',
  display: 'flex', flexDirection: 'column', zIndex: 201, overflow: 'hidden',
}

const TYPE_OPTIONS = ['number', 'boolean', 'string'] as const

// ── View Channel Drawer ──────────────────────────────────────────────────────

function ViewChannelDrawer({ channel, onClose }: { channel: Channel | null; onClose(): void }) {
  useEscapeKey(onClose, channel != null)
  if (!channel) return null
  const recent: { time: string; values: string[] }[] = []

  return (
    <>
      <div onClick={onClose} style={drawerOverlay} />
      <div style={{ ...drawerBase, width: 560 }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>📡</span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <h2 style={{ fontSize: 15, fontWeight: 700 }}>{channel.name}</h2>
              <Badge color={channel.public ? 'blue' : 'muted'}>{channel.public ? 'Public' : 'Private'}</Badge>
            </div>
            <div style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--muted)' }}>{channel.id}</div>
          </div>
          <button onClick={onClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        <div style={{ flex: 1, overflowY: 'auto', padding: '20px' }}>
          {/* Meta */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, marginBottom: 20 }}>
            {[
              { label: 'Device',    value: channel.device },
              { label: 'Workspace', value: channel.workspace },
              { label: 'Readings / 24h', value: channel.readings },
              { label: 'Last Updated', value: channel.updated },
            ].map(r => (
              <div key={r.label} style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '10px 14px' }}>
                <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 2 }}>{r.label}</div>
                <div style={{ fontSize: 13, fontWeight: 600 }}>{r.value}</div>
              </div>
            ))}
          </div>

          {/* Tags */}
          {channel.tags.length > 0 && (
            <div style={{ marginBottom: 20 }}>
              <div style={labelStyle}>Tags</div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {channel.tags.map(t => (
                  <span key={t} style={{ fontSize: 11, fontWeight: 600, padding: '3px 10px', borderRadius: 99, background: 'var(--surface2)', border: '1px solid var(--border)', color: 'var(--muted)' }}>{t}</span>
                ))}
              </div>
            </div>
          )}

          {/* Fields */}
          <div style={{ marginBottom: 20 }}>
            <div style={labelStyle}>Fields ({channel.fields.length})</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {channel.fields.map(f => (
                <div key={f.key} style={{ display: 'flex', alignItems: 'center', gap: 10, background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '8px 12px' }}>
                  <span style={{ width: 10, height: 10, borderRadius: '50%', background: f.color, flexShrink: 0 }} />
                  <span style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--muted)', flexShrink: 0, minWidth: 52 }}>{f.key}</span>
                  <span style={{ fontWeight: 600, fontSize: 13, flex: 1 }}>{f.name}</span>
                  {f.unit && <span style={{ fontSize: 11, color: 'var(--muted)', background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 4, padding: '1px 6px' }}>{f.unit}</span>}
                  <span style={{ fontSize: 11, color: 'var(--muted)' }}>{f.type}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Recent readings */}
          <div style={{ marginBottom: 20 }}>
            <div style={labelStyle}>Recent Readings</div>
            {recent.length === 0 ? (
              <div style={{ fontSize: 13, color: 'var(--muted)', padding: '12px 0' }}>No data yet.</div>
            ) : (
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
                  <thead>
                    <tr>
                      <th style={{ textAlign: 'left', padding: '6px 10px', borderBottom: '1px solid var(--border)', color: 'var(--muted)', fontWeight: 600, whiteSpace: 'nowrap' }}>Time</th>
                      {channel.fields.map(f => (
                        <th key={f.key} style={{ textAlign: 'right', padding: '6px 10px', borderBottom: '1px solid var(--border)', whiteSpace: 'nowrap' }}>
                          <span style={{ color: f.color, fontWeight: 700 }}>{f.name}</span>
                          {f.unit && <span style={{ color: 'var(--muted)', fontWeight: 400 }}> ({f.unit})</span>}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {recent.map((row, i) => (
                      <tr key={i}>
                        <td style={{ padding: '7px 10px', borderBottom: '1px solid var(--border)', fontFamily: 'monospace', color: 'var(--muted)', whiteSpace: 'nowrap' }}>{row.time}</td>
                        {row.values.map((v, j) => (
                          <td key={j} style={{ padding: '7px 10px', borderBottom: '1px solid var(--border)', textAlign: 'right', fontWeight: 600 }}>{v}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* API section */}
          <div>
            <div style={labelStyle}>API Access</div>
            <div style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '12px 14px' }}>
              <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 6 }}>Write endpoint</div>
              <code style={{ fontSize: 11, color: 'var(--accent-lt)', wordBreak: 'break-all', display: 'block', lineHeight: 1.6 }}>
                POST /api/v1/channels/{channel.id}/feeds
              </code>
              <div style={{ height: 1, background: 'var(--border)', margin: '10px 0' }} />
              <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 6 }}>Read latest</div>
              <code style={{ fontSize: 11, color: 'var(--accent-lt)', wordBreak: 'break-all', display: 'block', lineHeight: 1.6 }}>
                GET /api/v1/channels/{channel.id}/feeds/last
              </code>
            </div>
          </div>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', padding: '16px 20px', borderTop: '1px solid var(--border)', flexShrink: 0 }}>
          <Btn variant="ghost" onClick={onClose}>Close</Btn>
        </div>
      </div>
    </>
  )
}

// ── Edit Schema Drawer ───────────────────────────────────────────────────────

function EditSchemaDrawer({ channel, onClose, onSave }: {
  channel: Channel | null
  onClose(): void
  onSave(id: string, fields: FieldDef[]): void
}) {
  useEscapeKey(onClose, channel != null)
  const [fields, setFields] = useState<FieldDef[]>(
    Array.from({ length: 8 }, (_, i) => ({
      key: `field${i + 1}`, name: '', unit: '', type: 'number' as const,
      color: FIELD_COLORS[i % FIELD_COLORS.length], enabled: false,
    }))
  )
  const [loadingFields, setLoadingFields] = useState(false)

  // Fetch existing fields for this channel when it opens
  useEffect(() => {
    if (!channel) return
    setLoadingFields(true)
    fieldsApi.list(channel.id)
      .then(res => {
        if ((res.data as any[]).length > 0) {
          const slots = Array.from({ length: 8 }, (_, i) => ({
            key: `field${i + 1}`, name: '', unit: '', type: 'number' as const,
            color: FIELD_COLORS[i % FIELD_COLORS.length], enabled: false,
          }))
          ;(res.data as any[]).forEach(f => {
            const pos = (f.position ?? 1) - 1
            if (pos >= 0 && pos < 8) {
              slots[pos] = {
                key:     f.name,
                name:    f.label || f.name,
                unit:    f.unit ?? '',
                type:    (f.field_type === 'float' || f.field_type === 'integer' ? 'number' : f.field_type) as FieldDef['type'],
                color:   FIELD_COLORS[pos % FIELD_COLORS.length],
                enabled: true,
              }
            }
          })
          setFields(slots)
        } else {
          setFields(Array.from({ length: 8 }, (_, i) => ({
            key: `field${i + 1}`, name: '', unit: '', type: 'number' as const,
            color: FIELD_COLORS[i % FIELD_COLORS.length], enabled: false,
          })))
        }
      })
      .catch(() => {})
      .finally(() => setLoadingFields(false))
  }, [channel?.id])

  if (!channel) return null

  function update(idx: number, patch: Partial<FieldDef>) {
    setFields(prev => prev.map((f, i) => i === idx ? { ...f, ...patch } : f))
  }

  function toggle(idx: number) {
    setFields(prev => prev.map((f, i) => i === idx ? { ...f, enabled: !f.enabled } : f))
  }

  const enabledCount = fields.filter(f => f.enabled && f.name.trim()).length

  return (
    <>
      <div onClick={onClose} style={drawerOverlay} />
      <div style={{ ...drawerBase, width: 520 }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>🔢</span>
          <div style={{ flex: 1 }}>
            <h2 style={{ fontSize: 15, fontWeight: 700 }}>Edit Schema</h2>
            <div style={{ fontSize: 11, color: 'var(--muted)' }}>{channel.name}</div>
          </div>
          <button onClick={onClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        {/* Body */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '20px' }}>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 20 }}>
            Configure up to 8 fields. Enable a slot by clicking its checkbox or typing a name.
            <span style={{ display: 'block', marginTop: 4, color: 'var(--text)', fontWeight: 600 }}>{enabledCount} / 8 fields active</span>
          </p>

          {/* Column headers */}
          <div style={{ display: 'grid', gridTemplateColumns: '110px 1fr 80px 90px', gap: 8, marginBottom: 4 }}>
            {['Field', 'Name', 'Unit', 'Type'].map(h => (
              <div key={h} style={{ fontSize: 10, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', padding: '0 2px' }}>{h}</div>
            ))}
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {fields.map((f, i) => (
              <div key={f.key} style={{ display: 'grid', gridTemplateColumns: '110px 1fr 80px 90px', gap: 8, alignItems: 'center' }}>
                {/* Toggle + key */}
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <button
                    onClick={() => toggle(i)}
                    style={{
                      width: 16, height: 16, borderRadius: 4,
                      border: `2px solid ${f.enabled ? f.color : 'var(--border)'}`,
                      background: f.enabled ? f.color : 'transparent',
                      cursor: 'pointer', flexShrink: 0, transition: 'all .15s',
                    }}
                  />
                  <span style={{ fontFamily: 'monospace', fontSize: 11, color: f.enabled ? f.color : 'var(--muted)', fontWeight: 700 }}>{f.key}</span>
                </div>
                <input
                  value={f.name}
                  onChange={e => update(i, { name: e.target.value, enabled: e.target.value.trim().length > 0 })}
                  placeholder="e.g. Temperature"
                  style={{ ...inputStyle, opacity: f.enabled || f.name ? 1 : .45 }}
                />
                <input
                  value={f.unit}
                  onChange={e => update(i, { unit: e.target.value })}
                  placeholder="°C"
                  disabled={!f.enabled && !f.name}
                  style={{ ...inputStyle, opacity: f.enabled || f.name ? 1 : .45 }}
                />
                <select
                  value={f.type}
                  onChange={e => update(i, { type: e.target.value as FieldDef['type'] })}
                  disabled={!f.enabled && !f.name}
                  style={{ ...inputStyle, opacity: f.enabled || f.name ? 1 : .45, cursor: 'pointer' }}
                >
                  {TYPE_OPTIONS.map(t => <option key={t} value={t}>{t}</option>)}
                </select>
              </div>
            ))}
          </div>
        </div>

        {/* Footer */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 20px', borderTop: '1px solid var(--border)', flexShrink: 0 }}>
          <Btn variant="ghost" size="sm" onClick={() => setFields(Array.from({ length: 8 }, (_, i) => ({
            key: `field${i + 1}`, name: '', unit: '', type: 'number' as const,
            color: FIELD_COLORS[i % FIELD_COLORS.length], enabled: false,
          })))}>Reset</Btn>
          <div style={{ flex: 1 }} />
          <Btn variant="ghost" onClick={onClose}>Cancel</Btn>
          <Btn variant="primary" onClick={() => { onSave(channel.id, fields.filter(f => f.enabled && f.name.trim())); onClose() }}>
            Save Schema
          </Btn>
        </div>
      </div>
    </>
  )
}

// ── Delete Confirm Modal ─────────────────────────────────────────────────────

function DeleteModal({ channel, onClose, onConfirm }: {
  channel: Channel | null
  onClose(): void
  onConfirm(id: string): void
}) {
  useEscapeKey(onClose, channel != null)
  if (!channel) return null
  return (
    <>
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', backdropFilter: 'blur(4px)', zIndex: 300 }} />
      <div style={{
        position: 'fixed', top: '50%', left: '50%',
        transform: 'translate(-50%,-50%)',
        width: 420, background: 'var(--surface)',
        border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)',
        padding: 24, zIndex: 301,
      }}>
        <div style={{ fontSize: 28, marginBottom: 12 }}>🗑️</div>
        <h3 style={{ fontSize: 16, fontWeight: 700, marginBottom: 8 }}>Delete Channel</h3>
        <p style={{ fontSize: 13, color: 'var(--muted)', lineHeight: 1.6, marginBottom: 16 }}>
          Delete <strong style={{ color: 'var(--text)' }}>{channel.name}</strong>?
          This will permanently remove all <strong style={{ color: 'var(--text)' }}>{channel.fields.length} fields</strong> and
          all historical data. This action cannot be undone.
        </p>
        <div style={{ background: 'rgba(239,68,68,.08)', border: '1px solid rgba(239,68,68,.2)', borderRadius: 'var(--radius)', padding: '8px 12px', marginBottom: 20, fontSize: 12, color: 'var(--red)' }}>
          ⚠️ <strong>{channel.readings}</strong> readings will be permanently deleted.
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
          <Btn variant="ghost" onClick={onClose}>Cancel</Btn>
          <Btn variant="danger" onClick={() => { onConfirm(channel.id); onClose() }}>Delete Channel</Btn>
        </div>
      </div>
    </>
  )
}

// ── Create Channel Drawer ────────────────────────────────────────────────────

function CreateChannelDrawer({ open, onClose, onCreate, workspaceOptions, deviceOptions }: {
  open: boolean
  onClose(): void
  onCreate(name: string, deviceId: string, workspaceId: string, isPublic: boolean, tags: string[]): void
  workspaceOptions: { id: string; name: string }[]
  deviceOptions: { id: string; name: string }[]
}) {
  const [name,       setName]      = useState('')
  const [device,     setDevice]    = useState(deviceOptions[0]?.id ?? '')
  const [workspace,  setWorkspace] = useState(workspaceOptions[0]?.id ?? '')
  const [tagInput,   setTagInput]  = useState('')
  const [tags,       setTags]      = useState<string[]>([])
  const [isPublic,   setIsPublic]  = useState(false)

  function reset() {
    setName(''); setDevice(deviceOptions[0]?.id ?? ''); setWorkspace(workspaceOptions[0]?.id ?? '')
    setTagInput(''); setTags([]); setIsPublic(false)
  }

  function handleClose() { onClose(); setTimeout(reset, 300) }

  function addTag() {
    const t = tagInput.trim().toLowerCase()
    if (t && !tags.includes(t)) setTags(prev => [...prev, t])
    setTagInput('')
  }

  function handleCreate() {
    onCreate(name.trim(), device, workspace, isPublic, tags)
    handleClose()
  }

  const valid = name.trim().length > 0

  return (
    <>
      {open && <div onClick={handleClose} style={drawerOverlay} />}
      <div style={{
        ...drawerBase, width: 480,
        transform: open ? 'translateX(0)' : 'translateX(100%)',
        transition: 'transform 300ms cubic-bezier(.4,0,.2,1)',
      }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>📡</span>
          <h2 style={{ fontSize: 16, fontWeight: 700, flex: 1 }}>New Channel</h2>
          <button onClick={handleClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        {/* Body */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '24px 20px' }}>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 20 }}>
            Create a new data channel. Use <strong style={{ color: 'var(--text)' }}>Edit Schema</strong> afterwards to configure fields.
          </p>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Channel Name <span style={{ color: 'var(--red)' }}>*</span></label>
            <input
              value={name} onChange={e => setName(e.target.value)}
              placeholder="e.g. Greenhouse A — Env"
              style={inputStyle} autoFocus
            />
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Device</label>
            <select value={device} onChange={e => setDevice(e.target.value)} style={{ ...inputStyle, cursor: 'pointer' }}>
              {deviceOptions.length === 0
                ? <option value="">No devices in this workspace</option>
                : deviceOptions.map(d => <option key={d.id} value={d.id}>{d.name}</option>)
              }
            </select>
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Workspace</label>
            <select value={workspace} onChange={e => setWorkspace(e.target.value)} style={{ ...inputStyle, cursor: 'pointer' }}>
              {workspaceOptions.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}
            </select>
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Tags</label>
            <div style={{ display: 'flex', gap: 8 }}>
              <input
                value={tagInput}
                onChange={e => setTagInput(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter' || e.key === ',') { e.preventDefault(); addTag() } }}
                placeholder="e.g. env, production"
                style={{ ...inputStyle, flex: 1 }}
              />
              <Btn variant="ghost" size="sm" onClick={addTag} disabled={!tagInput.trim()}>Add</Btn>
            </div>
            {tags.length > 0 && (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 8 }}>
                {tags.map(t => (
                  <span key={t} style={{ display: 'inline-flex', alignItems: 'center', gap: 5, fontSize: 11, fontWeight: 600, padding: '3px 8px', borderRadius: 99, background: 'var(--surface2)', border: '1px solid var(--border)', color: 'var(--muted)' }}>
                    {t}
                    <button onClick={() => setTags(prev => prev.filter(x => x !== t))} style={{ background: 'none', border: 'none', color: 'var(--muted)', cursor: 'pointer', fontSize: 11, padding: 0, lineHeight: 1 }}>✕</button>
                  </span>
                ))}
              </div>
            )}
            <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>Press Enter or comma to add a tag.</div>
          </div>

          <div style={{ marginBottom: 16 }}>
            <label style={labelStyle}>Visibility</label>
            <div style={{ display: 'flex', gap: 10 }}>
              {[false, true].map(v => (
                <div
                  key={String(v)}
                  onClick={() => setIsPublic(v)}
                  style={{
                    flex: 1, padding: '10px 14px', borderRadius: 'var(--radius)',
                    border: `1px solid ${isPublic === v ? 'var(--accent)' : 'var(--border)'}`,
                    background: isPublic === v ? 'rgba(37,99,235,.1)' : 'var(--surface2)',
                    cursor: 'pointer', transition: 'all .15s',
                  }}
                >
                  <div style={{ fontWeight: 600, fontSize: 13 }}>{v ? '🌐 Public' : '🔒 Private'}</div>
                  <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 2 }}>
                    {v ? 'Anyone can read this channel' : 'Only workspace members can access'}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 20px', borderTop: '1px solid var(--border)', flexShrink: 0 }}>
          <div style={{ flex: 1 }} />
          <Btn variant="ghost" onClick={handleClose}>Cancel</Btn>
          <Btn variant="primary" onClick={handleCreate} disabled={!valid} style={{ opacity: valid ? 1 : .5 }}>
            Create Channel
          </Btn>
        </div>
      </div>
    </>
  )
}

function apiChannelToLocal(c: ApiChannel, wsName = ''): Channel {
  const fields: FieldDef[] = (c.fields ?? []).map((f, i) => ({
    key: f.name,
    name: f.label ?? '',
    unit: f.unit ?? '',
    type: (f.field_type === 'float' || f.field_type === 'integer' ? 'number' : f.field_type) as FieldDef['type'],
    color: FIELD_COLORS[i % FIELD_COLORS.length],
    enabled: f.enabled !== false,
  }))
  return {
    id: c.id,
    name: c.name,
    device: c.device_id,
    workspace: wsName,
    tags: c.tags ?? [],
    fields,
    lastReading: c.last_reading ?? '—',
    readings: String(c.reads_24h),
    updated: c.updated_at ? new Date(c.updated_at).toLocaleTimeString() : '—',
    public: c.visibility === 'public',
  }
}

// ── Page ─────────────────────────────────────────────────────────────────────

export function ChannelsPage() {
  const { toast } = useToast()
  const [channels,      setChannels]     = useState<Channel[]>([])
  const [loading,       setLoading]      = useState(true)
  const [createOpen,    setCreateOpen]   = useState(false)
  const [viewTarget,    setViewTarget]   = useState<Channel | null>(null)
  const [editTarget,    setEditTarget]   = useState<Channel | null>(null)
  const [deleteTarget,  setDeleteTarget] = useState<Channel | null>(null)
  const [workspaces,    setWorkspaces]   = useState<Workspace[]>([])
  const [activeWsId,    setActiveWsId]   = useState('')
  const [wsLoaded,      setWsLoaded]     = useState(false)
  const [devices,       setDevices]      = useState<{ id: string; name: string }[]>([])

  const orgId = localStorage.getItem('org_id') ?? ''

  // Load workspaces, then devices for that workspace, then channels for those devices
  useEffect(() => {
    if (!orgId) { setLoading(false); setWsLoaded(true); return }
    workspacesApi.list(orgId)
      .then(r => {
        setWorkspaces(r.data)
        const firstId = r.data[0]?.id ?? ''
        setActiveWsId(firstId)
        if (!firstId) setLoading(false)
      })
      .catch(() => { toast('Failed to load workspaces', 'error'); setLoading(false) })
      .finally(() => setWsLoaded(true))
  }, [orgId])

  useEffect(() => {
    if (!activeWsId) return
    setLoading(true)
    devicesApi.list({ workspace_id: activeWsId })
      .then(r => {
        const devList = r.data.map((d: any) => ({ id: d.id, name: d.name }))
        setDevices(devList)
        const deviceIds = r.data.map((d: any) => d.id)
        if (deviceIds.length === 0) { setChannels([]); setLoading(false); return }
        const wsName = workspaces.find(w => w.id === activeWsId)?.name ?? ''
        return Promise.all(deviceIds.map((id: string) => channelsApi.list({ device_id: id }).then(cr => cr.data)))
          .then(results => setChannels(results.flat().map(c => apiChannelToLocal(c, wsName))))
          .finally(() => setLoading(false))
      })
      .catch(() => { toast('Failed to load channels', 'error'); setLoading(false) })
  }, [activeWsId])

  const totalFields  = channels.reduce((s, c) => s + c.fields.filter(f => f.enabled).length, 0)
  const publicCount  = channels.filter(c => c.public).length
  const privateCount = channels.length - publicCount

  function handleCreate(name: string, deviceId: string, workspaceId: string, isPublic: boolean, _tags: string[]) {
    channelsApi.create({
      workspace_id: workspaceId || activeWsId,
      device_id: deviceId || undefined,
      name,
      visibility: isPublic ? 'public' : 'private',
    })
      .then(r => {
        const wsName = workspaces.find(w => w.id === (workspaceId || activeWsId))?.name ?? ''
        setChannels(prev => [...prev, apiChannelToLocal(r.data, wsName)])
        toast(`Channel "${name}" created`)
      })
      .catch(() => toast('Failed to create channel', 'error'))
  }

  async function handleSaveSchema(id: string, fields: FieldDef[]) {
    const ch = channels.find(c => c.id === id)
    try {
      const existing = await fieldsApi.list(id)
      await Promise.all((existing.data as any[]).map(f => fieldsApi.delete(f.id)))
      await Promise.all(
        fields.map((f, i) =>
          fieldsApi.create({
            channel_id: id,
            name:       f.key,
            label:      f.name,
            unit:       f.unit,
            field_type: f.type === 'number' ? 'float' : f.type as any,
            position:   i + 1,
          })
        )
      )
      setChannels(prev => prev.map(c => c.id === id ? { ...c, fields } : c))
      if (ch) toast(`Schema saved for "${ch.name}"`)
    } catch {
      if (ch) toast(`Failed to save schema for "${ch.name}"`, 'error')
    }
  }

  function handleDelete(id: string) {
    const ch = channels.find(c => c.id === id)
    channelsApi.delete(id)
      .then(() => {
        setChannels(prev => prev.filter(c => c.id !== id))
        if (ch) toast(`Channel "${ch.name}" deleted`, 'error')
      })
      .catch(() => toast('Failed to delete channel', 'error'))
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Channels</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>
            Data streams from your devices — configure fields and visibility
          </p>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
          {workspaces.length > 0 && (
            <select
              value={activeWsId}
              onChange={e => setActiveWsId(e.target.value)}
              style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '7px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }}
            >
              {workspaces.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}
            </select>
          )}
          <Btn variant="primary" size="sm" onClick={() => setCreateOpen(true)}>+ New Channel</Btn>
        </div>
      </div>

      {wsLoaded && workspaces.length === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 240, gap: 12, color: 'var(--muted)' }}>
          <div style={{ fontSize: 40 }}>🗂️</div>
          <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--text)' }}>No workspaces yet</div>
          <div style={{ fontSize: 13 }}>Create a workspace first, then add devices and channels.</div>
        </div>
      )}

      {/* Stats */}
      <div className="rg4" style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12 }}>
        {[
          { label: 'Total Channels', value: channels.length, icon: '📡' },
          { label: 'Active Fields',  value: totalFields,      icon: '🔢' },
          { label: 'Public',         value: publicCount,      icon: '🌐' },
          { label: 'Private',        value: privateCount,     icon: '🔒' },
        ].map(s => (
          <div key={s.label} style={{ background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', padding: '14px 16px', display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={{ fontSize: 22 }}>{s.icon}</span>
            <div>
              <div style={{ fontSize: 20, fontWeight: 700, lineHeight: 1 }}>{s.value}</div>
              <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>{s.label}</div>
            </div>
          </div>
        ))}
      </div>

      {/* Table */}
      <Card>
        <CardTitle>All Channels</CardTitle>
        {loading && workspaces.length > 0 ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 160 }}>
            <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
          </div>
        ) : workspaces.length === 0 ? null : channels.length === 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 220, gap: 12, color: 'var(--muted)' }}>
            <div style={{ fontSize: 40 }}>📡</div>
            <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--text)' }}>No channels yet</div>
            <div style={{ fontSize: 13 }}>Create a channel to start streaming data from your devices.</div>
            <Btn variant="primary" size="sm" onClick={() => setCreateOpen(true)}>+ New Channel</Btn>
          </div>
        ) : (
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Channel', 'Device', 'Tags', 'Fields', 'Last Reading', '24h Reads', 'Updated', 'Visibility', ''].map(h => (
                  <th key={h} style={{
                    fontSize: 11, fontWeight: 600, textTransform: 'uppercase',
                    letterSpacing: '.06em', color: 'var(--muted)', textAlign: 'left',
                    padding: '10px 12px', borderBottom: '1px solid var(--border)', whiteSpace: 'nowrap',
                  }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {channels.map(ch => (
                <tr key={ch.id}
                  onMouseEnter={e => (e.currentTarget.style.background = 'var(--surface2)')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)', maxWidth: 180 }}>
                    <div style={{ fontWeight: 600, fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{ch.name}</div>
                    <div style={{ fontFamily: 'monospace', fontSize: 10, color: 'var(--muted)' }}>{ch.id}</div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)', whiteSpace: 'nowrap' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 12 }}>
                      <span>📡</span>
                      <span style={{ color: 'var(--muted)' }}>{ch.device}</span>
                    </div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                      {ch.tags.map(t => (
                        <span key={t} style={{ fontSize: 10, fontWeight: 600, padding: '2px 7px', borderRadius: 99, background: 'var(--surface2)', border: '1px solid var(--border)', color: 'var(--muted)' }}>{t}</span>
                      ))}
                    </div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, maxWidth: 200 }}>
                      {ch.fields.slice(0, 4).map(f => (
                        <span key={f.key} style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 11, padding: '2px 7px', borderRadius: 99, background: 'var(--surface2)', border: '1px solid var(--border)' }}>
                          <span style={{ width: 6, height: 6, borderRadius: '50%', background: f.color, flexShrink: 0 }} />
                          {f.name}
                        </span>
                      ))}
                      {ch.fields.length > 4 && (
                        <span style={{ fontSize: 11, color: 'var(--muted)', padding: '2px 4px' }}>+{ch.fields.length - 4}</span>
                      )}
                    </div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)', whiteSpace: 'nowrap' }}>
                    <div style={{ fontWeight: 700, fontSize: 13 }}>{ch.lastReading}</div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 4, marginTop: 2 }}>
                      <Dot color="green" />
                      <span style={{ fontSize: 10, color: 'var(--muted)' }}>live</span>
                    </div>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)', whiteSpace: 'nowrap' }}>
                    <span style={{ fontWeight: 700 }}>{ch.readings}</span>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)', whiteSpace: 'nowrap', color: 'var(--muted)', fontSize: 12 }}>
                    {ch.updated}
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}>
                    <Badge color={ch.public ? 'blue' : 'muted'}>{ch.public ? 'Public' : 'Private'}</Badge>
                  </td>
                  <td style={{ padding: '12px 12px', borderBottom: '1px solid var(--border)' }}>
                    <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                      <Btn variant="ghost" size="sm" onClick={() => setViewTarget(ch)}>View</Btn>
                      <Btn variant="ghost" size="sm" onClick={() => setEditTarget(ch)}>Edit Schema</Btn>
                      <Btn variant="danger" size="sm" onClick={() => setDeleteTarget(ch)}>Delete</Btn>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        )}
      </Card>

      {/* Drawers & modals */}
      <CreateChannelDrawer
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreate={handleCreate}
        workspaceOptions={workspaces}
        deviceOptions={devices}
      />
      <ViewChannelDrawer channel={viewTarget} onClose={() => setViewTarget(null)} />
      <EditSchemaDrawer
        key={editTarget?.id}
        channel={editTarget}
        onClose={() => setEditTarget(null)}
        onSave={handleSaveSchema}
      />
      <DeleteModal
        channel={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
      />
    </div>
  )
}
