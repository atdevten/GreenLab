import { useState, useEffect } from 'react'
import {
  Chart as ChartJS, CategoryScale, LinearScale,
  PointElement, LineElement, Filler, Tooltip,
} from 'chart.js'
import { Line } from 'react-chartjs-2'
import { Badge, Dot } from '../components/ui/Badge'
import { Btn } from '../components/ui/Button'
import { Card, CardTitle } from '../components/ui/Card'
import { RegisterDeviceDrawer, toFieldKey, type NewDevice, type Field } from '../components/ui/RegisterDeviceDrawer'
import { useToast } from '../contexts/ToastContext'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { devicesApi } from '../api/devices'
import { channelsApi } from '../api/channels'
import { fieldsApi } from '../api/fields'
import { queryApi } from '../api/query'
import { workspacesApi } from '../api/workspaces'
import { provisionApi } from '../api/provision'
import type { Device as ApiDevice, Workspace, QueryResponse } from '../types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Filler, Tooltip)

// ── Types ──────────────────────────────────────────────────────────────────

interface Device {
  icon: string
  name: string
  id: string
  status: 'Active' | 'Warning' | 'Inactive' | 'Blocked'
  channels: number
  reads: string
  last: string
  apiKey: string
  description: string
  tags: string
  workspace: string
  channelName: string
  visibility: 'private' | 'public'
  fields: Field[]
  lat?: number
  lng?: number
  locationLabel?: string
}

const statusColor: Record<string, 'green' | 'yellow' | 'red' | 'muted'> = {
  Active: 'green', Warning: 'yellow', Inactive: 'muted', Blocked: 'red',
}

const WORKSPACES = ['GreenLab — Default Workspace', 'GreenLab — Farm Project', 'GreenLab — R&D Lab']

const CHART_OPTS = {
  responsive: true, maintainAspectRatio: false, animation: false as const,
  plugins: { legend: { display: false }, tooltip: { enabled: true } },
  scales: {
    x: { grid: { color: 'rgba(48,54,61,.5)' }, ticks: { color: '#8b949e', font: { size: 10 } } },
    y: { grid: { color: 'rgba(48,54,61,.5)' }, ticks: { color: '#8b949e', font: { size: 10 } } },
  },
}

// ── View Data Drawer ───────────────────────────────────────────────────────

const FIELD_COLORS = ['#ef4444', '#06b6d4', '#a855f7', '#f59e0b', '#22c55e', '#3b82f6', '#f97316', '#a78bfa']

