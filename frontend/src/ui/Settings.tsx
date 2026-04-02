import { useCallback } from 'react'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useAppStore } from '@/stores/store'
import { playerSettings, DEFAULT_SETTINGS, type PlayerSettingMap, Language } from '@/lib/api'
import { catchToAppError } from '@/utils/errors'
import { useToast } from './Toast'
import styles from './Settings.module.css'
import { SKINS } from '@/lib/skins'

// ---------------------------------------------------------------------------
// Settings cache helpers (mirrors App.tsx)
// ---------------------------------------------------------------------------

const SETTINGS_CACHE_KEY = 'tf:settings'

function saveSettingsToCache(settings: Required<PlayerSettingMap>) {
  try {
    localStorage.setItem(SETTINGS_CACHE_KEY, JSON.stringify(settings))
  } catch {
    // localStorage unavailable — fail silently.
  }
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

interface Props {
  onClose?: () => void
}

export function Settings({ onClose }: Props) {
  const player = useAppStore(s => s.player)!
  const settings = useAppStore(s => s.settings)
  const updateSetting = useAppStore(s => s.updateSetting)
  const toast = useToast()

  // Debounced backend sync — fires 600ms after the last change.
  // Each call sends the full current settings object so rapid changes
  // don't cause partial writes.
  const syncToBackend = useDebouncedCallback(
    async (patch: PlayerSettingMap) => {
      const [err] = await playerSettings
        .update(player.id, patch)
        .then(r => [null, r] as const)
        .catch(e => [catchToAppError(e), null] as const)

      if (err) {
        toast.showError(err)
      }
    },
    { wait: 600 },
  )

  // Optimistic update + cache write + debounced backend sync.
  const change = useCallback(
    <K extends keyof PlayerSettingMap>(key: K, value: PlayerSettingMap[K]) => {
      updateSetting(key, value)
      const next = { ...useAppStore.getState().settings, [key]: value }
      saveSettingsToCache(next)
      syncToBackend(next)
    },
    [updateSetting, syncToBackend],
  )

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <h2 className={styles.title}>Settings</h2>
        {onClose && (
          <button
            className='btn btn-ghost'
            onClick={onClose}
            style={{ padding: '4px 10px', fontSize: 11 }}
          >
            ✕
          </button>
        )}
      </header>

      <div className={styles.sections}>
        {/* ── Appearance ── */}
        <Section title='Appearance'>
          <div className={styles.row}>
            <div className={styles.rowLabel}>
              <span>Skin</span>
              <span className={styles.rowDesc}>Interface appearance</span>
            </div>
            <div className={styles.skinPicker}>
              {SKINS.map(skin => (
                <button
                  key={skin.id}
                  className={`${styles.skinOption} ${settings.theme === skin.id ? styles.skinOptionActive : ''}`}
                  onClick={() => change('theme', skin.id)}
                  title={`${skin.name} — ${skin.description}`}
                  aria-label={skin.name}
                >
                  <span
                    className={styles.skinSwatch}
                    style={{ background: skin.preview[0], borderColor: skin.preview[2] }}
                  />
                  <span className={styles.skinName}>{skin.name}</span>
                </button>
              ))}
            </div>
          </div>
          <SelectRow
            label='Language'
            description='Display language'
            value={settings.language}
            options={[
              { value: Language.En, label: 'English' },
              { value: Language.Es, label: 'Español (coming soon)', disabled: true },
            ]}
            onChange={v => change('language', v as Language)}
          />
          <SelectRow
            label='Font Size'
            value={settings.font_size}
            options={[
              { value: 'small', label: 'Small' },
              { value: 'medium', label: 'Medium' },
              { value: 'large', label: 'Large' },
            ]}
            onChange={v => change('font_size', v as PlayerSettingMap['font_size'])}
          />
          <ToggleRow
            label='Reduce Motion'
            description='Disable animations and transitions'
            checked={settings.reduce_motion}
            onChange={v => change('reduce_motion', v)}
          />
        </Section>

        {/* ── Gameplay ── */}
        <Section title='Gameplay'>
          <ToggleRow
            label='Show Move Hints'
            description='Highlight available moves on hover'
            checked={settings.show_move_hints}
            onChange={v => change('show_move_hints', v)}
          />
          <ToggleRow
            label='Confirm Move'
            description='Require confirmation before applying a move'
            checked={settings.confirm_move}
            onChange={v => change('confirm_move', v)}
          />
          <ToggleRow
            label='Turn Timer Warning'
            description='Alert when your turn timer is running low'
            checked={settings.show_timer_warning}
            onChange={v => change('show_timer_warning', v)}
          />
        </Section>

        {/* ── Notifications ── */}
        <Section title='Notifications'>
          <ToggleRow
            label='Direct Messages'
            checked={settings.notify_dm}
            onChange={v => change('notify_dm', v)}
          />
          <ToggleRow
            label='Game Invites'
            checked={settings.notify_game_invite}
            onChange={v => change('notify_game_invite', v)}
          />
          <ToggleRow
            label='Friend Requests'
            checked={settings.notify_friend_request}
            onChange={v => change('notify_friend_request', v)}
          />
          <ToggleRow
            label='Notification Sounds'
            checked={settings.notify_sound}
            onChange={v => change('notify_sound', v)}
          />
        </Section>

        {/* ── Audio ── */}
        <Section title='Audio'>
          <ToggleRow
            label='Mute All'
            checked={settings.mute_all}
            onChange={v => change('mute_all', v)}
          />
          <VolumeRow
            label='Master Volume'
            value={settings.volume_master}
            onChange={v => change('volume_master', v)}
          />
          <VolumeRow
            label='Sound Effects'
            value={settings.volume_sfx}
            onChange={v => change('volume_sfx', v)}
          />
          <VolumeRow
            label='UI Sounds'
            value={settings.volume_ui}
            onChange={v => change('volume_ui', v)}
          />
          <VolumeRow
            label='Notifications'
            value={settings.volume_notifications}
            onChange={v => change('volume_notifications', v)}
          />
          <VolumeRow
            label='Music'
            value={settings.volume_music}
            onChange={v => change('volume_music', v)}
          />
        </Section>

        {/* ── Privacy ── */}
        <Section title='Privacy'>
          <ToggleRow
            label='Show Online Status'
            description="Let other players see when you're online"
            checked={settings.show_online_status}
            onChange={v => change('show_online_status', v)}
          />
          <SelectRow
            label='Allow Direct Messages'
            value={settings.allow_dms}
            options={[
              { value: 'anyone', label: 'Anyone' },
              { value: 'friends_only', label: 'Friends only (coming soon)', disabled: true },
              { value: 'nobody', label: 'Nobody' },
            ]}
            onChange={v => change('allow_dms', v as PlayerSettingMap['allow_dms'])}
          />
        </Section>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Section wrapper
// ---------------------------------------------------------------------------

function Section({
  title,
  note,
  children,
}: {
  title: string
  note?: string
  children: React.ReactNode
}) {
  return (
    <section className={styles.section}>
      <p className={`label ${styles.sectionTitle}`}>{title}</p>
      {note && <p className={styles.sectionNote}>{note}</p>}
      <div className={styles.rows}>{children}</div>
    </section>
  )
}

// ---------------------------------------------------------------------------
// Toggle row
// ---------------------------------------------------------------------------

function ToggleRow({
  label,
  description,
  checked,
  onChange,
  disabled,
}: {
  label: string
  description?: string
  checked: boolean
  onChange: (v: boolean) => void
  disabled?: boolean
}) {
  return (
    <div className={`${styles.row} ${disabled ? styles.rowDisabled : ''}`}>
      <div className={styles.rowLabel}>
        <span>{label}</span>
        {description && <span className={styles.rowDesc}>{description}</span>}
      </div>
      <button
        role='switch'
        aria-checked={checked}
        className={`${styles.toggle} ${checked ? styles.toggleOn : ''}`}
        onClick={() => !disabled && onChange(!checked)}
        disabled={disabled}
        aria-label={label}
      >
        <span className={styles.toggleThumb} />
      </button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Select row
// ---------------------------------------------------------------------------

function SelectRow({
  label,
  description,
  value,
  options,
  onChange,
}: {
  label: string
  description?: string
  value: string | undefined
  options: { value: string; label: string; disabled?: boolean }[]
  onChange: (v: string) => void
}) {
  return (
    <div className={styles.row}>
      <div className={styles.rowLabel}>
        <span>{label}</span>
        {description && <span className={styles.rowDesc}>{description}</span>}
      </div>
      <select
        className={styles.select}
        value={value ?? ''}
        onChange={e => onChange(e.target.value)}
      >
        {options.map(o => (
          <option key={o.value} value={o.value} disabled={o.disabled}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Volume row — mute toggle + slider
// ---------------------------------------------------------------------------

function VolumeRow({
  label,
  value,
  onChange,
  disabled,
}: {
  label: string
  value: number
  onChange: (v: number) => void
  disabled?: boolean
}) {
  const muted = value === 0

  return (
    <div className={`${styles.row} ${styles.volumeRow} ${disabled ? styles.rowDisabled : ''}`}>
      <span className={styles.volumeLabel}>{label}</span>
      <div className={styles.volumeControls}>
        <button
          className={`${styles.muteBtn} ${muted ? styles.muteBtnActive : ''}`}
          onClick={() =>
            !disabled &&
            onChange(
              muted
                ? ((DEFAULT_SETTINGS[
                    label.toLowerCase().replace(' ', '_') as keyof typeof DEFAULT_SETTINGS
                  ] as number) ?? 1.0)
                : 0,
            )
          }
          disabled={disabled}
          aria-label={muted ? `Unmute ${label}` : `Mute ${label}`}
          title={muted ? 'Unmute' : 'Mute'}
        >
          {muted ? '🔇' : '🔊'}
        </button>
        <input
          type='range'
          min={0}
          max={1}
          step={0.01}
          value={value}
          disabled={disabled || muted}
          className={styles.slider}
          onChange={e => !disabled && onChange(parseFloat(e.target.value))}
          aria-label={`${label} volume`}
        />
        <span className={styles.volumeValue}>{Math.round(value * 100)}%</span>
      </div>
    </div>
  )
}
