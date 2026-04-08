import { useState, useRef, useMemo } from 'react'
import { Btn } from './Button'
import { Dot } from './Badge'

// ── Types ──────────────────────────────────────────────────────────────────

export interface Field {
  name: string
  unit: string
  type: 'float' | 'integer' | 'string' | 'boolean'
  key: string
  keyLocked?: boolean
}

export interface NewDevice {
  icon: string
  name: string
  id: string
  workspace: string
  description: string
  tags: string
  lat: string
  lng: string
  locationLabel: string
  channelName: string
  channelId: string
  visibility: 'private' | 'public'
  fields: Field[]
  apiKey: string
}

interface WorkspaceOption {
  id: string
  name: string
}

interface Props {
  open: boolean
  onClose: () => void
  onRegister: (device: NewDevice) => Promise<{ channelId: string; apiKey: string }>
  workspaces?: WorkspaceOption[]
}

// ── Constants ──────────────────────────────────────────────────────────────

const DEVICE_TYPES = [
  {
    icon: '🌡️', name: 'Environmental', desc: 'Temp, humidity, CO₂, light',
    defaultFields: [
      { name: 'Temperature', unit: '°C',  type: 'float' as const, key: 'temperature' },
      { name: 'Humidity',    unit: '%',   type: 'float' as const, key: 'humidity' },
      { name: 'CO₂',        unit: 'ppm', type: 'float' as const, key: 'co2' },
    ],
  },
  {
    icon: '💧', name: 'Water Quality', desc: 'pH, turbidity, dissolved O₂',
    defaultFields: [
      { name: 'pH',           unit: '',      type: 'float' as const, key: 'ph' },
      { name: 'Turbidity',    unit: 'NTU',   type: 'float' as const, key: 'turbidity' },
      { name: 'Dissolved O₂', unit: 'mg/L',  type: 'float' as const, key: 'dissolved_o2' },
    ],
  },
  {
    icon: '💨', name: 'Air Quality', desc: 'PM2.5, PM10, VOC, AQI',
    defaultFields: [
      { name: 'PM2.5', unit: 'μg/m³', type: 'float'   as const, key: 'pm2_5' },
      { name: 'PM10',  unit: 'μg/m³', type: 'float'   as const, key: 'pm10' },
      { name: 'AQI',   unit: '',      type: 'integer' as const, key: 'aqi' },
    ],
  },
  {
    icon: '⚡', name: 'Energy', desc: 'Power, voltage, current',
    defaultFields: [
      { name: 'Voltage', unit: 'V', type: 'float' as const, key: 'voltage' },
      { name: 'Current', unit: 'A', type: 'float' as const, key: 'current' },
      { name: 'Power',   unit: 'W', type: 'float' as const, key: 'power' },
    ],
  },
  {
    icon: '🌾', name: 'Soil / Agriculture', desc: 'Moisture, NPK, pH',
    defaultFields: [
      { name: 'Moisture',  unit: '%',    type: 'float' as const, key: 'moisture' },
      { name: 'pH',        unit: '',     type: 'float' as const, key: 'ph' },
      { name: 'Nitrogen',  unit: 'mg/L', type: 'float' as const, key: 'nitrogen' },
    ],
  },
  {
    icon: '📦', name: 'Custom', desc: 'Define your own fields',
    defaultFields: [
      { name: 'field1', unit: '', type: 'float' as const, key: 'field1' },
    ],
  },
]


// ── Helpers ────────────────────────────────────────────────────────────────

function genId() {
  return 'dev_' + Math.random().toString(36).slice(2, 10)
}

function genApiKey() {
  const chars = '0123456789abcdef'
  return 'ts_' + Array.from({ length: 64 }, () => chars[Math.floor(Math.random() * 16)]).join('')
}

// Normalise a human label into a safe slug used as the JSON key
const SCRIPT_MAP: Record<string, string> = {
  '₀':'0','₁':'1','₂':'2','₃':'3','₄':'4','₅':'5','₆':'6','₇':'7','₈':'8','₉':'9',
  '⁰':'0','¹':'1','²':'2','³':'3','⁴':'4','⁵':'5','⁶':'6','⁷':'7','⁸':'8','⁹':'9',
  'μ':'u','µ':'u','°':'','Ω':'ohm','±':'',
}

export function toFieldKey(label: string): string {
  return label
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[₀₁₂₃₄₅₆₇₈₉⁰¹²³⁴⁵⁶⁷⁸⁹μµ°Ω±]/g, c => SCRIPT_MAP[c] ?? '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 32) || 'field'
}

// ── Shared styles ──────────────────────────────────────────────────────────