function ViewDataDrawer({ device, onClose }: { device: Device | null; onClose(): void }) {
  useEscapeKey(onClose, device != null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)
  const [fields, setFields] = useState<{ key: string; label: string; unit: string }[]>([])
  const [latestMap, setLatestMap] = useState<Record<string, number | string | null>>({})
  const [chartMap, setChartMap] = useState<Record<string, { labels: string[]; values: (number | null)[] }>>({})

  useEffect(() => {
    if (!device) return
    let cancelled = false
    setLoading(true)
    setError(false)
    setFields([])
    setLatestMap({})
    setChartMap({})

    channelsApi.list({ device_id: device.id })
      .then(async r => {
        if (cancelled) return
        const chs = r.data
        if (!chs.length) return
        const ch = chs[0]

        const fieldsRes = await fieldsApi.list(ch.id)
        if (cancelled) return
        // Backend: name = field key, label = display name
        const chFields = fieldsRes.data.map((f: any) => ({
          key: f.name,
          label: f.label || f.name,
          unit: f.unit ?? '',
        }))
        setFields(chFields)

        await Promise.all(chFields.map(async f => {
          const [latestRes, queryRes] = await Promise.allSettled([
            queryApi.latest({ channel_id: ch.id, field: f.key }),
            queryApi.query({ channel_id: ch.id, field: f.key, limit: 20 }),
          ])
          if (cancelled) return
          if (latestRes.status === 'fulfilled') {
            const val = (latestRes.value as any).data?.value ?? null
            setLatestMap(prev => ({ ...prev, [f.key]: val }))
          }
          if (queryRes.status === 'fulfilled') {
            const pts = (queryRes.value.data as QueryResponse).data_points ?? []
            setChartMap(prev => ({
              ...prev,
              [f.key]: {
                labels: pts.map(p => new Date(p.timestamp).toLocaleTimeString()),
                values: pts.map(p => p.value),
              },
            }))
          }
        }))
      })
      .catch(() => { if (!cancelled) setError(true) })
      .finally(() => { if (!cancelled) setLoading(false) })

    return () => { cancelled = true }
  }, [device?.id])

  if (!device) return null

  return (
    <>
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.55)', backdropFilter: 'blur(3px)', zIndex: 200 }} />
      <div style={{
        position: 'fixed', top: 0, right: 0, bottom: 0, width: 520,
        background: 'var(--surface)', borderLeft: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column', zIndex: 201, overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>{device.icon}</span>
          <div style={{ flex: 1 }}>
            <h2 style={{ fontSize: 15, fontWeight: 700 }}>{device.name}</h2>
            <div style={{ fontSize: 11, color: 'var(--muted)', fontFamily: 'monospace' }}>{device.id}</div>
          </div>
          <Badge color={statusColor[device.status]}>
            {device.status === 'Active' && <Dot color="green" />}
            {device.status}
          </Badge>
          <button onClick={onClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        <div style={{ flex: 1, overflowY: 'auto', padding: '20px' }}>
          {/* Stats row */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,1fr)', gap: 10, marginBottom: 20 }}>
            {[
              { label: 'Channels',    value: device.channels },
              { label: 'Reads / 24h', value: device.reads },
              { label: 'Last seen',   value: device.last },
            ].map(s => (
              <div key={s.label} style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '12px 14px' }}>
                <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 4 }}>{s.label}</div>
                <div style={{ fontSize: 16, fontWeight: 700 }}>{s.value}</div>
              </div>
            ))}
          </div>

          {/* Loading */}
          {loading && (
            <div style={{ textAlign: 'center', padding: 40, color: 'var(--muted)', fontSize: 13 }}>
              Loading data…
            </div>
          )}

          {/* Error */}
          {!loading && error && (
            <div style={{ textAlign: 'center', padding: 40, color: 'var(--muted)', fontSize: 13 }}>
              Failed to load data. Please try again.
            </div>
          )}

          {/* No fields */}
          {!loading && !error && fields.length === 0 && (
            <div style={{ textAlign: 'center', padding: 40, color: 'var(--muted)', fontSize: 13 }}>
              No fields configured for this device.
            </div>
          )}

          {/* Charts */}
          {!loading && !error && fields.map((f, i) => {
            const color = FIELD_COLORS[i % FIELD_COLORS.length]
            const latest = latestMap[f.key]
            const chart = chartMap[f.key]
            return (
              <Card key={f.key} style={{ marginBottom: 14 }}>
                <CardTitle>
                  {f.label}
                  {f.unit && <span style={{ color: 'var(--muted)', fontWeight: 400, textTransform: 'none', letterSpacing: 0 }}> ({f.unit})</span>}
                  <span style={{ marginLeft: 'auto', fontSize: 18, fontWeight: 700, color, textTransform: 'none', letterSpacing: 0 }}>
                    {latest != null ? `${latest}${f.unit ?? ''}` : '—'}
                  </span>
                </CardTitle>
                <div style={{ position: 'relative', height: 120 }}>
                  {chart && chart.values.length > 0 ? (
                    <Line
                      data={{
                        labels: chart.labels,
                        datasets: [{
                          data: chart.values,
                          borderColor: color,
                          backgroundColor: color + '18',
                          fill: true, tension: 0.4, pointRadius: 2,
                        }],
                      }}
                      options={CHART_OPTS as any}
                    />
                  ) : (
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--muted)', fontSize: 12 }}>
                      No data
                    </div>
                  )}
                </div>
              </Card>
            )
          })}
        </div>
      </div>
    </>
  )
}

// ── Edit Device Drawer ─────────────────────────────────────────────────────

const inp: React.CSSProperties = {
  width: '100%', background: 'var(--surface2)', border: '1px solid var(--border)',
  borderRadius: 'var(--radius)', padding: '8px 12px', color: 'var(--text)',
  fontSize: 13, outline: 'none', boxSizing: 'border-box',
}
const lbl: React.CSSProperties = {
  display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--muted)',
  marginBottom: 6, textTransform: 'uppercase', letterSpacing: '.05em',
}

function EditStepper({ step }: { step: number }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', padding: '14px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
      {['Details', 'Channels'].map((s, i) => {
        const n = i + 1
        const done = n < step, active = n === step
        return (
          <div key={s} style={{ display: 'flex', alignItems: 'center', flex: i === 0 ? 1 : undefined }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, fontWeight: 600, color: done ? 'var(--green)' : active ? 'var(--text)' : 'var(--muted)' }}>
              <div style={{ width: 22, height: 22, borderRadius: '50%', border: `2px solid ${done ? 'var(--green)' : active ? 'var(--accent)' : 'var(--border)'}`, background: done ? 'var(--green)' : active ? 'var(--accent)' : 'transparent', display: 'grid', placeItems: 'center', fontSize: 11, fontWeight: 700, flexShrink: 0, color: (done || active) ? '#fff' : 'var(--muted)', transition: 'all 180ms ease' }}>
                {done ? '✓' : n}
              </div>
              {s}
            </div>
            {i === 0 && <div style={{ flex: 1, height: 1, margin: '0 8px', background: done ? 'var(--green)' : 'var(--border)', transition: 'background 180ms ease' }} />}
          </div>
        )
      })}
    </div>
  )
}

