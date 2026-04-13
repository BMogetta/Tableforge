import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { sessions, scenarios as scenariosApi } from '@/lib/api/sessions'
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

export function ScenarioPicker() {
  const sessionId = useActiveSessionId()

  const { data: session } = useQuery({
    queryKey: ['scenario-picker:session', sessionId],
    queryFn: () => sessions.get(sessionId!),
    enabled: !!sessionId,
  })
  const gameId = session?.session.game_id ?? null

  const { data: list = [], isLoading, error } = useQuery({
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
    return (
      <div style={{ padding: 12, fontSize: 12, color: 'var(--color-text-muted, #777)' }}>
        Open a game session to load a scenario.
      </div>
    )
  }

  return (
    <div
      {...testId('scenario-picker')}
      style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: 12, fontSize: 12 }}
    >
      <div style={{ color: 'var(--color-text-muted, #777)' }}>
        session <code>{sessionId.slice(0, 8)}…</code>
        {gameId && <> · game <code>{gameId}</code></>}
      </div>

      {!gameId && <div>Loading session…</div>}
      {gameId && isLoading && <div>Loading scenarios…</div>}
      {gameId && !isLoading && error && (
        <div style={{ color: 'var(--color-danger, #c33)' }}>
          Failed to list scenarios: {String(error)}
        </div>
      )}
      {gameId && !isLoading && !error && list.length === 0 && (
        <div style={{ color: 'var(--color-text-muted, #777)' }}>
          No scenarios registered for <code>{gameId}</code>.
        </div>
      )}

      {list.length > 0 && (
        <>
          <select
            value={selectedId}
            onChange={e => setSelectedId(e.target.value)}
            style={{ padding: 6, fontFamily: 'inherit', fontSize: 12 }}
            {...testId('scenario-picker-select')}
          >
            <option value=''>— pick a scenario —</option>
            {list.map(s => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>

          {selected && (
            <p style={{ margin: 0, color: 'var(--color-text-secondary, #aaa)' }}>
              {selected.description}
            </p>
          )}

          <button
            type='button'
            onClick={apply}
            disabled={!selectedId || status === 'applying'}
            style={{
              padding: '6px 12px',
              cursor: !selectedId || status === 'applying' ? 'default' : 'pointer',
              alignSelf: 'flex-start',
            }}
            {...testId('scenario-picker-apply')}
          >
            {status === 'applying' ? 'Applying…' : 'Apply scenario'}
          </button>

          {status === 'ok' && <div style={{ color: 'var(--color-success, #4c4)' }}>Applied.</div>}
          {status === 'err' && (
            <div style={{ color: 'var(--color-danger, #c33)' }}>Failed: {errMsg}</div>
          )}
        </>
      )}
    </div>
  )
}
