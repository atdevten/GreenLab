import { useState, useEffect } from 'react'
import { Badge } from '../components/ui/Badge'
import { Btn } from '../components/ui/Button'
import { Card, CardTitle } from '../components/ui/Card'
import { useToast } from '../contexts/ToastContext'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { alertsApi } from '../api/alerts'
import { workspacesApi } from '../api/workspaces'
import { devicesApi } from '../api/devices'
import type { AlertRule as ApiAlertRule, Device } from '../types'

// ── Types ────────────────────────────────────────────────────────────────────

type Severity = 'critical' | 'warning' | 'info'
type Operator = '>' | '<' | '>=' | '<=' | '==' | '!='
type NotifChannel = 'email' | 'telegram' | 'slack' | 'sms'

interface AlertRule {
  id: string
  icon: string
  title: string
  severity: Severity
  device: string
  field: string
  operator: Operator
  threshold: string
  unit: string
  notifications: NotifChannel[]
  active: boolean
  triggered: string
}

// ── Seed data ────────────────────────────────────────────────────────────────

const INITIAL_RULES: AlertRule[] = [
  { id: 'r1', icon: '🌡️', title: 'High Temp Critical', severity: 'critical', device: 'Greenhouse Sensor A', field: 'Temperature', operator: '>', threshold: '85', unit: '°C',     notifications: ['telegram','email'], active: true,  triggered: '2 min ago' },
  { id: 'r2', icon: '💧', title: 'Low Humidity Warning', severity: 'warning', device: 'Farm Node B',          field: 'Humidity',    operator: '<', threshold: '30', unit: '%',      notifications: ['email'],            active: true,  triggered: '18 min ago' },
  { id: 'r3', icon: '🌬️', title: 'High CO₂ Alert',      severity: 'warning', device: 'Air Monitor',          field: 'CO₂',         operator: '>', threshold: '1000', unit: 'ppm', notifications: ['slack'],             active: true,  triggered: '1 hr ago' },
  { id: 'r4', icon: '☀️', title: 'Solar Offline',        severity: 'info',    device: 'Solar Tracker',        field: 'field1',      operator: '==', threshold: '0', unit: '',      notifications: ['email'],            active: false, triggered: 'Never' },
  { id: 'r5', icon: '💧', title: 'Water pH Critical',    severity: 'critical', device: 'Water Quality Probe', field: 'pH',          operator: '<', threshold: '5.5', unit: '',     notifications: ['sms','email'],       active: true,  triggered: '2d ago' },
]

const DEVICES = ['Greenhouse Sensor A', 'Farm Node B', 'Air Monitor', 'Water Quality Probe', 'R&D Lab Node', 'Solar Tracker']
const DEVICE_FIELDS: Record<string, string[]> = {
  'Greenhouse Sensor A': ['Temperature', 'Humidity', 'CO₂', 'Light'],
  'Farm Node B':         ['Moisture', 'pH', 'Nitrogen', 'Phosphorus', 'Potassium'],
  'Air Monitor':         ['PM2.5', 'PM10', 'CO', 'NO₂', 'O₃', 'AQI'],
  'Water Quality Probe': ['Temp', 'pH', 'DO', 'Turbidity', 'TDS'],
  'R&D Lab Node':        ['Voltage', 'Current'],
  'Solar Tracker':       ['field1', 'field2'],
}

const SEVERITY_ICONS: Record<Severity, string> = { critical: '🔴', warning: '🟡', info: '🔵' }
const SEVERITY_COLORS: Record<Severity, 'red' | 'yellow' | 'blue'> = { critical: 'red', warning: 'yellow', info: 'blue' }
const OPERATORS: Operator[] = ['>', '<', '>=', '<=', '==', '!=']
const NOTIF_META: { key: NotifChannel; label: string; icon: string; color: string }[] = [
  { key: 'email',    label: 'Email',    icon: '📧', color: 'var(--accent)' },
  { key: 'telegram', label: 'Telegram', icon: '✈️',  color: '#229ED9' },
  { key: 'slack',    label: 'Slack',    icon: '💬', color: '#4A154B' },
  { key: 'sms',      label: 'SMS',      icon: '📱', color: 'var(--green)' },
]
const ICONS = ['🌡️','💧','🌬️','☀️','🔔','⚡','🏭','🌊','🌾','🔬','📡','⚠️']

// ── Shared styles ────────────────────────────────────────────────────────────