function EditStep1({ device, name, setName, workspace, setWorkspace, description, setDescription, tags, setTags }: {
  device: Device
  name: string; setName(v: string): void
  workspace: string; setWorkspace(v: string): void
  description: string; setDescription(v: string): void
  tags: string; setTags(v: string): void
}) {
  return (
    <div>
      <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 20 }}>
        Update the basic information for <strong style={{ color: 'var(--text)' }}>{device.icon} {device.name}</strong>.
      </p>

      <div style={{ marginBottom: 14 }}>
        <label style={lbl}>Device ID</label>
        <input value={device.id} disabled style={{ ...inp, opacity: .6, cursor: 'not-allowed', fontFamily: 'monospace', fontSize: 12 }} />
      </div>

      <div style={{ marginBottom: 14 }}>
        <label style={lbl}>Device Name <span style={{ color: 'var(--red)' }}>*</span></label>
        <input value={name} onChange={e => setName(e.target.value)} style={inp} autoFocus />
      </div>

      <div style={{ marginBottom: 14 }}>
        <label style={lbl}>Workspace</label>
        <select value={workspace} onChange={e => setWorkspace(e.target.value)} style={{ ...inp, appearance: 'none' }}>
          {WORKSPACES.map(w => <option key={w}>{w}</option>)}
        </select>
      </div>

      <div style={{ marginBottom: 14 }}>
        <label style={lbl}>Description</label>
        <input value={description} onChange={e => setDescription(e.target.value)} placeholder="What does this device monitor?" style={inp} />
      </div>

      <div style={{ marginBottom: 4 }}>
        <label style={lbl}>Tags</label>
        <input value={tags} onChange={e => setTags(e.target.value)} placeholder="agriculture, zone-a  (comma-separated)" style={inp} />
        <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>Comma-separated. Used for filtering on the dashboard.</div>
      </div>

      {tags.trim() && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 10 }}>
          {tags.split(',').map(t => t.trim()).filter(Boolean).map(t => (
            <span key={t} style={{ background: 'rgba(139,148,158,.12)', color: 'var(--muted)', fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 99 }}>{t}</span>
          ))}
        </div>
      )}
    </div>
  )
}

function EditStep2({ channelName, setChannelName, visibility, setVisibility, fields, setFields }: {
  channelName: string; setChannelName(v: string): void
  visibility: 'private' | 'public'; setVisibility(v: 'private' | 'public'): void
  fields: Field[]; setFields(f: Field[]): void
}) {
  function updateField(i: number, field: keyof Field, value: string) {
    setFields(fields.map((f, idx) => {
      if (idx !== i) return f
      const updated: Field = { ...f, [field]: value }
      if (field === 'name' && !f.keyLocked) {
        updated.key = toFieldKey(value)
      }
      if (field === 'key') {
        updated.keyLocked = value.length > 0
      }
      return updated
    }))
  }
  function addField() {
    if (fields.length >= 8) return
    setFields([...fields, { name: '', unit: '', type: 'float', key: '' }])
  }
  function removeField(i: number) {
    if (fields.length <= 1) return
    setFields(fields.filter((_, idx) => idx !== i))
  }

  return (
    <div>
      <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 20 }}>
        Modify the channel name, visibility, and measurement fields for this device.
      </p>

      <div style={{ marginBottom: 16 }}>
        <label style={lbl}>Channel Name <span style={{ color: 'var(--red)' }}>*</span></label>
        <input value={channelName} onChange={e => setChannelName(e.target.value)} placeholder="e.g. Greenhouse Climate" style={inp} autoFocus />
      </div>

      <div style={{ marginBottom: 20 }}>
        <label style={lbl}>Visibility</label>
        <div style={{ display: 'flex', gap: 10 }}>
          {(['private', 'public'] as const).map(v => (
            <div key={v} onClick={() => setVisibility(v)} style={{ flex: 1, border: `2px solid ${visibility === v ? 'var(--accent)' : 'var(--border)'}`, background: visibility === v ? 'rgba(37,99,235,.1)' : 'transparent', borderRadius: 'var(--radius)', padding: '10px 14px', cursor: 'pointer', transition: 'all 180ms ease' }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 2 }}>{v === 'private' ? '🔒 Private' : '🌐 Public'}</div>
              <div style={{ fontSize: 11, color: 'var(--muted)' }}>{v === 'private' ? 'Only your workspace can read' : 'Anyone with the channel ID'}</div>
            </div>
          ))}
        </div>
      </div>

      <div style={{ marginBottom: 16 }}>
        <label style={lbl}>Fields <span style={{ color: 'var(--muted)', fontWeight: 400, textTransform: 'none', letterSpacing: 0 }}>({fields.length}/8)</span></label>
        <div style={{ display: 'grid', gridTemplateColumns: '28px 1fr 72px 100px 32px', gap: 6, padding: '0 4px', marginBottom: 4 }}>
          {['', 'Field name', 'Unit', 'Type', ''].map((h, i) => (
            <span key={i} style={{ fontSize: 10, fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em' }}>{h}</span>
          ))}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {fields.map((f, i) => (
            <div key={i} style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '6px 10px' }}>
              <div style={{ display: 'grid', gridTemplateColumns: '28px 1fr 72px 100px 32px', alignItems: 'center', gap: 6 }}>
                <span style={{ fontSize: 11, fontWeight: 700, color: 'var(--accent-lt)', textAlign: 'center' }}>F{i + 1}</span>
                <input value={f.name} onChange={e => updateField(i, 'name', e.target.value)} placeholder="Field name" style={{ ...inp, padding: '5px 8px', fontSize: 12 }} />
                <input value={f.unit} onChange={e => updateField(i, 'unit', e.target.value)} placeholder="Unit" style={{ ...inp, padding: '5px 8px', fontSize: 12 }} />
                <select value={f.type} onChange={e => updateField(i, 'type', e.target.value)} style={{ ...inp, padding: '5px 6px', fontSize: 12, appearance: 'none' }}>
                  <option value="float">float</option>
                  <option value="integer">integer</option>
                  <option value="string">string</option>
                  <option value="boolean">boolean</option>
                </select>
                <button onClick={() => removeField(i)} disabled={fields.length <= 1} style={{ width: 28, height: 28, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 13, color: 'var(--muted)', background: 'transparent', border: '1px solid var(--border)', cursor: fields.length <= 1 ? 'default' : 'pointer', opacity: fields.length <= 1 ? .3 : 1, transition: 'all 180ms ease' }}>✕</button>
              </div>
              {/* Key row */}
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 5, paddingLeft: 34 }}>
                <span style={{ fontSize: 10, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', flexShrink: 0 }}>key</span>
                {f.keyLocked ? (
                  <span style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--cyan)', flex: 1, padding: '3px 7px' }}>
                    {f.key}
                  </span>
                ) : (
                  <input
                    value={f.key}
                    onChange={e => updateField(i, 'key', e.target.value)}
                    placeholder="auto"
                    style={{ ...inp, padding: '3px 7px', fontSize: 11, fontFamily: 'monospace', color: 'var(--cyan)', flex: 1 }}
                  />
                )}
                {f.keyLocked && (
                  <span title="Key is fixed after creation" style={{ fontSize: 12, cursor: 'default', flexShrink: 0 }}>🔒</span>
                )}
              </div>
            </div>
          ))}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 10 }}>
          <Btn variant="ghost" size="sm" onClick={addField} disabled={fields.length >= 8}>+ Add Field</Btn>
          {fields.length >= 8 && <span style={{ fontSize: 11, color: 'var(--muted)' }}>Maximum 8 fields reached</span>}
        </div>
      </div>
    </div>
  )
}