const label: React.CSSProperties = {
  display: 'block', fontSize: 12, fontWeight: 600,
  color: 'var(--muted)', marginBottom: 6,
  textTransform: 'uppercase', letterSpacing: '.05em',
}

const inp: React.CSSProperties = {
  width: '100%', background: 'var(--surface2)',
  border: '1px solid var(--border)', borderRadius: 'var(--radius)',
  padding: '8px 12px', color: 'var(--text)', fontSize: 13, outline: 'none',
  boxSizing: 'border-box',
}

// ── Stepper ────────────────────────────────────────────────────────────────

function Stepper({ step }: { step: number }) {
  const steps = ['Details', 'Channels', 'API Key']
  return (
    <div style={{
      display: 'flex', alignItems: 'center',
      padding: '14px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0,
    }}>
      {steps.map((lbl, i) => {
        const n = i + 1
        const done   = n < step
        const active = n === step
        return (
          <div key={lbl} style={{ display: 'flex', alignItems: 'center', flex: i < steps.length - 1 ? 1 : undefined }}>
            <div style={{
              display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, fontWeight: 600,
              color: done ? 'var(--green)' : active ? 'var(--text)' : 'var(--muted)',
            }}>
              <div style={{
                width: 22, height: 22, borderRadius: '50%',
                border: `2px solid ${done ? 'var(--green)' : active ? 'var(--accent)' : 'var(--border)'}`,
                background: done ? 'var(--green)' : active ? 'var(--accent)' : 'transparent',
                display: 'grid', placeItems: 'center',
                fontSize: 11, fontWeight: 700, flexShrink: 0,
                color: (done || active) ? '#fff' : 'var(--muted)',
                transition: 'all 180ms ease',
              }}>
                {done ? '✓' : n}
              </div>
              {lbl}
            </div>
            {i < steps.length - 1 && (
              <div style={{
                flex: 1, height: 1, margin: '0 8px',
                background: done ? 'var(--green)' : 'var(--border)',
                transition: 'background 180ms ease',
              }} />
            )}
          </div>
        )
      })}
    </div>
  )
}

// ── Step 1: Device Details ─────────────────────────────────────────────────

function Step1({
  name, setName, typeIdx, setTypeIdx,
  workspace, setWorkspace, workspaceOptions, description, setDescription, tags, setTags,
  lat, setLat, lng, setLng, locationLabel, setLocationLabel,
}: {
  name: string; setName(v: string): void
  typeIdx: number; setTypeIdx(i: number): void
  workspace: string; setWorkspace(v: string): void
  workspaceOptions: WorkspaceOption[]
  description: string; setDescription(v: string): void
  tags: string; setTags(v: string): void
  lat: string; setLat(v: string): void
  lng: string; setLng(v: string): void
  locationLabel: string; setLocationLabel(v: string): void
}) {
  return (
    <div>
      <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 20 }}>
        Basic information about your device. The name and type help you identify it on the dashboard.
      </p>

      {/* Name */}
      <div style={{ marginBottom: 16 }}>
        <label style={label}>Device Name <span style={{ color: 'var(--red)' }}>*</span></label>
        <input
          value={name} onChange={e => setName(e.target.value)}
          placeholder="e.g. Greenhouse Sensor A"
          style={inp}
          autoFocus
        />
      </div>

      {/* Type grid */}
      <div style={{ marginBottom: 16 }}>
        <label style={label}>Device Type</label>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
          {DEVICE_TYPES.map((t, i) => (
            <div
              key={t.name}
              onClick={() => setTypeIdx(i)}
              style={{
                border: `2px solid ${i === typeIdx ? 'var(--accent)' : 'var(--border)'}`,
                background: i === typeIdx ? 'rgba(37,99,235,.1)' : 'transparent',
                borderRadius: 'var(--radius-lg)', padding: '14px 12px',
                cursor: 'pointer', textAlign: 'center',
                transition: 'all 180ms ease',
              }}
            >
              <div style={{ fontSize: 24, marginBottom: 5 }}>{t.icon}</div>
              <div style={{ fontSize: 12, fontWeight: 600 }}>{t.name}</div>
              <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 2 }}>{t.desc}</div>
            </div>
          ))}
        </div>
      </div>

      {/* Workspace */}
      <div style={{ marginBottom: 16 }}>
        <label style={{ ...label, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span>Workspace</span>
          <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--accent-lt)', cursor: 'pointer', textTransform: 'none', letterSpacing: 0 }}>
            + New workspace
          </span>
        </label>
        <select value={workspace} onChange={e => setWorkspace(e.target.value)} style={{ ...inp, appearance: 'none' }}>
          {workspaceOptions.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}
        </select>
      </div>

      {/* Description */}
      <div style={{ marginBottom: 16 }}>
        <label style={label}>Description</label>
        <input value={description} onChange={e => setDescription(e.target.value)} placeholder="Optional — what this device monitors" style={inp} />
      </div>

      {/* Tags */}
      <div style={{ marginBottom: 16 }}>
        <label style={label}>Tags</label>
        <input value={tags} onChange={e => setTags(e.target.value)} placeholder="agriculture, zone-a, outdoor  (comma-separated)" style={inp} />
        <div style={{ fontSize: 11, color: 'var(--muted)', marginTop: 5 }}>
          Tags help you filter and group devices on the dashboard.
        </div>
      </div>

      {/* Location */}
      <div style={{ marginBottom: 4 }}>
        <label style={label}>Location <span style={{ fontWeight: 400, textTransform: 'none', letterSpacing: 0, color: 'var(--muted)' }}>(optional)</span></label>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 8 }}>
          <div>
            <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 4 }}>Latitude</div>
            <input
              value={lat} onChange={e => setLat(e.target.value)}
              placeholder="e.g. 10.7769"
              type="number"
              step="any"
              style={inp}
            />
          </div>
          <div>
            <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 4 }}>Longitude</div>
            <input
              value={lng} onChange={e => setLng(e.target.value)}
              placeholder="e.g. 106.7009"
              type="number"
              step="any"
              style={inp}
            />
          </div>
        </div>
        <input
          value={locationLabel} onChange={e => setLocationLabel(e.target.value)}
          placeholder="Address or location name (e.g. Greenhouse A, Building 3)"
          style={inp}
        />
      </div>
    </div>
  )
}

