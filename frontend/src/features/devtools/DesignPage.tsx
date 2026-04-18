import { useEffect, useState } from 'react'
import styles from './Design.module.css'

type Theme = 'dark' | 'parchment' | 'slate' | 'ivory'

export function DesignPage() {
  const [theme, setTheme] = useState<Theme>(() => {
    return (document.documentElement.getAttribute('data-theme') as Theme) ?? 'dark'
  })

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    return () => {
      document.documentElement.setAttribute('data-theme', 'dark')
    }
  }, [theme])

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <span className={styles.headerLabel}>Recess</span>
        <h1 className={styles.headerTitle}>Design System</h1>
        <div className={styles.themeToggle}>
          {(['dark', 'parchment', 'slate', 'ivory'] as Theme[]).map(t => (
            <button
              type='button'
              key={t}
              className={`${styles.themeBtn} ${theme === t ? styles.themeBtnActive : ''}`}
              onClick={() => setTheme(t)}
            >
              {t}
            </button>
          ))}
        </div>
        <span className={styles.headerVersion}>v1.0 — dev only</span>
      </header>

      <main className={styles.main}>
        {/* ── Colors ─────────────────────────────────────────────────────── */}
        <Section title='Color primitives'>
          <div className={styles.colorGrid}>
            <ColorSwatch
              name='interactive'
              value='var(--color-interactive)'
              hex='--color-interactive'
            />
            <ColorSwatch
              name='interactive-dim'
              value='var(--color-interactive-dim)'
              hex='--color-interactive-dim'
            />
            <ColorSwatch
              name='interactive-glow'
              value='var(--color-interactive-glow)'
              hex='--color-interactive-glow'
            />
            <ColorSwatch name='danger' value='var(--color-danger)' hex='--color-danger' />
            <ColorSwatch name='success' value='var(--color-success)' hex='--color-success' />
          </div>
        </Section>

        <Section title='Surfaces'>
          <div className={styles.colorGrid}>
            <SurfaceSwatch name='color-bg-base' value='var(--color-bg-base)' />
            <SurfaceSwatch name='color-bg-surface' value='var(--color-bg-surface)' />
            <SurfaceSwatch name='color-bg-elevated' value='var(--color-bg-elevated)' />
            <SurfaceSwatch name='color-bg-hover' value='var(--color-bg-hover)' />
          </div>
        </Section>

        <Section title='Text'>
          <div className={styles.typeScale}>
            <div style={{ color: 'var(--color-text-primary)', fontSize: 'var(--text-base)' }}>
              text-primary — The quick brown fox
            </div>
            <div style={{ color: 'var(--color-text-secondary)', fontSize: 'var(--text-base)' }}>
              text-secondary — The quick brown fox
            </div>
            <div style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-base)' }}>
              text-muted — The quick brown fox
            </div>
          </div>
        </Section>

        {/* ── Typography ─────────────────────────────────────────────────── */}
        <Section title='Typography'>
          <div className={styles.typeScale}>
            <div className={styles.typeRow}>
              <span className={styles.typeLabel}>display / 32px</span>
              <span
                style={{
                  fontFamily: 'var(--font-display)',
                  fontSize: 32,
                  color: 'var(--color-interactive)',
                  letterSpacing: '0.08em',
                }}
              >
                Recess
              </span>
            </div>
            <div className={styles.typeRow}>
              <span className={styles.typeLabel}>display / 20px</span>
              <span
                style={{
                  fontFamily: 'var(--font-display)',
                  fontSize: 20,
                  color: 'var(--color-text-primary)',
                  letterSpacing: '0.1em',
                }}
              >
                Room Code: ALPHA
              </span>
            </div>
            <div className={styles.typeRow}>
              <span className={styles.typeLabel}>mono / 14px</span>
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: 'var(--text-base)',
                  color: 'var(--color-text-primary)',
                }}
              >
                Body text — game in progress
              </span>
            </div>
            <div className={styles.typeRow}>
              <span className={styles.typeLabel}>mono / 12px</span>
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: 'var(--text-sm)',
                  color: 'var(--color-text-secondary)',
                }}
              >
                Secondary — Move 14 · 3 players
              </span>
            </div>
            <div className={styles.typeRow}>
              <span className={styles.typeLabel}>mono / 11px uppercase</span>
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: 'var(--text-xs)',
                  letterSpacing: '0.1em',
                  textTransform: 'uppercase',
                  color: 'var(--color-text-secondary)',
                }}
              >
                Label — Waiting for player
              </span>
            </div>
          </div>
        </Section>

        {/* ── Buttons ────────────────────────────────────────────────────── */}
        <Section title='Buttons'>
          <div className={styles.row}>
            <button type='button' className='btn btn-primary'>
              Start Game
            </button>
            <button type='button' className='btn btn-ghost'>
              Leave Room
            </button>
            <button type='button' className='btn btn-danger'>
              Forfeit
            </button>
            <button type='button' className='btn btn-secondary'>
              View Replay
            </button>
            <button type='button' className='btn btn-primary' disabled={true}>
              Disabled
            </button>
          </div>
          <div className={styles.row} style={{ marginTop: 12 }}>
            <button
              type='button'
              className='btn btn-ghost'
              style={{ padding: '4px 10px', fontSize: 11 }}
            >
              ← Lobby
            </button>
            <button
              type='button'
              className='btn btn-ghost'
              style={{ padding: '4px 10px', fontSize: 11 }}
            >
              ⏸ Pause
            </button>
          </div>
        </Section>

        {/* ── Badges ─────────────────────────────────────────────────────── */}
        <Section title='Badges'>
          <div className={styles.row}>
            <span className='badge badge-amber'>Host</span>
            <span className='badge badge-amber'>Waiting</span>
            <span className='badge badge-muted'>You</span>
            <span className='badge badge-muted'>Spectating</span>
            <span className='badge badge-muted'>Bot</span>
          </div>
        </Section>

        {/* ── Inputs ─────────────────────────────────────────────────────── */}
        <Section title='Inputs'>
          <div className={styles.stack}>
            <div>
              <label className='label' htmlFor='design-room-code'>
                Room Code
              </label>
              <input
                id='design-room-code'
                className='input'
                placeholder='Enter code...'
                style={{ maxWidth: 280 }}
              />
            </div>
            <div>
              <label className='label' htmlFor='design-disabled-input'>
                Disabled
              </label>
              <input
                id='design-disabled-input'
                className='input'
                placeholder='Not editable'
                disabled={true}
                style={{ maxWidth: 280 }}
              />
            </div>
          </div>
        </Section>

        {/* ── Cards / surfaces ───────────────────────────────────────────── */}
        <Section title='Surfaces & cards'>
          <div className={styles.row} style={{ alignItems: 'flex-start' }}>
            <div className='card' style={{ flex: 1, maxWidth: 200 }}>
              <p
                style={{
                  fontSize: 'var(--text-xs)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                  color: 'var(--color-text-secondary)',
                  marginBottom: 'var(--space-2)',
                }}
              >
                bg-surface card
              </p>
              <p style={{ fontSize: 'var(--text-sm)', color: 'var(--color-text-primary)' }}>
                Game in progress
              </p>
            </div>
            <div
              style={{
                flex: 1,
                maxWidth: 200,
                background: 'var(--color-bg-elevated)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius)',
                padding: 20,
              }}
            >
              <p
                style={{
                  fontSize: 'var(--text-xs)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                  color: 'var(--color-text-secondary)',
                  marginBottom: 'var(--space-2)',
                }}
              >
                bg-elevated card
              </p>
              <p style={{ fontSize: 'var(--text-sm)', color: 'var(--color-text-primary)' }}>
                Vote to pause
              </p>
            </div>
          </div>
        </Section>

        {/* ── Borders ────────────────────────────────────────────────────── */}
        <Section title='Borders & dividers'>
          <div className={styles.stack}>
            <div
              style={{
                padding: '12px 16px',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius)',
              }}
            >
              <span style={{ fontSize: 'var(--text-sm)', color: 'var(--color-text-secondary)' }}>
                border — default (--color-border)
              </span>
            </div>
            <div
              style={{
                padding: '12px 16px',
                border: '1px solid var(--color-border-bright)',
                borderRadius: 'var(--radius)',
              }}
            >
              <span style={{ fontSize: 'var(--text-sm)', color: 'var(--color-text-secondary)' }}>
                border-bright — active / hover
              </span>
            </div>
            <hr className='divider' />
            <span style={{ fontSize: 'var(--text-xs)', color: 'var(--color-text-muted)' }}>
              divider utility
            </span>
          </div>
        </Section>

        {/* ── Status indicators ──────────────────────────────────────────── */}
        <Section title='Status indicators'>
          <div className={styles.stack}>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                padding: '12px 16px',
                background: 'var(--color-bg-surface)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius)',
              }}
            >
              <div
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  background: 'var(--color-interactive)',
                  boxShadow: '0 0 8px var(--color-interactive-glow)',
                  animation: 'pulse 2s ease infinite',
                }}
              />
              <span style={{ fontSize: 'var(--text-sm)', color: 'var(--color-text-secondary)' }}>
                Your turn
              </span>
              <span
                style={{
                  marginLeft: 'auto',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  fontSize: 'var(--text-xs)',
                  color: 'var(--color-text-muted)',
                }}
              >
                <span
                  style={{
                    width: 7,
                    height: 7,
                    borderRadius: '50%',
                    background: 'var(--color-success)',
                    boxShadow: '0 0 4px var(--color-success)',
                    display: 'inline-block',
                  }}
                />
                Opponent online
              </span>
            </div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                padding: '12px 16px',
                background: 'var(--color-bg-surface)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius)',
              }}
            >
              <div
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  background: 'var(--color-text-muted)',
                }}
              />
              <span style={{ fontSize: 'var(--text-sm)', color: 'var(--color-text-secondary)' }}>
                Opponent's turn
              </span>
              <span
                style={{
                  marginLeft: 'auto',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  fontSize: 'var(--text-xs)',
                  color: 'var(--color-text-muted)',
                }}
              >
                <span
                  style={{
                    width: 7,
                    height: 7,
                    borderRadius: '50%',
                    background: 'var(--color-text-muted)',
                    display: 'inline-block',
                  }}
                />
                Opponent offline
              </span>
            </div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                padding: '12px 16px',
                background: 'var(--color-bg-surface)',
                border: '1px solid var(--color-border)',
                borderRadius: 'var(--radius)',
              }}
            >
              <div
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  background: 'var(--color-text-muted)',
                }}
              />
              <span
                style={{
                  fontSize: 'var(--text-sm)',
                  fontStyle: 'italic',
                  color: 'var(--color-interactive-dim)',
                }}
              >
                Game paused
              </span>
            </div>
          </div>
        </Section>

        {/* ── Toasts ─────────────────────────────────────────────────────── */}
        <Section title='Toasts (static preview)'>
          <div className={styles.stack}>
            <div className={styles.toastPreview} data-variant='error'>
              <span className={styles.toastIcon}>✕</span>
              <div className={styles.toastBody}>
                <span className={styles.toastMessage}>Session not found</span>
                <span className={styles.toastCode}>NOT_FOUND</span>
              </div>
            </div>
            <div className={styles.toastPreview} data-variant='warning'>
              <span className={styles.toastIcon}>⚠</span>
              <div className={styles.toastBody}>
                <span className={styles.toastMessage}>Connection lost — reconnecting...</span>
              </div>
            </div>
            <div className={styles.toastPreview} data-variant='info'>
              <span className={styles.toastIcon}>ℹ</span>
              <div className={styles.toastBody}>
                <span className={styles.toastMessage}>Game saved to history</span>
              </div>
            </div>
          </div>
        </Section>

        {/* ── Loading states ─────────────────────────────────────────────── */}
        <Section title='Loading & empty states'>
          <div className={styles.stack}>
            <div style={{ textAlign: 'center', padding: '32px 0' }}>
              <p
                className='pulse'
                style={{
                  color: 'var(--color-text-muted)',
                  letterSpacing: '0.1em',
                  fontSize: 'var(--text-sm)',
                }}
              >
                Loading room...
              </p>
            </div>
            <div
              style={{
                textAlign: 'center',
                padding: '32px 0',
                borderTop: '1px solid var(--color-border)',
              }}
            >
              <p
                style={{
                  color: 'var(--color-text-muted)',
                  fontSize: 'var(--text-sm)',
                  marginBottom: 'var(--space-2)',
                }}
              >
                No rooms available
              </p>
              <p
                style={{
                  color: 'var(--color-text-muted)',
                  fontSize: 'var(--text-xs)',
                  letterSpacing: '0.05em',
                }}
              >
                Create one to get started
              </p>
            </div>
          </div>
        </Section>

        {/* ── Progress bar ───────────────────────────────────────────────── */}
        <Section title='Progress bar (GameLoading)'>
          <div className={styles.stack}>
            <div>
              <p
                style={{
                  fontSize: 'var(--text-xs)',
                  color: 'var(--color-text-muted)',
                  marginBottom: 'var(--space-2)',
                }}
              >
                0%
              </p>
              <div
                style={{
                  width: '100%',
                  height: 1,
                  background: 'var(--color-border)',
                  borderRadius: 'var(--radius)',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    height: '100%',
                    width: '0%',
                    background: 'var(--color-interactive)',
                    boxShadow: '0 0 8px var(--color-interactive-glow)',
                  }}
                />
              </div>
            </div>
            <div>
              <p
                style={{
                  fontSize: 'var(--text-xs)',
                  color: 'var(--color-text-muted)',
                  marginBottom: 'var(--space-2)',
                }}
              >
                60%
              </p>
              <div
                style={{
                  width: '100%',
                  height: 1,
                  background: 'var(--color-border)',
                  borderRadius: 'var(--radius)',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    height: '100%',
                    width: '60%',
                    background: 'var(--color-interactive)',
                    boxShadow: '0 0 8px var(--color-interactive-glow)',
                  }}
                />
              </div>
            </div>
            <div>
              <p
                style={{
                  fontSize: 'var(--text-xs)',
                  color: 'var(--color-text-muted)',
                  marginBottom: 'var(--space-2)',
                }}
              >
                100%
              </p>
              <div
                style={{
                  width: '100%',
                  height: 1,
                  background: 'var(--color-border)',
                  borderRadius: 'var(--radius)',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    height: '100%',
                    width: '100%',
                    background: 'var(--color-interactive)',
                    boxShadow: '0 0 8px var(--color-interactive-glow)',
                  }}
                />
              </div>
            </div>
          </div>
        </Section>

        {/* ── Spacing scale ──────────────────────────────────────────────── */}
        <Section title='Spacing scale'>
          <div className={styles.spacingGrid}>
            {[
              { name: '--space-1', px: 4 },
              { name: '--space-2', px: 8 },
              { name: '--space-3', px: 12 },
              { name: '--space-4', px: 16 },
              { name: '--space-5', px: 20 },
              { name: '--space-6', px: 24 },
              { name: '--space-8', px: 32 },
              { name: '--space-10', px: 40 },
              { name: '--space-12', px: 48 },
            ].map(({ name, px }) => (
              <div key={name} className={styles.spacingRow}>
                <div
                  style={{
                    width: `var(${name})`,
                    height: `var(${name})`,
                    background: 'var(--color-interactive-dim)',
                    borderRadius: 'var(--radius)',
                    flexShrink: 0,
                  }}
                />
                <span style={{ fontSize: 'var(--text-xs)', color: 'var(--color-text-secondary)' }}>
                  {name} · {px}px
                </span>
              </div>
            ))}
          </div>
        </Section>

        {/* ── Token architecture ─────────────────────────────────────────── */}
        <Section title='Token architecture'>
          <div className={styles.tokenTable}>
            <div className={styles.tokenHeader}>
              <span>Semántico</span>
              <span>Dark</span>
              <span>Light (Pergamino)</span>
            </div>
            {[
              ['--color-interactive', 'primitive-amber-500', 'primitive-ink-600'],
              ['--color-bg-base', 'primitive-obsidian-900', 'primitive-parchment-50'],
              ['--color-bg-surface', 'primitive-obsidian-800', 'primitive-parchment-100'],
              ['--color-bg-elevated', 'primitive-obsidian-700', 'primitive-parchment-200'],
              ['--color-text-primary', 'primitive-sand-100', 'primitive-ink-800'],
              ['--color-text-secondary', 'primitive-sand-400', 'primitive-ink-600'],
              ['--color-text-muted', 'primitive-sand-700', 'primitive-parchment-600'],
              ['--color-border', 'amber 0.12α', 'ink 0.12α'],
              ['--noise-opacity', '0.4', '0.15'],
            ].map(([token, dark, light]) => (
              <div key={token} className={styles.tokenRow}>
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: 'var(--text-xs)',
                    color: 'var(--color-interactive)',
                  }}
                >
                  {token}
                </span>
                <span style={{ fontSize: 'var(--text-xs)', color: 'var(--color-text-secondary)' }}>
                  {dark}
                </span>
                <span style={{ fontSize: 'var(--text-xs)', color: 'var(--color-text-secondary)' }}>
                  {light}
                </span>
              </div>
            ))}
          </div>
        </Section>
      </main>
    </div>
  )
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>{title}</h2>
      <div className={styles.sectionBody}>{children}</div>
    </section>
  )
}

function ColorSwatch({ name, value, hex }: { name: string; value: string; hex: string }) {
  return (
    <div className={styles.swatch}>
      <div className={styles.swatchColor} style={{ background: value }} />
      <span className={styles.swatchName}>{name}</span>
      <span className={styles.swatchHex}>{hex}</span>
    </div>
  )
}

function SurfaceSwatch({ name, value }: { name: string; value: string }) {
  return (
    <div className={styles.swatch}>
      <div
        className={styles.swatchColor}
        style={{ background: value, border: '1px solid var(--color-border-bright)' }}
      />
      <span className={styles.swatchName}>{name}</span>
    </div>
  )
}