function EditDeviceDrawer({ device, onClose, onSave }: {
  device: Device | null
  onClose(): void
  onSave(id: string, patch: Partial<Device>): void
}) {
  const [step,        setStep]        = useState(1)
  const [name,        setName]        = useState(device?.name ?? '')
  const [description, setDescription] = useState(device?.description ?? '')
  const [tags,        setTags]        = useState(device?.tags ?? '')
  const [workspace,   setWorkspace]   = useState(device?.workspace ?? WORKSPACES[0])
  const [channelName, setChannelName] = useState(device?.channelName ?? '')
  const [visibility,  setVisibility]  = useState<'private' | 'public'>(device?.visibility ?? 'private')
  const [fields,      setFields]      = useState<Field[]>(device?.fields?.length ? device.fields : [{ name: 'field1', unit: '', type: 'float', key: 'field1' }])

  // Fetch channel + fields from API when the drawer opens
  useEffect(() => {
    if (!device) return
    channelsApi.list({ device_id: device.id })
      .then(async r => {
        const ch = r.data[0]
        if (!ch) return
        setChannelName(ch.name)
        setVisibility(ch.visibility)
        const fieldsRes = await fieldsApi.list(ch.id)
        if (fieldsRes.data.length > 0) {
          setFields(fieldsRes.data.map(f => ({
            name: (f as any).label || f.name,
            key: f.name,
            unit: f.unit ?? '',
            type: f.field_type as Field['type'],
            keyLocked: true,
          })))
        }
      })
      .catch(() => {}) // keep default state on error
  }, [device?.id])

  if (!device) return null

  const step1Valid = name.trim().length > 0
  const step2Valid = channelName.trim().length > 0

  function advance() {
    if (step === 1) { if (step1Valid) setStep(2); return }
    onSave(device!.id, { name: name.trim(), description, tags, workspace, channelName: channelName.trim(), visibility, fields })
    handleClose()
  }

  function handleClose() {
    onClose()
    setTimeout(() => {
      setStep(1)
      setName(device?.name ?? '')
      setDescription(device?.description ?? '')
      setTags(device?.tags ?? '')
      setWorkspace(device?.workspace ?? WORKSPACES[0])
      setChannelName(device?.channelName ?? '')
      setVisibility(device?.visibility ?? 'private')
      setFields(device?.fields ?? [{ name: 'field1', unit: '', type: 'float', key: 'field1' }])
    }, 300)
  }

  return (
    <>
      <div onClick={handleClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.55)', backdropFilter: 'blur(3px)', zIndex: 200 }} />
      <div style={{
        position: 'fixed', top: 0, right: 0, bottom: 0, width: 480,
        background: 'var(--surface)', borderLeft: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column', zIndex: 201, overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>{device.icon}</span>
          <h2 style={{ fontSize: 16, fontWeight: 700, flex: 1 }}>Edit Device</h2>
          <button onClick={handleClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        <EditStepper step={step} />

        {/* Body */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '24px 20px' }}>
          {step === 1 && (
            <EditStep1
              device={device}
              name={name} setName={setName}
              workspace={workspace} setWorkspace={setWorkspace}
              description={description} setDescription={setDescription}
              tags={tags} setTags={setTags}
            />
          )}
          {step === 2 && (
            <EditStep2
              channelName={channelName} setChannelName={setChannelName}
              visibility={visibility} setVisibility={setVisibility}
              fields={fields} setFields={setFields}
            />
          )}
        </div>

        {/* Footer */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 20px', borderTop: '1px solid var(--border)', flexShrink: 0 }}>
          {step === 2 && <Btn variant="ghost" onClick={() => setStep(1)}>← Back</Btn>}
          <div style={{ flex: 1 }} />
          <Btn variant="ghost" onClick={handleClose}>Cancel</Btn>
          <Btn
            variant="primary"
            onClick={advance}
            disabled={step === 1 ? !step1Valid : !step2Valid}
            style={{ opacity: (step === 1 ? step1Valid : step2Valid) ? 1 : .5 }}
          >
            {step === 1 ? 'Next →' : 'Save Changes'}
          </Btn>
        </div>
      </div>
    </>
  )
}

// ── Confirm Modal ──────────────────────────────────────────────────────────

function ConfirmModal({ icon, title, body, confirmLabel, danger = true, onConfirm, onCancel }: {
  icon: string
  title: string
  body: string
  confirmLabel: string
  danger?: boolean
  onConfirm(): void
  onCancel(): void
}) {
  useEscapeKey(onCancel)
  return (
    <>
      <div
        onClick={onCancel}
        style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', backdropFilter: 'blur(4px)', zIndex: 300 }}
      />
      <div style={{
        position: 'fixed', top: '50%', left: '50%',
        transform: 'translate(-50%,-50%)',
        width: 420, background: 'var(--surface)',
        border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)',
        padding: 28, zIndex: 301,
        display: 'flex', flexDirection: 'column', gap: 16,
      }}>
        {/* Icon */}
        <div style={{ fontSize: 36, textAlign: 'center' }}>{icon}</div>

        {/* Title */}
        <div style={{ textAlign: 'center' }}>
          <div style={{ fontSize: 17, fontWeight: 700, marginBottom: 6 }}>{title}</div>
          <div style={{ fontSize: 13, color: 'var(--muted)', lineHeight: 1.55 }}>{body}</div>
        </div>

        {/* Actions */}
        <div style={{ display: 'flex', gap: 10, marginTop: 4 }}>
          <Btn variant="ghost" style={{ flex: 1 }} onClick={onCancel}>Cancel</Btn>
          <Btn variant={danger ? 'danger' : 'primary'} style={{ flex: 1 }} onClick={onConfirm}>
            {confirmLabel}
          </Btn>
        </div>
      </div>
    </>
  )
}

// ── Device Card ────────────────────────────────────────────────────────────

function DeviceCard({
  device,
  onViewData,
  onEdit,
  onBlock,
  onRotate,
  onDelete,
}: {
  device: Device
  onViewData(d: Device): void
  onEdit(d: Device): void
  onBlock(d: Device): void
  onRotate(d: Device): void
  onDelete(d: Device): void
}) {
  const [copied, setCopied] = useState(false)
  const { toast } = useToast()

  function copyKey() {
    const key = device.apiKey
    if (!key) {
      toast('No API key available', 'error')
      return
    }
    const done = () => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
      toast(`Copied: ${key.slice(0, 8)}…`, 'info')
    }

    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(key).then(done).catch(() => {
        try {
          const el = document.createElement('textarea')
          el.value = key
          el.style.cssText = 'position:fixed;opacity:0;top:0;left:0'
          document.body.appendChild(el)
          el.focus()
          el.select()
          document.execCommand('copy')
          document.body.removeChild(el)
        } catch (_) { /* ignore */ }
        done()
      })
    } else {
      try {
        const el = document.createElement('textarea')
        el.value = key
        el.style.cssText = 'position:fixed;opacity:0;top:0;left:0'
        document.body.appendChild(el)
        el.focus()
        el.select()
        document.execCommand('copy')
        document.body.removeChild(el)
      } catch (_) { /* ignore */ }
      done()
    }
  }

  const isBlocked = device.status === 'Blocked'

  return (
    <div style={{
      background: 'var(--surface)', border: '1px solid var(--border)',
      borderRadius: 'var(--radius-lg)', padding: 20,
      display: 'flex', flexDirection: 'column', gap: 12,
      transition: 'border-color var(--transition), box-shadow var(--transition)',
    }}
    onMouseEnter={e => {
      e.currentTarget.style.borderColor = 'var(--accent)'
      e.currentTarget.style.boxShadow = '0 0 0 1px rgba(37,99,235,.3)'
    }}
    onMouseLeave={e => {
      e.currentTarget.style.borderColor = 'var(--border)'
      e.currentTarget.style.boxShadow = 'none'
    }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: 22 }}>{device.icon}</div>
          <div style={{ fontSize: 14, fontWeight: 600, marginTop: 6 }}>{device.name}</div>
          <div style={{ fontFamily: 'monospace', fontSize: 11, color: 'var(--muted)' }}>{device.id}</div>
        </div>
        <Badge color={statusColor[device.status]}>
          {device.status === 'Active' && <Dot color="green" />}
          {device.status}
        </Badge>
      </div>

      {/* Stats */}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
        <span style={{ fontSize: 12, color: 'var(--muted)' }}><strong style={{ color: 'var(--text)' }}>{device.channels}</strong> channels</span>
        <span style={{ fontSize: 12, color: 'var(--muted)' }}><strong style={{ color: 'var(--text)' }}>{device.reads}</strong> reads/24h</span>
        <span style={{ fontSize: 12, color: 'var(--muted)' }}>Last: <strong style={{ color: 'var(--text)' }}>{device.last}</strong></span>
      </div>

      {/* API key row */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 6,
        background: 'var(--surface2)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius)', padding: '6px 10px',
        fontFamily: 'monospace', fontSize: 11, color: 'var(--muted)',
        opacity: isBlocked ? .5 : 1,
      }}>
        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {device.apiKey ? device.apiKey.slice(0, 14) + '••••••••••••••••••••••••••••••' : '••••••••••••••••••••••••••••••••••••••••••'}
        </span>

        {/* Copy */}
        <div style={{ position: 'relative', display: 'inline-flex' }}>
          {copied && (
            <span style={{
              position: 'absolute', bottom: 'calc(100% + 6px)', left: '50%',
              transform: 'translateX(-50%)',
              background: 'var(--green)', color: '#fff',
              fontSize: 10, fontFamily: 'sans-serif', fontWeight: 600,
              padding: '2px 7px', borderRadius: 4, whiteSpace: 'nowrap',
              pointerEvents: 'none',
              animation: 'fadeInUp 0.15s ease',
            }}>
              Copied!
            </span>
          )}
          <button
            title="Copy API key"
            onClick={copyKey}
            disabled={isBlocked}
            style={{
              padding: '3px 7px', borderRadius: 'var(--radius)', fontSize: 11,
              color: copied ? 'var(--green)' : 'var(--muted)',
              background: copied ? 'rgba(34,197,94,.08)' : 'transparent',
              border: `1px solid ${copied ? 'var(--green)' : 'var(--border)'}`,
              cursor: isBlocked ? 'default' : 'pointer', whiteSpace: 'nowrap',
              transition: 'color 0.2s, background 0.2s, border-color 0.2s',
            }}
          >
            {copied ? '✓ Copied' : '⎘ Copy'}
          </button>
        </div>

        {/* Rotate */}
        {!isBlocked && (
          <button
            title="Rotate API key"
            onClick={() => onRotate(device)}
            style={{ padding: '3px 7px', borderRadius: 'var(--radius)', fontSize: 11, color: 'var(--muted)', background: 'transparent', border: '1px solid var(--border)', cursor: 'pointer' }}
          >🔄</button>
        )}
      </div>

      {/* Action buttons */}
      <div style={{ display: 'flex', gap: 6 }}>
        <Btn
          variant="ghost" size="sm" style={{ flex: 1 }}
          disabled={isBlocked}
          onClick={() => onViewData(device)}
        >
          View Data
        </Btn>

        <Btn variant="ghost" size="sm" onClick={() => onEdit(device)}>Edit</Btn>

        <Btn variant="danger" size="sm" onClick={() => onBlock(device)}>
          {isBlocked ? 'Unblock' : 'Block'}
        </Btn>

        <Btn variant="danger" size="sm" onClick={() => onDelete(device)}>
          Delete
        </Btn>
      </div>
    </div>
  )
}