const labelStyle: React.CSSProperties = {
  display: 'block', fontSize: 11, fontWeight: 600,
  color: 'var(--muted)', marginBottom: 5,
  textTransform: 'uppercase', letterSpacing: '.05em',
}
const inputStyle: React.CSSProperties = {
  width: '100%', background: 'var(--surface2)',
  border: '1px solid var(--border)', borderRadius: 'var(--radius)',
  padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none',
  boxSizing: 'border-box',
}

// ── Stepper ──────────────────────────────────────────────────────────────────

function Stepper({ step }: { step: number }) {
  const steps = ['Rule Details', 'Condition & Notify']
  return (
    <div style={{ display: 'flex', alignItems: 'center', padding: '14px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
      {steps.map((s, i) => {
        const done = i < step; const active = i === step
        return (
          <div key={s} style={{ display: 'flex', alignItems: 'center', flex: i < steps.length - 1 ? 1 : undefined }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
              <div style={{
                width: 22, height: 22, borderRadius: '50%', fontSize: 11, fontWeight: 700,
                display: 'grid', placeItems: 'center', transition: 'all 180ms',
                background: done ? 'var(--green)' : active ? 'var(--accent)' : 'transparent',
                border: `2px solid ${done ? 'var(--green)' : active ? 'var(--accent)' : 'var(--border)'}`,
                color: (done || active) ? '#fff' : 'var(--muted)',
              }}>{done ? '✓' : i + 1}</div>
              <span style={{ fontSize: 12, fontWeight: active ? 600 : 400, color: active ? 'var(--text)' : 'var(--muted)' }}>{s}</span>
            </div>
            {i < steps.length - 1 && (
              <div style={{ flex: 1, height: 1, background: done ? 'var(--green)' : 'var(--border)', margin: '0 10px', transition: 'background 180ms' }} />
            )}
          </div>
        )
      })}
    </div>
  )
}

// ── Alert Rule Drawer (New & Edit) ───────────────────────────────────────────

function AlertRuleDrawer({ open, rule, devices, onClose, onSave }: {
  open: boolean
  rule: AlertRule | null   // null → create, non-null → edit
  devices: Device[]
  onClose(): void
  onSave(r: AlertRule): void
}) {
  const isEdit = rule != null
  const deviceOpts = devices.map(d => d.name)
  const deviceFieldMap: Record<string, string[]> = Object.fromEntries(
    devices.map(d => [d.name, [] as string[]])
  )

  const [step,   setStep]   = useState(0)
  const [title,  setTitle]  = useState(rule?.title ?? '')
  const [icon,   setIcon]   = useState(rule?.icon ?? '🔔')
  const [sev,    setSev]    = useState<Severity>(rule?.severity ?? 'warning')
  const [device, setDevice] = useState(rule?.device ?? deviceOpts[0] ?? '')
  const [field,  setField]  = useState(rule?.field ?? '')
  const [op,     setOp]     = useState<Operator>(rule?.operator ?? '>')
  const [thresh, setThresh] = useState(rule?.threshold ?? '')
  const [unit,   setUnit]   = useState(rule?.unit ?? '')
  const [notifs, setNotifs] = useState<NotifChannel[]>(rule?.notifications ?? ['email'])

  const fields = deviceFieldMap[device] ?? []
  const resolvedField = field || fields[0] || ''

  function reset() {
    setStep(0); setTitle(''); setIcon('🔔'); setSev('warning')
    setDevice(deviceOpts[0] ?? ''); setField(''); setOp('>'); setThresh(''); setUnit(''); setNotifs(['email'])
  }

  function handleClose() { onClose(); if (!isEdit) setTimeout(reset, 300) }

  function toggleNotif(key: NotifChannel) {
    setNotifs(prev => prev.includes(key) ? prev.filter(n => n !== key) : [...prev, key])
  }

  function handleSave() {
    onSave({
      id: rule?.id ?? `r_${Math.random().toString(36).slice(2, 8)}`,
      icon, title: title.trim(), severity: sev, device,
      field: resolvedField, operator: op,
      threshold: thresh, unit: unit.trim(),
      notifications: notifs,
      active: rule?.active ?? true,
      triggered: rule?.triggered ?? 'Never',
    })
    handleClose()
  }

  const step1Valid = title.trim().length > 0
  const step2Valid = thresh.trim().length > 0 && notifs.length > 0

  return (
    <>
      {open && <div onClick={handleClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.55)', backdropFilter: 'blur(3px)', zIndex: 200 }} />}
      <div style={{
        position: 'fixed', top: 0, right: 0, bottom: 0, width: 500,
        background: 'var(--surface)', borderLeft: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column', zIndex: 201, overflow: 'hidden',
        transform: open ? 'translateX(0)' : 'translateX(100%)',
        transition: 'transform 300ms cubic-bezier(.4,0,.2,1)',
      }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
          <span style={{ fontSize: 20 }}>{isEdit ? icon : '🔔'}</span>
          <h2 style={{ fontSize: 16, fontWeight: 700, flex: 1 }}>{isEdit ? 'Edit Alert Rule' : 'New Alert Rule'}</h2>
          <button onClick={handleClose} style={{ width: 30, height: 30, borderRadius: 'var(--radius)', display: 'grid', placeItems: 'center', fontSize: 18, color: 'var(--muted)', cursor: 'pointer', background: 'transparent', border: 'none' }}>✕</button>
        </div>

        <Stepper step={step} />

        {/* Body */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '24px 20px' }}>

          {/* ── Step 1: Rule Details ── */}
          {step === 0 && (
            <>
              <div style={{ marginBottom: 16 }}>
                <label style={labelStyle}>Rule Name <span style={{ color: 'var(--red)' }}>*</span></label>
                <input value={title} onChange={e => setTitle(e.target.value)} placeholder="e.g. High Temp Critical" style={inputStyle} autoFocus />
              </div>

              <div style={{ marginBottom: 16 }}>
                <label style={labelStyle}>Icon</label>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                  {ICONS.map(ic => (
                    <button
                      key={ic}
                      onClick={() => setIcon(ic)}
                      style={{
                        width: 36, height: 36, fontSize: 18, borderRadius: 'var(--radius)',
                        border: `2px solid ${icon === ic ? 'var(--accent)' : 'var(--border)'}`,
                        background: icon === ic ? 'rgba(37,99,235,.12)' : 'var(--surface2)',
                        cursor: 'pointer', display: 'grid', placeItems: 'center',
                        transition: 'all .15s',
                      }}
                    >{ic}</button>
                  ))}
                </div>
              </div>

              <div style={{ marginBottom: 16 }}>
                <label style={labelStyle}>Severity</label>
                <div style={{ display: 'flex', gap: 10 }}>
                  {(['warning', 'critical', 'info'] as Severity[]).map(s => (
                    <div
                      key={s}
                      onClick={() => setSev(s)}
                      style={{
                        flex: 1, padding: '10px 12px', borderRadius: 'var(--radius)', cursor: 'pointer',
                        border: `1px solid ${sev === s ? `var(--${SEVERITY_COLORS[s] === 'red' ? 'red' : SEVERITY_COLORS[s] === 'yellow' ? 'yellow' : 'accent'})` : 'var(--border)'}`,
                        background: sev === s ? `rgba(${s === 'critical' ? '239,68,68' : s === 'warning' ? '245,158,11' : '37,99,235'},.1)` : 'var(--surface2)',
                        transition: 'all .15s',
                      }}
                    >
                      <div style={{ fontSize: 16 }}>{SEVERITY_ICONS[s]}</div>
                      <div style={{ fontSize: 12, fontWeight: 600, marginTop: 4, textTransform: 'capitalize' }}>{s}</div>
                    </div>
                  ))}
                </div>
              </div>

              <div style={{ marginBottom: 16 }}>
                <label style={labelStyle}>Device</label>
                <select value={device} onChange={e => { setDevice(e.target.value); setField('') }} style={{ ...inputStyle, cursor: 'pointer' }}>
                  {deviceOpts.length === 0
                    ? <option value="">No devices available</option>
                    : deviceOpts.map(d => <option key={d} value={d}>{d}</option>)
                  }
                </select>
              </div>

              <div style={{ marginBottom: 16 }}>
                <label style={labelStyle}>Field</label>
                <select value={resolvedField} onChange={e => setField(e.target.value)} style={{ ...inputStyle, cursor: 'pointer' }}>
                  {fields.map(f => <option key={f} value={f}>{f}</option>)}
                </select>
              </div>
            </>
          )}

          {/* ── Step 2: Condition & Notifications ── */}
          {step === 1 && (
            <>
              <div style={{ marginBottom: 20 }}>
                <label style={labelStyle}>Condition <span style={{ color: 'var(--red)' }}>*</span></label>
                <div style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', padding: '14px 16px' }}>
                  <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 10 }}>
                    Trigger when <strong style={{ color: 'var(--text)' }}>{device} · {resolvedField}</strong> is:
                  </div>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <select value={op} onChange={e => setOp(e.target.value as Operator)} style={{ ...inputStyle, width: 80, textAlign: 'center', fontFamily: 'monospace', fontWeight: 700, cursor: 'pointer' }}>
                      {OPERATORS.map(o => <option key={o} value={o}>{o}</option>)}
                    </select>
                    <input
                      value={thresh}
                      onChange={e => setThresh(e.target.value)}
                      placeholder="threshold value"
                      type="number"
                      style={{ ...inputStyle, flex: 1 }}
                      autoFocus
                    />
                    <input
                      value={unit}
                      onChange={e => setUnit(e.target.value)}
                      placeholder="unit"
                      style={{ ...inputStyle, width: 70 }}
                    />
                  </div>
                  {thresh && (
                    <div style={{ marginTop: 10, fontSize: 12, padding: '6px 10px', background: 'rgba(37,99,235,.08)', border: '1px solid rgba(37,99,235,.2)', borderRadius: 'var(--radius)', color: 'var(--accent-lt)' }}>
                      🔔 Alert when <strong>{resolvedField} {op} {thresh}{unit ? ` ${unit}` : ''}</strong>
                    </div>
                  )}
                </div>
              </div>

              <div style={{ marginBottom: 20 }}>
                <label style={labelStyle}>Notify via <span style={{ color: 'var(--red)' }}>*</span></label>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
                  {NOTIF_META.map(n => {
                    const on = notifs.includes(n.key)
                    return (
                      <div
                        key={n.key}
                        onClick={() => toggleNotif(n.key)}
                        style={{
                          display: 'flex', alignItems: 'center', gap: 10,
                          padding: '10px 14px', borderRadius: 'var(--radius)', cursor: 'pointer',
                          border: `1px solid ${on ? n.color : 'var(--border)'}`,
                          background: on ? `${n.color}18` : 'var(--surface2)',
                          transition: 'all .15s',
                        }}
                      >
                        <span style={{ fontSize: 18 }}>{n.icon}</span>
                        <span style={{ fontSize: 13, fontWeight: on ? 600 : 400, color: on ? 'var(--text)' : 'var(--muted)' }}>{n.label}</span>
                        {on && <span style={{ marginLeft: 'auto', fontSize: 12, color: n.color, fontWeight: 700 }}>✓</span>}
                      </div>
                    )
                  })}
                </div>
                {notifs.length === 0 && (
                  <div style={{ fontSize: 12, color: 'var(--red)', marginTop: 6 }}>Select at least one notification channel.</div>
                )}
              </div>

              {/* Summary */}
              <div style={{ background: 'var(--surface2)', border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', padding: '14px 16px' }}>
                <div style={{ fontSize: 11, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.05em', marginBottom: 10 }}>Summary</div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {[
                    { k: 'Rule',      v: title || '—' },
                    { k: 'Device',    v: device },
                    { k: 'Condition', v: thresh ? `${resolvedField} ${op} ${thresh}${unit ? ' ' + unit : ''}` : '—' },
                    { k: 'Severity',  v: `${SEVERITY_ICONS[sev]} ${sev}` },
                    { k: 'Notify',    v: notifs.length ? notifs.join(', ') : '—' },
                  ].map(row => (
                    <div key={row.k} style={{ display: 'flex', gap: 8, fontSize: 12 }}>
                      <span style={{ color: 'var(--muted)', minWidth: 80 }}>{row.k}</span>
                      <span style={{ fontWeight: 600 }}>{row.v}</span>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </div>

        {/* Footer */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 20px', borderTop: '1px solid var(--border)', flexShrink: 0 }}>
          <div style={{ flex: 1 }} />
          <Btn variant="ghost" onClick={handleClose}>Cancel</Btn>
          {step === 0 ? (
            <Btn variant="primary" onClick={() => setStep(1)} disabled={!step1Valid} style={{ opacity: step1Valid ? 1 : .5 }}>
              Next: Condition →
            </Btn>
          ) : (
            <>
              <Btn variant="ghost" onClick={() => setStep(0)}>← Back</Btn>
              <Btn variant="primary" onClick={handleSave} disabled={!step2Valid} style={{ opacity: step2Valid ? 1 : .5 }}>
                {isEdit ? 'Save Changes' : 'Create Rule'}
              </Btn>
            </>
          )}
        </div>
      </div>
    </>
  )
}

// ── Delete Confirm Modal ──────────────────────────────────────────────────────

function DeleteModal({ rule, onClose, onConfirm }: {
  rule: AlertRule | null
  onClose(): void
  onConfirm(id: string): void
}) {
  useEscapeKey(onClose, rule != null)
  if (!rule) return null
  return (
    <>
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', backdropFilter: 'blur(4px)', zIndex: 300 }} />
      <div style={{
        position: 'fixed', top: '50%', left: '50%', transform: 'translate(-50%,-50%)',
        width: 420, background: 'var(--surface)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius-lg)', padding: 24, zIndex: 301,
      }}>
        <div style={{ fontSize: 28, marginBottom: 12 }}>{rule.icon}</div>
        <h3 style={{ fontSize: 16, fontWeight: 700, marginBottom: 8 }}>Delete Alert Rule</h3>
        <p style={{ fontSize: 13, color: 'var(--muted)', lineHeight: 1.6, marginBottom: 16 }}>
          Delete <strong style={{ color: 'var(--text)' }}>{rule.title}</strong>?
          This rule will stop monitoring <strong style={{ color: 'var(--text)' }}>{rule.device} · {rule.field}</strong> and
          all associated notifications will be disabled. This cannot be undone.
        </p>
        <div style={{ background: 'rgba(239,68,68,.08)', border: '1px solid rgba(239,68,68,.2)', borderRadius: 'var(--radius)', padding: '8px 12px', marginBottom: 20, fontSize: 12, color: 'var(--red)' }}>
          ⚠️ Severity: <strong>{SEVERITY_ICONS[rule.severity]} {rule.severity}</strong> · Last triggered: <strong>{rule.triggered}</strong>
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
          <Btn variant="ghost" onClick={onClose}>Cancel</Btn>
          <Btn variant="danger" onClick={() => { onConfirm(rule.id); onClose() }}>Delete Rule</Btn>
        </div>
      </div>
    </>
  )
}

// ── Toggle Confirm Modal ──────────────────────────────────────────────────────

function ToggleModal({ rule, onClose, onConfirm }: {
  rule: AlertRule | null
  onClose(): void
  onConfirm(id: string): void
}) {
  useEscapeKey(onClose, rule != null)
  if (!rule) return null
  const enabling = !rule.active
  return (
    <>
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.6)', backdropFilter: 'blur(4px)', zIndex: 300 }} />
      <div style={{
        position: 'fixed', top: '50%', left: '50%', transform: 'translate(-50%,-50%)',
        width: 400, background: 'var(--surface)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius-lg)', padding: 24, zIndex: 301,
      }}>
        <div style={{ fontSize: 28, marginBottom: 12 }}>{rule.icon}</div>
        <h3 style={{ fontSize: 16, fontWeight: 700, marginBottom: 8 }}>
          {enabling ? 'Enable' : 'Disable'} Alert Rule
        </h3>
        <p style={{ fontSize: 13, color: 'var(--muted)', lineHeight: 1.6, marginBottom: 16 }}>
          {enabling
            ? <>Enable <strong style={{ color: 'var(--text)' }}>{rule.title}</strong>? This will start monitoring <strong style={{ color: 'var(--text)' }}>{rule.device} · {rule.field}</strong> and send notifications when the condition is met.</>
            : <>Disable <strong style={{ color: 'var(--text)' }}>{rule.title}</strong>? This will pause monitoring <strong style={{ color: 'var(--text)' }}>{rule.device} · {rule.field}</strong> and no notifications will be sent.</>
          }
        </p>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
          <Btn variant="ghost" onClick={onClose}>Cancel</Btn>
          <Btn variant={enabling ? 'primary' : 'danger'} onClick={() => { onConfirm(rule.id); onClose() }}>
            {enabling ? 'Enable Rule' : 'Disable Rule'}
          </Btn>
        </div>
      </div>
    </>
  )
}

function apiRuleToLocal(r: ApiAlertRule): AlertRule {
  return {
    id: r.id,
    icon: '🔔',
    title: r.name,
    severity: r.severity,
    device: r.device_id,
    field: r.field_id,
    operator: r.operator,
    threshold: String(r.threshold),
    unit: r.unit ?? '',
    notifications: (r.notification_channels ?? []) as NotifChannel[],
    active: r.enabled,
    triggered: r.last_triggered ?? 'Never',
  }
}

const Spinner = () => (
  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 160 }}>
    <div style={{ width: 24, height: 24, border: '2px solid var(--accent)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 0.7s linear infinite' }} />
  </div>
)

// ── Page ─────────────────────────────────────────────────────────────────────

export function AlertsPage() {
  const { toast } = useToast()
  const [rules,         setRules]        = useState<AlertRule[]>([])
  const [loading,       setLoading]      = useState(true)
  const [drawerOpen,    setDrawerOpen]   = useState(false)
  const [editTarget,    setEditTarget]   = useState<AlertRule | null>(null)
  const [deleteTarget,  setDeleteTarget] = useState<AlertRule | null>(null)
  const [toggleTarget,  setToggleTarget] = useState<AlertRule | null>(null)
  const [wsId,          setWsId]         = useState('')
  const [devices,       setDevices]      = useState<Device[]>([])

  useEffect(() => {
    const orgId = localStorage.getItem('org_id') ?? ''
    if (!orgId) { setLoading(false); return }
    workspacesApi.list(orgId)
      .then(r => {
        const id = r.data[0]?.id
        if (!id) { setLoading(false); return }
        setWsId(id)
        return Promise.all([
          alertsApi.list({ workspace_id: id }),
          devicesApi.list({ workspace_id: id }),
        ]).then(([rulesRes, devicesRes]) => {
          setRules(rulesRes.data.map(apiRuleToLocal))
          setDevices(devicesRes.data)
        })
      })
      .catch(() => toast('Failed to load alert rules', 'error'))
      .finally(() => setLoading(false))
  }, [])

  const activeCount   = rules.filter(r => r.active).length
  const criticalCount = rules.filter(r => r.severity === 'critical' && r.active).length
  const recentCount   = rules.filter(r => r.triggered !== 'Never' && r.active).length

  function handleSave(r: AlertRule) {
    const isEdit = rules.some(x => x.id === r.id)
    if (isEdit) {
      alertsApi.update(r.id, { name: r.title, severity: r.severity, operator: r.operator, threshold: Number(r.threshold), unit: r.unit, notification_channels: r.notifications, enabled: r.active })
        .then(res => {
          setRules(prev => prev.map(x => x.id === r.id ? apiRuleToLocal(res.data) : x))
          toast(`Rule "${r.title}" updated`)
        })
        .catch(() => toast('Failed to update rule', 'error'))
    } else {
      alertsApi.create({ workspace_id: wsId, name: r.title, device_id: r.device, field_id: r.field, operator: r.operator, threshold: Number(r.threshold), unit: r.unit, severity: r.severity, enabled: r.active, notification_channels: r.notifications })
        .then(res => {
          setRules(prev => [...prev, apiRuleToLocal(res.data)])
          toast(`Rule "${r.title}" created`)
        })
        .catch(() => toast('Failed to create rule', 'error'))
    }
    setEditTarget(null)
  }

  function handleDelete(id: string) {
    const rule = rules.find(r => r.id === id)
    alertsApi.delete(id)
      .then(() => {
        setRules(prev => prev.filter(r => r.id !== id))
        if (rule) toast(`Rule "${rule.title}" deleted`, 'error')
      })
      .catch(() => toast('Failed to delete rule', 'error'))
  }

  function toggleActive(id: string) {
    const rule = rules.find(r => r.id === id)
    if (!rule) return
    alertsApi.toggle(id, !rule.active)
      .then(() => {
        setRules(prev => prev.map(r => r.id === id ? { ...r, active: !r.active } : r))
        setToggleTarget(null)
        toast(rule.active ? `"${rule.title}" disabled` : `"${rule.title}" enabled`)
      })
      .catch(() => toast('Failed to toggle rule', 'error'))
  }

  function openEdit(r: AlertRule) {
    setEditTarget(r)
    setDrawerOpen(true)
  }

  function closeDrawer() {
    setDrawerOpen(false)
    setTimeout(() => setEditTarget(null), 300)
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <div>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>Alert Rules</h1>
          <p style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>Threshold-based triggers for your sensor data</p>
        </div>
        <div style={{ marginLeft: 'auto' }}>
          <Btn variant="primary" size="sm" onClick={() => { setEditTarget(null); setDrawerOpen(true) }}>+ New Rule</Btn>
        </div>
      </div>

      {/* Stats */}
      <div className="rg4" style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12 }}>
        {[
          { label: 'Total Rules',      value: rules.length,   icon: '📋' },
          { label: 'Active',           value: activeCount,    icon: '✅' },
          { label: 'Critical Active',  value: criticalCount,  icon: '🔴' },
          { label: 'Triggered Today',  value: recentCount,    icon: '🔔' },
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

      {/* Rules list */}
      <Card>
        <CardTitle>All Rules</CardTitle>
        {loading ? <Spinner /> : <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {rules.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--muted)' }}>
              <div style={{ fontSize: 32, marginBottom: 12 }}>🔕</div>
              <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: 4 }}>No alert rules yet</div>
              <div style={{ fontSize: 13, marginBottom: 16 }}>Create a rule to start monitoring your sensor data.</div>
              <Btn variant="primary" size="sm" onClick={() => setDrawerOpen(true)}>+ New Rule</Btn>
            </div>
          ) : rules.map(r => (
            <div
              key={r.id}
              style={{
                background: 'var(--surface2)', borderRadius: 'var(--radius-lg)',
                border: `1px solid ${r.active ? 'var(--border)' : 'var(--border2)'}`,
                padding: '14px 16px',
                opacity: r.active ? 1 : .6,
                transition: 'all .15s',
              }}
              onMouseEnter={e => (e.currentTarget.style.borderColor = 'var(--accent)')}
              onMouseLeave={e => (e.currentTarget.style.borderColor = r.active ? 'var(--border)' : 'var(--border2)')}
            >
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 14 }}>
                <span style={{ fontSize: 22, flexShrink: 0, marginTop: 2 }}>{r.icon}</span>

                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4, flexWrap: 'wrap' }}>
                    <span style={{ fontSize: 14, fontWeight: 700 }}>{r.title}</span>
                    <Badge color={SEVERITY_COLORS[r.severity]}>{SEVERITY_ICONS[r.severity]} {r.severity}</Badge>
                    <Badge color={r.active ? 'green' : 'muted'}>{r.active ? 'Active' : 'Disabled'}</Badge>
                  </div>

                  <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 8 }}>
                    <strong style={{ color: 'var(--text)', fontFamily: 'monospace' }}>{r.device}</strong>
                    {' · '}{r.field} {r.operator} {r.threshold}{r.unit ? ` ${r.unit}` : ''}
                  </div>

                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, alignItems: 'center' }}>
                    {r.notifications.map(n => {
                      const m = NOTIF_META.find(x => x.key === n)!
                      return (
                        <span key={n} style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 11, padding: '2px 8px', borderRadius: 99, background: 'var(--surface)', border: '1px solid var(--border)' }}>
                          {m.icon} {m.label}
                        </span>
                      )
                    })}
                    <span style={{ fontSize: 11, color: 'var(--muted)', marginLeft: 4 }}>Last: <strong style={{ color: 'var(--text)' }}>{r.triggered}</strong></span>
                  </div>
                </div>

                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 8, flexShrink: 0 }}>
                  {/* Enable / Disable toggle */}
                  <button
                    onClick={() => setToggleTarget(r)}
                    style={{
                      position: 'relative', width: 36, height: 20, borderRadius: 99,
                      background: r.active ? 'var(--accent)' : 'var(--border)',
                      border: 'none', cursor: 'pointer', flexShrink: 0, transition: 'background .2s',
                    }}
                  >
                    <span style={{
                      position: 'absolute', top: 3, left: r.active ? 19 : 3,
                      width: 14, height: 14, borderRadius: '50%', background: '#fff',
                      transition: 'left .2s',
                    }} />
                  </button>
                  <div style={{ display: 'flex', gap: 6 }}>
                    <Btn variant="ghost" size="sm" onClick={() => openEdit(r)}>Edit</Btn>
                    <Btn variant="danger" size="sm" onClick={() => setDeleteTarget(r)}>Delete</Btn>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>}
      </Card>

      {/* Drawer & Modal */}
      <AlertRuleDrawer
        key={editTarget?.id ?? 'new'}
        open={drawerOpen}
        rule={editTarget}
        devices={devices}
        onClose={closeDrawer}
        onSave={handleSave}
      />
      <DeleteModal
        rule={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
      />
      <ToggleModal
        rule={toggleTarget}
        onClose={() => setToggleTarget(null)}
        onConfirm={toggleActive}
      />
    </div>
  )
}