// ── Step 2: Channels & Fields ──────────────────────────────────────────────

function Step2({
  channelName, setChannelName,
  visibility, setVisibility,
  fields, setFields,
}: {
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
        Define the channel this device will publish to and its measurement fields (up to 8).
      </p>

      {/* Channel name */}
      <div style={{ marginBottom: 16 }}>
        <label style={label}>Channel Name <span style={{ color: 'var(--red)' }}>*</span></label>
        <input
          value={channelName}
          onChange={e => setChannelName(e.target.value)}
          placeholder="e.g. Greenhouse Climate"
          style={inp}
          autoFocus
        />
      </div>

      {/* Visibility */}
      <div style={{ marginBottom: 20 }}>
        <label style={label}>Visibility</label>
        <div style={{ display: 'flex', gap: 10 }}>
          {(['private', 'public'] as const).map(v => (
            <div
              key={v}
              onClick={() => setVisibility(v)}
              style={{
                flex: 1, border: `2px solid ${visibility === v ? 'var(--accent)' : 'var(--border)'}`,
                background: visibility === v ? 'rgba(37,99,235,.1)' : 'transparent',
                borderRadius: 'var(--radius)', padding: '10px 14px',
                cursor: 'pointer', transition: 'all 180ms ease',
              }}
            >
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 2 }}>
                {v === 'private' ? '🔒 Private' : '🌐 Public'}
              </div>
              <div style={{ fontSize: 11, color: 'var(--muted)' }}>
                {v === 'private' ? 'Only your workspace can read' : 'Anyone with the channel ID'}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Field builder */}
      <div style={{ marginBottom: 16 }}>
        <label style={label}>
          Fields{' '}
          <span style={{ color: 'var(--muted)', fontWeight: 400, textTransform: 'none', letterSpacing: 0 }}>
            ({fields.length}/8)
          </span>
        </label>

        {/* Column header */}
        <div style={{
          display: 'grid', gridTemplateColumns: '28px 1fr 72px 100px 32px',
          gap: 6, padding: '0 4px', marginBottom: 4,
        }}>
          {['', 'Field name', 'Unit', 'Type', ''].map((h, i) => (
            <span key={i} style={{ fontSize: 10, fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em' }}>
              {h}
            </span>
          ))}
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {(() => {
            const keyCounts = fields.reduce<Record<string, number>>((acc, f) => {
              if (f.key) acc[f.key] = (acc[f.key] ?? 0) + 1
              return acc
            }, {})
            const dupKeys = new Set(Object.entries(keyCounts).filter(([, n]) => n > 1).map(([k]) => k))

            return fields.map((f, i) => {
              const isDup = f.key ? dupKeys.has(f.key) : false
              return (
                <div key={i} style={{
                  background: 'var(--surface2)',
                  border: `1px solid ${isDup ? 'var(--red)' : 'var(--border)'}`,
                  borderRadius: 'var(--radius)', padding: '6px 10px',
                  transition: 'border-color 180ms ease',
                }}>
                  {/* Top row */}
                  <div style={{ display: 'grid', gridTemplateColumns: '28px 1fr 72px 100px 32px', alignItems: 'center', gap: 6 }}>
                    <span style={{ fontSize: 11, fontWeight: 700, color: 'var(--accent-lt)', textAlign: 'center' }}>F{i + 1}</span>
                    <input
                      value={f.name}
                      onChange={e => updateField(i, 'name', e.target.value)}
                      placeholder="Field name"
                      style={{ ...inp, padding: '5px 8px', fontSize: 12 }}
                    />
                    <input
                      value={f.unit}
                      onChange={e => updateField(i, 'unit', e.target.value)}
                      placeholder="Unit"
                      style={{ ...inp, padding: '5px 8px', fontSize: 12 }}
                    />
                    <select
                      value={f.type}
                      onChange={e => updateField(i, 'type', e.target.value)}
                      style={{ ...inp, padding: '5px 6px', fontSize: 12, appearance: 'none' }}
                    >
                      <option value="float">float</option>
                      <option value="integer">integer</option>
                      <option value="string">string</option>
                      <option value="boolean">boolean</option>
                    </select>
                    <button
                      onClick={() => removeField(i)}
                      disabled={fields.length <= 1}
                      style={{
                        width: 28, height: 28, borderRadius: 'var(--radius)',
                        display: 'grid', placeItems: 'center', fontSize: 13,
                        color: 'var(--muted)', background: 'transparent',
                        border: '1px solid var(--border)',
                        cursor: fields.length <= 1 ? 'default' : 'pointer',
                        opacity: fields.length <= 1 ? .3 : 1,
                        transition: 'all 180ms ease',
                      }}
                    >✕</button>
                  </div>

                  {/* Key row */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 5, paddingLeft: 34 }}>
                    <span style={{ fontSize: 10, fontWeight: 700, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', flexShrink: 0 }}>key</span>
                    <input
                      value={f.key}
                      onChange={e => updateField(i, 'key', e.target.value)}
                      placeholder="auto"
                      style={{
                        ...inp, padding: '3px 7px', fontSize: 11,
                        fontFamily: 'monospace', color: isDup ? 'var(--red)' : 'var(--cyan)',
                        flex: 1,
                      }}
                    />
                    {f.keyLocked && (
                      <span
                        title="Key manually set — won't auto-update from label"
                        style={{ fontSize: 12, cursor: 'default', flexShrink: 0 }}
                      >🔒</span>
                    )}
                  </div>
                  {isDup && (
                    <div style={{ fontSize: 10, color: 'var(--red)', marginTop: 3, paddingLeft: 34, fontWeight: 600 }}>
                      Duplicate key — must be unique
                    </div>
                  )}
                </div>
              )
            })
          })()}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 10 }}>
          <Btn variant="ghost" size="sm" onClick={addField} disabled={fields.length >= 8}>
            + Add Field
          </Btn>
          {fields.length >= 8 && (
            <span style={{ fontSize: 11, color: 'var(--muted)' }}>Maximum 8 fields reached</span>
          )}
        </div>
      </div>

      <div style={{
        background: 'rgba(37,99,235,.08)', border: '1px solid rgba(37,99,235,.2)',
        borderRadius: 'var(--radius)', padding: '12px 14px',
        fontSize: 12, color: 'var(--accent-lt)',
      }}>
        💡 You can add more channels and modify fields at any time from the Channels page.
      </div>
    </div>
  )
}

// ── Step 3: API Key ────────────────────────────────────────────────────────

type SnippetFormat = 'json' | 'ojson' | 'msgpack' | 'binary'

const FORMAT_TABS: { id: SnippetFormat; label: string }[] = [
  { id: 'json',    label: 'JSON'    },
  { id: 'ojson',   label: 'OJson'   },
  { id: 'msgpack', label: 'MsgPack' },
  { id: 'binary',  label: 'Binary'  },
]

function Step3({ device }: { device: NewDevice }) {
  const [copiedKey,  setCopiedKey]  = useState(false)
  const [copiedSnip, setCopiedSnip] = useState(false)
  const [format, setFormat] = useState<SnippetFormat>('json')

  const exampleValue = (f: Field) => {
    if (f.type === 'boolean') return 'true'
    if (f.type === 'string')  return '"normal"'
    if (f.type === 'integer') return '42'
    // Float: pick a realistic seed based on common field names
    const k = (f.key || f.name).toLowerCase()
    if (k.includes('temp'))    return '23.5'
    if (k.includes('humid'))   return '61.2'
    if (k.includes('ph'))      return '7.2'
    if (k.includes('co2'))     return '412.0'
    if (k.includes('pm'))      return '18.4'
    if (k.includes('aqi'))     return '52.0'
    if (k.includes('volt'))    return '12.1'
    if (k.includes('curr'))    return '1.35'
    if (k.includes('power'))   return '16.3'
    if (k.includes('moisture') || k.includes('soil')) return '42.8'
    if (k.includes('o2') || k.includes('oxygen'))     return '8.2'
    if (k.includes('turb'))    return '1.4'
    if (k.includes('nitro'))   return '24.0'
    return '0.0'
  }

  const exampleValueNum = (f: Field): number => parseFloat(exampleValue(f)) || 0

  const fieldLines = device.fields
    .map(f => `      "${f.key || f.name}": ${exampleValue(f)}`)
    .join(',\n')

  const baseUrl = import.meta.env.VITE_API_URL ?? 'http://localhost:9080'
  const url = `${baseUrl}/v1/channels/${device.channelId}/data`

  const { nowIso, nowMs } = useMemo(() => {
    const now = new Date()
    return {
      nowIso: now.toISOString().replace(/\.\d{3}Z$/, 'Z'),
      nowMs:  now.getTime(),
    }
  }, [])

  const fieldTimestampLines = device.fields
    .map((f, i) => {
      const ts = new Date(nowMs - i * 5000).toISOString().replace(/\.\d{3}Z$/, 'Z')
      return `      "${f.key || f.name}": "${ts}"`
    })
    .join(',\n')

  const snippets: Record<SnippetFormat, string> = {
    json:
`# Single timestamp for all fields
curl -X POST ${url} \\
  -H "X-API-Key: ${device.apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "fields": {
${fieldLines}
    },
    "timestamp": "${nowIso}"
  }'

# Per-field timestamps (each sensor captured at a different time)
curl -X POST ${url} \\
  -H "X-API-Key: ${device.apiKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "fields": {
${fieldLines}
    },
    "field_timestamps": {
${fieldTimestampLines}
    }
  }'`,

    ojson:
`# Compact ordered JSON — fields are positional (f[0] = field 1, f[1] = field 2, ...)
# t = base timestamp (unix ms), td = per-field offset in seconds from t

# Single timestamp for all fields
curl -X POST ${url} \\
  -H "X-API-Key: ${device.apiKey}" \\
  -H "Content-Type: application/x-greenlab-ojson" \\
  -d '{"f":[${device.fields.map(f => exampleValue(f)).join(',')}],"t":${nowMs},"sv":1}'

# Per-field timestamps via td offsets (td[i] = seconds after t for field i)
curl -X POST ${url} \\
  -H "X-API-Key: ${device.apiKey}" \\
  -H "Content-Type: application/x-greenlab-ojson" \\
  -d '{"f":[${device.fields.map(f => exampleValue(f)).join(',')}],"t":${nowMs},"td":[${device.fields.map((_, i) => i * 5).join(',')}],"sv":1}'`,

    msgpack:
`# MsgPack — same structure as OJson, encoded as binary (use a library)
# Python example:
import msgpack, requests

# Single timestamp for all fields
payload = msgpack.packb({
    "f": [${device.fields.map(f => exampleValueNum(f)).join(', ')}],
    "t": ${nowMs},
    "sv": 1,
})

# Per-field timestamps via td offsets (td[i] = seconds after t for field i)
payload = msgpack.packb({
    "f": [${device.fields.map(f => exampleValueNum(f)).join(', ')}],
    "t": ${nowMs},
    "td": [${device.fields.map((_, i) => i * 5).join(', ')}],
    "sv": 1,
})

requests.post(
    "${url}",
    data=payload,
    headers={
        "X-API-Key": "${device.apiKey}",
        "Content-Type": "application/msgpack",
    },
)`,

    binary:
`# Binary frame — ultra-compact for microcontrollers (12 + N×2 bytes)
# Frame layout: VER(1) | DEVID(4) | TS(4) | FIELDMSK(1) | VALUES(N×2) | CRC16(2)
#
# VER       = 0x01
# DEVID     = first 4 bytes of your device UUID
# TS        = unix timestamp (uint32 big-endian)
# FIELDMSK  = bitmask of fields present (bit 0 = field 1, bit 1 = field 2, ...)
# VALUES    = uint16 per field, big-endian
# CRC16     = CRC16/CCITT-FALSE over all preceding bytes
#
# Device ID: ${device.id}
# Fields:    ${device.fields.map((f, i) => `bit ${i} → ${f.key || f.name}`).join(', ')}
#
# Use the GreenLab embedded SDK or implement the frame encoder in your firmware.`,
  }

  const tips: Record<SnippetFormat, string> = {
    json:    'Use timestamp for a single shared time across all fields. Use field_timestamps to assign a different capture time per field — omit both and the server uses receive time.',
    ojson:   'Up to ~70% smaller than JSON. Use td (time-delta array) for per-field timestamps — each value is seconds after t. Fields must match the order defined in your channel schema.',
    msgpack: 'MsgPack encodes the same OJson structure as binary — great for Python, Node.js, or Go clients. Supports td for per-field timestamps, same as OJson. Typically 40–60% smaller than JSON.',
    binary:  'Smallest possible payload (as low as 14 bytes for 1 field). Designed for constrained microcontrollers (Arduino, ESP32, STM32). Requires firmware-side CRC16 computation.',
  }

  const responsePreview =
`{
  "success": true,
  "data": {
    "accepted": 1,
    "written_at": "..."
  }
}`

  function copy(text: string, set: (v: boolean) => void) {
    navigator.clipboard.writeText(text).catch(() => {})
    set(true)
    setTimeout(() => set(false), 2000)
  }

  return (
    <div>
      {/* Success header */}
      <div style={{ textAlign: 'center', padding: '16px 0 24px' }}>
        <div style={{ fontSize: 52, marginBottom: 12 }}>✅</div>
        <div style={{ fontSize: 18, fontWeight: 700, marginBottom: 6 }}>
          {device.icon} {device.name} registered!
        </div>
        <div style={{ fontSize: 13, color: 'var(--muted)' }}>
          Your device is ready to send telemetry data.
        </div>
      </div>

      {/* API Key card */}
      <div style={{
        background: 'var(--surface2)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius-lg)', padding: 20, marginBottom: 16,
      }}>
        <div style={{
          fontSize: 11, fontWeight: 700, textTransform: 'uppercase',
          letterSpacing: '.08em', color: 'var(--muted)', marginBottom: 10,
        }}>
          Device API Key
        </div>
        <div style={{
          fontFamily: 'monospace', fontSize: 11, color: 'var(--green)',
          wordBreak: 'break-all', background: 'var(--bg)',
          border: '1px solid var(--border)', borderRadius: 'var(--radius)',
          padding: '12px 14px', marginBottom: 12,
          letterSpacing: '.04em', lineHeight: 1.8,
        }}>
          {device.apiKey}
        </div>
        <Btn
          variant="ghost" size="sm"
          style={{ width: '100%', justifyContent: 'center' }}
          onClick={() => copy(device.apiKey, setCopiedKey)}
        >
          {copiedKey ? '✓ Copied!' : '⎘ Copy API Key'}
        </Btn>
      </div>

      {/* One-time warning */}
      <div style={{
        display: 'flex', alignItems: 'flex-start', gap: 10,
        background: 'rgba(245,158,11,.1)', border: '1px solid rgba(245,158,11,.3)',
        borderRadius: 'var(--radius)', padding: '12px 14px',
        fontSize: 12, color: 'var(--yellow)', marginBottom: 20,
      }}>
        <span style={{ fontSize: 16, flexShrink: 0, marginTop: 1 }}>⚠️</span>
        <span>
          This key is shown <strong>only once</strong>. Store it securely — you cannot retrieve it again.
          If lost, rotate it from the Devices page.
        </span>
      </div>

      {/* Quick Start */}
      <div style={{
        fontSize: 11, fontWeight: 700, textTransform: 'uppercase',
        letterSpacing: '.06em', color: 'var(--muted)', marginBottom: 10,
      }}>
        Quick Start — Send a Reading
      </div>

      {/* Format tabs */}
      <div style={{
        display: 'flex', gap: 4, marginBottom: 8,
        background: 'var(--surface2)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius)', padding: 4,
      }}>
        {FORMAT_TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => { setFormat(tab.id); setCopiedSnip(false) }}
            style={{
              flex: 1, padding: '5px 0', fontSize: 11, fontWeight: 600,
              border: 'none', borderRadius: 'calc(var(--radius) - 2px)',
              cursor: 'pointer', transition: 'background .15s, color .15s',
              background: format === tab.id ? 'var(--accent)' : 'transparent',
              color: format === tab.id ? '#fff' : 'var(--muted)',
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Snippet */}
      <pre style={{
        background: 'var(--bg)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius)', padding: '14px 16px',
        fontFamily: 'monospace', fontSize: 11, color: 'var(--cyan)',
        whiteSpace: 'pre', overflowX: 'auto', marginBottom: 8, lineHeight: 1.7,
      }}>{snippets[format]}</pre>
      <Btn variant="ghost" size="sm" onClick={() => copy(snippets[format], setCopiedSnip)}>
        {copiedSnip ? '✓ Copied!' : '⎘ Copy snippet'}
      </Btn>

      {/* Response preview */}
      <div style={{
        fontSize: 11, fontWeight: 700, textTransform: 'uppercase',
        letterSpacing: '.06em', color: 'var(--muted)', margin: '16px 0 8px',
      }}>
        Expected Response
      </div>
      <pre style={{
        background: 'var(--bg)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius)', padding: '14px 16px',
        fontFamily: 'monospace', fontSize: 11, color: 'var(--green)',
        whiteSpace: 'pre', overflowX: 'auto', marginBottom: 8, lineHeight: 1.7,
      }}>{responsePreview}</pre>

      {/* Tip */}
      <div style={{
        display: 'flex', gap: 8, alignItems: 'flex-start',
        background: 'rgba(37,99,235,.08)', border: '1px solid rgba(37,99,235,.2)',
        borderRadius: 'var(--radius)', padding: '10px 14px',
        fontSize: 12, color: 'var(--accent-lt)', marginBottom: 24,
      }}>
        <span style={{ flexShrink: 0 }}>💡</span>
        <span>{tips[format]}</span>
      </div>

      {/* Summary */}
      <div style={{
        fontSize: 11, fontWeight: 700, textTransform: 'uppercase',
        letterSpacing: '.06em', color: 'var(--muted)', marginBottom: 10,
      }}>
        Device Summary
      </div>
      <div style={{
        background: 'var(--surface2)', border: '1px solid var(--border)',
        borderRadius: 'var(--radius-lg)', overflow: 'hidden',
      }}>
        {[
          { label: 'Name',       value: <span style={{ fontWeight: 600 }}>{device.icon} {device.name}</span> },
          { label: 'Device ID',  value: <span style={{ fontFamily: 'monospace', fontSize: 12, color: 'var(--cyan)' }}>{device.id}</span> },
          { label: 'Workspace',  value: <span style={{ color: 'var(--muted)' }}>{device.workspace}</span> },
          { label: 'Channel',    value: device.channelName },
          { label: 'Visibility', value: device.visibility === 'public' ? '🌐 Public' : '🔒 Private' },
          { label: 'Fields',     value: device.fields.map(f => `${f.name}${f.unit ? ` (${f.unit})` : ''}`).join(', ') },
          {
            label: 'Status',
            value: (
              <span style={{
                display: 'inline-flex', alignItems: 'center', gap: 4,
                background: 'rgba(34,197,94,.15)', color: 'var(--green)',
                fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 99,
              }}>
                <Dot color="green" /> Active
              </span>
            ),
          },
        ].map((row, i, arr) => (
          <div key={row.label} style={{
            display: 'flex', gap: 12, alignItems: 'flex-start',
            padding: '10px 16px',
            borderBottom: i < arr.length - 1 ? '1px solid var(--border2)' : 'none',
          }}>
            <span style={{ fontSize: 12, color: 'var(--muted)', width: 90, flexShrink: 0, paddingTop: 1 }}>{row.label}</span>
            <span style={{ fontSize: 13, flex: 1, wordBreak: 'break-all' }}>{row.value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Main Drawer ────────────────────────────────────────────────────────────

export function RegisterDeviceDrawer({ open, onClose, onRegister, workspaces: workspaceProp = [] }: Props) {
  const [step, setStep] = useState(1)

  // Step 1 state
  const [name,          setName]          = useState('')
  const [typeIdx,       setTypeIdx]       = useState(0)
  const [workspace,     setWorkspace]     = useState(workspaceProp[0]?.id ?? '')
  const [description,   setDescription]   = useState('')
  const [tags,          setTags]          = useState('')
  const [lat,           setLat]           = useState('')
  const [lng,           setLng]           = useState('')
  const [locationLabel, setLocationLabel] = useState('')

  // Step 2 state
  const [channelName, setChannelName] = useState('')
  const [visibility,  setVisibility]  = useState<'private' | 'public'>('private')
  const [fields,      setFields]      = useState<Field[]>(DEVICE_TYPES[0].defaultFields)
  const [submitting,  setSubmitting]  = useState(false)

  // Step 3 — stored in a ref so it's immediately available on render
  const deviceRef = useRef<NewDevice | null>(null)

  function handleTypeChange(i: number) {
    setTypeIdx(i)
    setFields(DEVICE_TYPES[i].defaultFields)
  }

  const step1Valid = name.trim().length > 0
  const step2Valid = channelName.trim().length > 0

  async function advance() {
    if (step === 1) {
      if (!step1Valid) return
      // Seed channel name from device name + type when entering step 2
      if (!channelName) {
        setChannelName(`${name.trim()} — ${DEVICE_TYPES[typeIdx].name}`)
      }
      setStep(2)
      return
    }

    if (step === 2) {
      if (!step2Valid) return
      const device: NewDevice = {
        icon:          DEVICE_TYPES[typeIdx].icon,
        name:          name.trim(),
        id:            genId(),
        workspace,
        description,
        tags,
        lat,
        lng,
        locationLabel,
        channelName:   channelName.trim(),
        channelId:     '',
        visibility,
        fields,
        apiKey:        genApiKey(),
      }
      setSubmitting(true)
      try {
        const result = await onRegister(device)
        deviceRef.current = { ...device, channelId: result.channelId, apiKey: result.apiKey }
      } finally {
        setSubmitting(false)
      }
      setStep(3)
      return
    }

    // Step 3 → Done
    handleClose()
  }

  function back() { setStep(s => s - 1) }

  function handleClose() {
    onClose()
    setTimeout(() => {
      setStep(1); setName(''); setTypeIdx(0)
      setWorkspace(workspaceProp[0]?.id ?? ''); setDescription(''); setTags('')
      setLat(''); setLng(''); setLocationLabel('')
      setChannelName(''); setVisibility('private')
      setFields(DEVICE_TYPES[0].defaultFields)
      setSubmitting(false)
      deviceRef.current = null
    }, 300)
  }

  const nextLabel  = step === 2 ? (submitting ? 'Registering…' : 'Register Device →') : step === 3 ? '✓ Done' : 'Next →'
  const nextValid  = step === 1 ? step1Valid : step === 2 ? (step2Valid && !submitting) : true

  return (
    <>
      {open && (
        <div
          onClick={handleClose}
          style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,.55)', backdropFilter: 'blur(3px)', zIndex: 200 }}
        />
      )}

      <div style={{
        position: 'fixed', top: 0, right: 0, bottom: 0,
        width: 480,
        background: 'var(--surface)',
        borderLeft: '1px solid var(--border)',
        display: 'flex', flexDirection: 'column',
        transform: open ? 'translateX(0)' : 'translateX(100%)',
        transition: 'transform 300ms cubic-bezier(.4,0,.2,1)',
        zIndex: 201, overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', gap: 12,
          padding: '18px 20px', borderBottom: '1px solid var(--border)', flexShrink: 0,
        }}>
          <span style={{ fontSize: 20 }}>🔌</span>
          <h2 style={{ fontSize: 16, fontWeight: 700, flex: 1 }}>Register New Device</h2>
          <button
            onClick={handleClose}
            style={{
              width: 30, height: 30, borderRadius: 'var(--radius)',
              display: 'grid', placeItems: 'center',
              fontSize: 18, color: 'var(--muted)', cursor: 'pointer',
              background: 'transparent', border: 'none',
            }}
          >✕</button>
        </div>

        <Stepper step={step} />

        {/* Body */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '24px 20px' }}>
          {step === 1 && (
            <Step1
              name={name} setName={setName}
              typeIdx={typeIdx} setTypeIdx={handleTypeChange}
              workspace={workspace} setWorkspace={setWorkspace}
              workspaceOptions={workspaceProp}
              description={description} setDescription={setDescription}
              tags={tags} setTags={setTags}
              lat={lat} setLat={setLat}
              lng={lng} setLng={setLng}
              locationLabel={locationLabel} setLocationLabel={setLocationLabel}
            />
          )}
          {step === 2 && (
            <Step2
              channelName={channelName} setChannelName={setChannelName}
              visibility={visibility} setVisibility={setVisibility}
              fields={fields} setFields={setFields}
            />
          )}
          {step === 3 && deviceRef.current && (
            <Step3 device={deviceRef.current} />
          )}
        </div>

        {/* Footer */}
        <div style={{
          display: 'flex', alignItems: 'center', gap: 10,
          padding: '16px 20px', borderTop: '1px solid var(--border)', flexShrink: 0,
        }}>
          {step > 1 && step < 3 && (
            <Btn variant="ghost" onClick={back}>← Back</Btn>
          )}
          <div style={{ flex: 1 }} />
          {step < 3 && <Btn variant="ghost" onClick={handleClose}>Cancel</Btn>}
          <Btn
            variant="primary"
            onClick={advance}
            disabled={!nextValid}
            style={{ opacity: nextValid ? 1 : .5 }}
          >
            {nextLabel}
          </Btn>
        </div>
      </div>
    </>
  )
}