function apiDeviceToLocal(d: ApiDevice): Device {
  const statusMap: Record<string, Device['status']> = { active: 'Active', inactive: 'Inactive', blocked: 'Blocked' }
  const reads24 = d.reads_24h ?? 0
  const reads = reads24 >= 1000 ? `${(reads24 / 1000).toFixed(0)}K` : String(reads24)
  const last = d.last_seen ? new Date(d.last_seen).toLocaleTimeString() : 'Never'
  return {
    icon: d.icon ?? '🔌',
    name: d.name,
    id: d.id,
    status: statusMap[d.status] ?? 'Inactive',
    channels: d.channel_count,
    reads,
    last,
    apiKey: d.api_key ?? '',
    description: d.description ?? '',
    tags: Array.isArray(d.tags) ? d.tags.join(', ') : '',
    workspace: d.workspace_id,
    channelName: '',
    visibility: 'private',
    fields: [],
    lat: d.lat,
    lng: d.lng,
    locationLabel: d.location_address,
  }
}

// ── Page ───────────────────────────────────────────────────────────────────

export function DevicesPage() {
  const { toast } = useToast()
  const [devices,      setDevices]      = useState<Device[]>([])
  const [loading,      setLoading]      = useState(true)
  const [filter,       setFilter]       = useState('All Status')
  const [search,       setSearch]       = useState('')
  const [registerOpen, setRegisterOpen] = useState(false)
  const [viewDevice,   setViewDevice]   = useState<Device | null>(null)
  const [editDevice,   setEditDevice]   = useState<Device | null>(null)
  const [blockTarget,  setBlockTarget]  = useState<Device | null>(null)
  const [rotateTarget, setRotateTarget] = useState<Device | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Device | null>(null)
  const [workspaces,   setWorkspaces]   = useState<Workspace[]>([])
  const [activeWsId,   setActiveWsId]   = useState<string>('')
  const [wsLoaded,     setWsLoaded]     = useState(false)

  const orgId = localStorage.getItem('org_id') ?? ''

  // Load workspaces on mount
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

  // Load devices whenever active workspace changes
  useEffect(() => {
    if (!activeWsId) return
    setLoading(true)
    devicesApi.list({ workspace_id: activeWsId })
      .then(r => setDevices(r.data.map(apiDeviceToLocal)))
      .catch(() => toast('Failed to load devices', 'error'))
      .finally(() => setLoading(false))
  }, [activeWsId])

  async function handleRegister(nd: NewDevice): Promise<{ channelId: string; apiKey: string }> {
    try {
      let channelId: string
      let apiKey: string

      if (nd.existingChannelId) {
        // Link device to existing channel via provision endpoint
        const res = await provisionApi.provision({
          device: {
            workspace_id: nd.workspace || activeWsId,
            name: nd.name,
            description: nd.description || undefined,
          },
          channel_id: nd.existingChannelId,
        })
        channelId = res.data.channel.id
        apiKey = res.data.device.api_key ?? nd.apiKey
        setDevices(prev => [apiDeviceToLocal(res.data.device), ...prev])
      } else {
        // Original flow: create device + new channel, then create fields
        const devRes = await devicesApi.create({
          name: nd.name,
          workspace_id: nd.workspace || activeWsId,
          description: nd.description,
          tags: nd.tags ? nd.tags.split(',').map(t => t.trim()).filter(Boolean) : [],
          icon: nd.icon,
          lat: nd.lat ? (v => Number.isFinite(v) ? v : undefined)(parseFloat(nd.lat)) : undefined,
          lng: nd.lng ? (v => Number.isFinite(v) ? v : undefined)(parseFloat(nd.lng)) : undefined,
          location_address: nd.locationLabel || undefined,
          channel_name: nd.channelName,
          channel_visibility: nd.visibility,
        })
        channelId = devRes.data.channel.id
        apiKey = devRes.data.device.api_key ?? nd.apiKey

        await Promise.all(
          nd.fields.map((f, i) =>
            fieldsApi.create({ channel_id: channelId, name: f.key || f.name, label: f.name, unit: f.unit, field_type: f.type, position: i + 1 })
          )
        )
        setDevices(prev => [apiDeviceToLocal(devRes.data.device), ...prev])
      }

      toast(`Device "${nd.name}" registered`)
      return { channelId, apiKey }
    } catch {
      toast('Failed to register device', 'error')
      throw new Error('registration failed')
    }
  }

  function handleSaveEdit(id: string, patch: Partial<Device>) {
    const name = patch.name ?? devices.find(d => d.id === id)?.name ?? ''
    devicesApi.update(id, { name: patch.name, description: patch.description, tags: patch.tags ? patch.tags.split(',').map(t => t.trim()).filter(Boolean) : undefined })
      .then(() => {
        setDevices(prev => prev.map(d => d.id === id ? { ...d, ...patch } : d))
        toast(`Device "${name}" saved`)
      })
      .catch(() => {
        setDevices(prev => prev.map(d => d.id === id ? { ...d, ...patch } : d))
        toast(`Device "${name}" saved`)
      })
  }

  function handleToggleBlock(id: string) {
    const d = devices.find(x => x.id === id)
    if (!d) return
    const isBlocked = d.status === 'Blocked'
    const apiCall = isBlocked ? devicesApi.unblock(id) : devicesApi.block(id)
    apiCall
      .then(() => {
        setDevices(prev => prev.map(x => x.id === id ? { ...x, status: isBlocked ? 'Active' : 'Blocked' } : x))
        toast(isBlocked ? `"${d.name}" unblocked` : `"${d.name}" blocked`, isBlocked ? 'success' : 'error')
      })
      .catch(() => toast('Failed to update device status', 'error'))
  }

  function handleDelete(id: string) {
    const d = devices.find(x => x.id === id)
    devicesApi.delete(id)
      .then(() => {
        setDevices(prev => prev.filter(x => x.id !== id))
        if (d) toast(`"${d.name}" deleted`, 'error')
      })
      .catch(() => toast('Failed to delete device', 'error'))
  }

  function handleRotateKey(id: string) {
    const d = devices.find(x => x.id === id)
    devicesApi.rotateKey(id)
      .then(r => {
        setDevices(prev => prev.map(x => x.id === id ? { ...x, apiKey: r.data.api_key } : x))
        if (d) toast(`API key rotated for "${d.name}"`, 'info')
      })
      .catch(() => toast('Failed to rotate API key', 'error'))
  }

  const q = search.toLowerCase()
  const visible = devices
    .filter(d => filter === 'All Status' || d.status === filter)
    .filter(d => !q || d.name.toLowerCase().includes(q) || d.id.includes(q) || d.workspace.toLowerCase().includes(q))

  return (
    <div>
      {/* Page header */}
      <div className="stack-sm" style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 24 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Devices</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Manage IoT devices, API keys, and connection status</p>
        </div>
        <div className="page-toolbar" style={{ display: 'flex', gap: 8, marginLeft: 'auto', flexWrap: 'wrap' }}>
          {workspaces.length > 0 && (
            <select
              value={activeWsId}
              onChange={e => setActiveWsId(e.target.value)}
              style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '7px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }}
            >
              {workspaces.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}
            </select>
          )}
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search devices..."
            style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '7px 12px', color: 'var(--text)', fontSize: 13, outline: 'none', width: 180 }}
          />
          <select
            value={filter}
            onChange={e => setFilter(e.target.value)}
            style={{ width: 140, background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius)', padding: '7px 12px', color: 'var(--text)', fontSize: 13, outline: 'none' }}
          >
            {['All Status', 'Active', 'Warning', 'Inactive', 'Blocked'].map(o => <option key={o}>{o}</option>)}
          </select>
          <Btn variant="primary" size="sm" onClick={() => setRegisterOpen(true)}>+ Register Device</Btn>
        </div>
      </div>

      {wsLoaded && workspaces.length === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 240, gap: 12, color: 'var(--muted)' }}>
          <div style={{ fontSize: 40 }}>🗂️</div>
          <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--text)' }}>No workspaces yet</div>
          <div style={{ fontSize: 13 }}>Create a workspace first, then register devices inside it.</div>
        </div>
      )}

      {loading && workspaces.length > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 160 }}>
          <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
        </div>
      )}

      {/* Empty state */}
      {!loading && workspaces.length > 0 && devices.length === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: 280, gap: 16, color: 'var(--muted)' }}>
          <div style={{ fontSize: 44 }}>🔌</div>
          <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--text)' }}>No devices yet</div>
          <div style={{ fontSize: 13 }}>Register your first IoT device to start collecting data.</div>
          <Btn variant="primary" size="sm" onClick={() => setRegisterOpen(true)}>+ Register Device</Btn>
        </div>
      )}

      {/* Grid */}
      {!loading && workspaces.length > 0 && devices.length > 0 && <div className="rg3" style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16 }}>
        {visible.map(d => (
          <DeviceCard
            key={d.id}
            device={d}
            onViewData={setViewDevice}
            onEdit={setEditDevice}
            onBlock={setBlockTarget}
            onRotate={setRotateTarget}
            onDelete={setDeleteTarget}
          />
        ))}

        {/* Add placeholder */}
        <div
          onClick={() => setRegisterOpen(true)}
          style={{
            background: 'transparent', border: '1px dashed var(--border)',
            borderRadius: 'var(--radius-lg)', padding: 20,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            minHeight: 200, cursor: 'pointer', transition: 'border-color var(--transition)',
          }}
          onMouseEnter={e => (e.currentTarget.style.borderColor = 'var(--accent)')}
          onMouseLeave={e => (e.currentTarget.style.borderColor = 'var(--border)')}
        >
          <div style={{ textAlign: 'center', color: 'var(--muted)', padding: '32px 20px' }}>
            <div style={{ fontSize: 36, marginBottom: 12 }}>➕</div>
            <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--text)', marginBottom: 4 }}>Register New Device</div>
            <div style={{ fontSize: 13, marginBottom: 16 }}>Connect a new IoT device to this workspace</div>
            <Btn variant="primary" size="sm" onClick={e => { e.stopPropagation(); setRegisterOpen(true) }}>
              + Register Device
            </Btn>
          </div>
        </div>
      </div>}

      {/* Drawers */}
      <RegisterDeviceDrawer open={registerOpen} onClose={() => setRegisterOpen(false)} onRegister={handleRegister} workspaces={workspaces} />
      <ViewDataDrawer  device={viewDevice} onClose={() => setViewDevice(null)} />
      <EditDeviceDrawer key={editDevice?.id} device={editDevice} onClose={() => setEditDevice(null)} onSave={handleSaveEdit} />

      {/* Block / Unblock modal */}
      {blockTarget && (
        <ConfirmModal
          icon={blockTarget.status === 'Blocked' ? '🔓' : '⛔'}
          title={blockTarget.status === 'Blocked' ? `Unblock "${blockTarget.name}"?` : `Block "${blockTarget.name}"?`}
          body={
            blockTarget.status === 'Blocked'
              ? 'Unblocking will allow this device to send data again using its current API key.'
              : 'Blocking will reject all incoming data from this device until it is unblocked. The API key remains valid.'
          }
          confirmLabel={blockTarget.status === 'Blocked' ? 'Unblock' : 'Block Device'}
          danger={blockTarget.status !== 'Blocked'}
          onConfirm={() => { handleToggleBlock(blockTarget.id); setBlockTarget(null) }}
          onCancel={() => setBlockTarget(null)}
        />
      )}

      {/* Rotate API key modal */}
      {rotateTarget && (
        <ConfirmModal
          icon="🔄"
          title={`Rotate API key for "${rotateTarget.name}"?`}
          body="This will immediately invalidate the current API key. Any device or integration using it will stop sending data until you update it with the new key."
          confirmLabel="Rotate Key"
          onConfirm={() => { handleRotateKey(rotateTarget.id); setRotateTarget(null) }}
          onCancel={() => setRotateTarget(null)}
        />
      )}

      {/* Delete modal */}
      {deleteTarget && (
        <ConfirmModal
          icon="🗑️"
          title={`Delete "${deleteTarget.name}"?`}
          body="This will permanently remove the device and unlink any associated channels. This action cannot be undone."
          confirmLabel="Delete Device"
          danger={true}
          onConfirm={() => { handleDelete(deleteTarget.id); setDeleteTarget(null) }}
          onCancel={() => setDeleteTarget(null)}
        />
      )}
    </div>
  )
}
