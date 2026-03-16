import { useState } from 'react'
import { rooms, type LobbySetting } from '../../lib/api'
import styles from './RoomSettings.module.css'

interface Props {
  roomId: string
  playerId: string
  isOwner: boolean
  /** Descriptor list from GET /games — defines types, labels, options. */
  descriptors: LobbySetting[]
  /** Current values from RoomView.settings. */
  values: Record<string, string>
  /** Called after a setting is successfully saved so Room.tsx can update its state. */
  onSettingChange: (key: string, value: string) => void
}

export function RoomSettings({
  roomId,
  playerId,
  isOwner,
  descriptors,
  values,
  onSettingChange,
}: Props) {
  const [pending, setPending] = useState<string | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})

  async function handleChange(key: string, value: string) {
    setPending(key)
    setErrors(e => ({ ...e, [key]: '' }))
    try {
      await rooms.updateSetting(roomId, playerId, key, value)
      onSettingChange(key, value)
    } catch (err) {
      setErrors(e => ({
        ...e,
        [key]: err instanceof Error ? err.message : 'Failed to save',
      }))
    } finally {
      setPending(null)
    }
  }

  if (descriptors.length === 0) return null

  return (
    <section className={styles.root} data-testid='lobby-settings'>
      <p className='label'>Game Settings</p>
      <div className={styles.list}>
        {descriptors.map(setting => (
          <SettingRow
            key={setting.key}
            setting={setting}
            value={values[setting.key] ?? setting.default}
            isOwner={isOwner}
            isPending={pending === setting.key}
            error={errors[setting.key]}
            onChange={value => handleChange(setting.key, value)}
          />
        ))}
      </div>
    </section>
  )
}

// --- SettingRow --------------------------------------------------------------

interface RowProps {
  setting: LobbySetting
  value: string
  isOwner: boolean
  isPending: boolean
  error?: string
  onChange: (value: string) => void
}

function SettingRow({ setting, value, isOwner, isPending, error, onChange }: RowProps) {
  return (
    <div className={styles.row} data-testid={`setting-row-${setting.key}`}>
      <div className={styles.labelGroup}>
        <span className={styles.settingLabel}>{setting.label}</span>
        {setting.description && <span className={styles.settingDesc}>{setting.description}</span>}
      </div>

      <div className={styles.control}>
        {isOwner ? (
          <SettingControl
            setting={setting}
            value={value}
            isPending={isPending}
            onChange={onChange}
          />
        ) : (
          <ReadonlyValue setting={setting} value={value} />
        )}
        {error && <span className={styles.error}>{error}</span>}
      </div>
    </div>
  )
}

// --- SettingControl (owner only) ---------------------------------------------

interface ControlProps {
  setting: LobbySetting
  value: string
  isPending: boolean
  onChange: (value: string) => void
}

function SettingControl({ setting, value, isPending, onChange }: ControlProps) {
  if (setting.type === 'select' && setting.options) {
    return (
      <select
        className={styles.select}
        value={value}
        disabled={isPending}
        data-testid={`setting-select-${setting.key}`}
        onChange={e => onChange(e.target.value)}
      >
        {setting.options.map(opt => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
    )
  }

  if (setting.type === 'int') {
    return (
      <input
        type='number'
        className={styles.input}
        value={value}
        disabled={isPending}
        min={setting.min}
        max={setting.max}
        data-testid={`setting-input-${setting.key}`}
        onChange={e => onChange(e.target.value)}
        onBlur={e => onChange(e.target.value)}
      />
    )
  }

  return null
}

// --- ReadonlyValue (non-owners) ----------------------------------------------

function ReadonlyValue({ setting, value }: { setting: LobbySetting; value: string }) {
  // For select settings, show the human-readable label instead of the raw value.
  if (setting.type === 'select' && setting.options) {
    const opt = setting.options.find(o => o.value === value)
    return (
      <span className={styles.readonlyValue} data-testid={`setting-value-${setting.key}`}>
        {opt?.label ?? value}
      </span>
    )
  }

  return (
    <span className={styles.readonlyValue} data-testid={`setting-value-${setting.key}`}>
      {value}
    </span>
  )
}
