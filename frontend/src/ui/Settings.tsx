import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { useDebouncedCallback } from '@tanstack/react-pacer'
import { useAppStore } from '@/stores/store'
import { playerSettings, DEFAULT_SETTINGS, type PlayerSettingMap, Language } from '@/lib/api'
import { catchToAppError } from '@/utils/errors'
import { useToast } from './Toast'
import { useBlockPlayer } from '@/hooks/useBlockPlayer'
import styles from './Settings.module.css'
import { SKINS } from '@/lib/skins'
import { testId } from '@/utils/testId'

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
  const { blockedPlayers, unblock, unblockPending } = useBlockPlayer()
  const { t } = useTranslation()

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
        <h2 className={styles.title}>{t('settings.title')}</h2>
        {onClose && (
          <button className='btn btn-ghost btn-sm' onClick={onClose}>
            ✕
          </button>
        )}
      </header>

      <div className={styles.sections}>
        {/* ── Appearance ── */}
        <Section title={t('settings.appearance')}>
          <div className={styles.row}>
            <div className={styles.rowLabel}>
              <span>{t('settings.theme')}</span>
              <span className={styles.rowDesc}>{t('settings.themeDescription')}</span>
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
            label={t('settings.language')}
            description={t('settings.languageDescription')}
            value={settings.language}
            options={[
              { value: Language.En, label: 'English' },
              { value: Language.Es, label: 'Español' },
            ]}
            onChange={v => change('language', v as Language)}
          />
          <SelectRow
            label={t('settings.fontSize')}
            value={settings.font_size}
            options={[
              { value: 'small', label: 'Small' },
              { value: 'medium', label: 'Medium' },
              { value: 'large', label: 'Large' },
            ]}
            onChange={v => change('font_size', v as PlayerSettingMap['font_size'])}
          />
          <ToggleRow
            label={t('settings.reduceMotion')}
            description={t('settings.reduceMotionDescription')}
            checked={settings.reduce_motion}
            onChange={v => change('reduce_motion', v)}
          />
        </Section>

        {/* ── Gameplay ── */}
        <Section title={t('settings.gameplay')}>
          <ToggleRow
            label={t('settings.showMoveHints')}
            description={t('settings.showMoveHintsDescription')}
            checked={settings.show_move_hints}
            onChange={v => change('show_move_hints', v)}
          />
          <ToggleRow
            label={t('settings.confirmMove')}
            description={t('settings.confirmMoveDescription')}
            checked={settings.confirm_move}
            onChange={v => change('confirm_move', v)}
          />
          <ToggleRow
            label={t('settings.showTimerWarning')}
            description={t('settings.showTimerWarningDescription')}
            checked={settings.show_timer_warning}
            onChange={v => change('show_timer_warning', v)}
          />
        </Section>

        {/* ── Notifications ── */}
        <Section title={t('notifications.title')}>
          <ToggleRow
            label={t('settings.notifyDm')}
            checked={settings.notify_dm}
            onChange={v => change('notify_dm', v)}
          />
          <ToggleRow
            label={t('settings.notifyGameInvite')}
            checked={settings.notify_game_invite}
            onChange={v => change('notify_game_invite', v)}
          />
          <ToggleRow
            label={t('settings.notifyFriendRequest')}
            checked={settings.notify_friend_request}
            onChange={v => change('notify_friend_request', v)}
          />
          <ToggleRow
            label={t('settings.notifySound')}
            checked={settings.notify_sound}
            onChange={v => change('notify_sound', v)}
          />
        </Section>

        {/* ── Audio ── */}
        <Section title={t('settings.sound')}>
          <ToggleRow
            label={t('settings.muteAll')}
            checked={settings.mute_all}
            onChange={v => change('mute_all', v)}
          />
          <VolumeRow
            label={t('settings.masterVolume')}
            value={settings.volume_master}
            onChange={v => change('volume_master', v)}
          />
          <VolumeRow
            label={t('settings.sfxVolume')}
            value={settings.volume_sfx}
            onChange={v => change('volume_sfx', v)}
          />
          <VolumeRow
            label={t('settings.uiVolume')}
            value={settings.volume_ui}
            onChange={v => change('volume_ui', v)}
          />
          <VolumeRow
            label={t('settings.notificationVolume')}
            value={settings.volume_notifications}
            onChange={v => change('volume_notifications', v)}
          />
          <VolumeRow
            label={t('settings.musicVolume')}
            value={settings.volume_music}
            onChange={v => change('volume_music', v)}
          />
        </Section>

        {/* ── Privacy ── */}
        <Section title={t('settings.social')}>
          <ToggleRow
            label={t('settings.showOnlineStatus')}
            description={t('settings.showOnlineStatusDescription')}
            checked={settings.show_online_status}
            onChange={v => change('show_online_status', v)}
          />
          <SelectRow
            label={t('settings.allowDms')}
            value={settings.allow_dms}
            options={[
              { value: 'anyone', label: t('settings.allowDmsAnyone') },
              { value: 'friends_only', label: t('settings.allowDmsFriendsOnly') },
              { value: 'nobody', label: t('settings.allowDmsNobody') },
            ]}
            onChange={v => change('allow_dms', v as PlayerSettingMap['allow_dms'])}
          />
        </Section>

        {/* ── Blocked Players ── */}
        <Section
          title={t('settings.blockedPlayers')}
          note={
            blockedPlayers.length === 0
              ? undefined
              : t('settings.blockedPlayersDescription')
          }
        >
          {blockedPlayers.length === 0 ? (
            <p className={styles.emptyBlocked}>{t('settings.noBlockedPlayers')}</p>
          ) : (
            blockedPlayers.map(bp => (
              <div key={bp.id} className={styles.blockedRow} {...testId(`blocked-player-${bp.id}`)}>
                {bp.avatar_url && (
                  <img src={bp.avatar_url} alt='' className={styles.blockedAvatar} />
                )}
                <span className={styles.blockedName}>{bp.username}</span>
                <button
                  className='btn btn-ghost btn-sm'
                  onClick={() => unblock(bp.id)}
                  disabled={unblockPending}
                  {...testId(`unblock-btn-${bp.id}`)}
                >
                  {unblockPending ? '...' : t('settings.unblock')}
                </button>
              </div>
            ))
          )}
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
