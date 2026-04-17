import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { scenarios as scenariosApi, sessions } from '@/lib/api/sessions'
import { testId } from '@/utils/testId'

// Parses the active session UUID from `/game/<uuid>` URLs. Returns null when
// the user is on any other route — the picker hides itself in that case so
// it isn't an attractive nuisance from the lobby.
function useActiveSessionId(): string | null {
  const [pathname, setPathname] = useState(() => window.location.pathname)
  useEffect(() => {
    const update = () => setPathname(window.location.pathname)
    window.addEventListener('popstate', update)
    // Pushed navigations don't fire popstate. Poll lightly so the picker
    // refreshes when you click into a game from the lobby. Cheap enough.
    const id = window.setInterval(update, 1000)
    return () => {
      window.removeEventListener('popstate', update)
      window.clearInterval(id)
    }
  }, [])
  const match = pathname.match(/^\/game\/([0-9a-f-]{36})/i)
  return match ? match[1] : null
}

// Inherits CSS variables from the host devtools panel — no hardcoded colors,
// matches the look of WsDevtools.
const styles = {
  panel: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: 10,
    padding: 12,
    fontSize: 11,
    fontFamily: 'var(--font-mono, monospace)',
    color: 'var(--color-text-primary, #e8e4d9)',
  },
  meta: {
    color: 'var(--color-text-muted, #555)',
  },
  hint: {
    color: 'var(--color-text-muted, #555)',
    padding: 12,
    fontSize: 11,
    fontFamily: 'var(--font-mono, monospace)',
  },
  description: {
    margin: 0,
    color: 'var(--color-text-secondary, #7a7568)',
    fontSize: 11,
    lineHeight: 1.4,
  },
  control: {
    padding: '5px 8px',
    fontSize: 11,
    fontFamily: 'inherit',
    background: 'var(--color-bg-surface, rgba(0,0,0,0.2))',
    color: 'var(--color-text-primary, #e8e4d9)',
    border: '1px solid var(--color-border, rgba(255,255,255,0.1))',
    borderRadius: 3,
  },
  applyBtn: {
    padding: '5px 12px',
    fontSize: 11,
    fontFamily: 'inherit',
    background: 'transparent',
    color: 'var(--color-interactive, #7b8cde)',
    border: '1px solid var(--color-interactive, #7b8cde)',
    borderRadius: 3,
    cursor: 'pointer',
    alignSelf: 'flex-start' as const,
  },
  applyBtnDisabled: {
    color: 'var(--color-text-muted, #555)',
    borderColor: 'var(--color-border, rgba(255,255,255,0.1))',
    cursor: 'default' as const,
  },
  ok: {
    color: 'var(--color-interactive, #7b8cde)',
  },
  err: {
    color: 'var(--color-danger, #c33)',
  },
} as const

export function ScenarioPicker() {
  const sessionId = useActiveSessionId()

  const { data: session } = useQuery({
    queryKey: ['scenario-picker:session', sessionId],
    queryFn: () => sessions.get(sessionId!),
    enabled: !!sessionId,
  })
  const gameId = session?.session.game_id ?? null

  const {
    data: list = [],
    isLoading,
    error,
  } = useQuery({
    queryKey: ['scenario-picker:list', gameId],
    queryFn: () => scenariosApi.list(gameId!),
    enabled: !!gameId,
  })

  const [selectedId, setSelectedId] = useState<string>('')
  const [status, setStatus] = useState<'idle' | 'applying' | 'ok' | 'err'>('idle')
  const [errMsg, setErrMsg] = useState<string>('')

  const selected = useMemo(() => list.find(s => s.id === selectedId) ?? null, [list, selectedId])

  async function apply() {
    if (!sessionId || !selectedId) return
    setStatus('applying')
    setErrMsg('')
    try {
      await sessions.loadScenario(sessionId, selectedId)
      setStatus('ok')
      window.setTimeout(() => setStatus('idle'), 1500)
    } catch (e) {
      setStatus('err')
      setErrMsg(e instanceof Error ? e.message : String(e))
    }
  }

  if (!sessionId) {
    return <div style={styles.hint}>Open a game session to load a scenario.</div>
  }

  const applyDisabled = !selectedId || status === 'applying'

  return (
    <div {...testId('scenario-picker')} style={styles.panel}>
      <div style={styles.meta}>
        session <code>{sessionId.slice(0, 8)}…</code>
        {gameId && (
          <>
            {' '}
            · game <code>{gameId}</code>
          </>
        )}
      </div>

      {!gameId && <div style={styles.meta}>Loading session…</div>}
      {gameId && isLoading && <div style={styles.meta}>Loading scenarios…</div>}
      {gameId && !isLoading && error && (
        <div style={styles.err}>Failed to list scenarios: {String(error)}</div>
      )}
      {gameId && !isLoading && !error && list.length === 0 && (
        <div style={styles.meta}>
          No scenarios registered for <code>{gameId}</code>.
        </div>
      )}

      {list.length > 0 && (
        <>
          <select
            value={selectedId}
            onChange={e => setSelectedId(e.target.value)}
            style={styles.control}
            {...testId('scenario-picker-select')}
          >
            <option value=''>— pick a scenario —</option>
            {list.map(s => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>

          {selected && <p style={styles.description}>{selected.description}</p>}

          <button
            type='button'
            onClick={apply}
            disabled={applyDisabled}
            style={{ ...styles.applyBtn, ...(applyDisabled ? styles.applyBtnDisabled : null) }}
            {...testId('scenario-picker-apply')}
          >
            {status === 'applying' ? 'Applying…' : 'Apply scenario'}
          </button>

          {status === 'ok' && <div style={styles.ok}>Applied.</div>}
          {status === 'err' && <div style={styles.err}>Failed: {errMsg}</div>}
        </>
      )}
    </div>
  )
}
