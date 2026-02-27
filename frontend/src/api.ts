// All types mirror the Go store models.

export interface Player {
  id: string
  username: string
  avatar_url?: string
  created_at: string
}

export interface Room {
  id: string
  code: string
  game_id: string
  owner_id: string
  status: 'waiting' | 'in_progress' | 'finished'
  max_players: number
  created_at: string
}

export interface RoomView {
  room: Room
  players: Player[]
}

export interface GameSession {
  id: string
  room_id: string
  game_id: string
  state: unknown
  move_count: number
  started_at: string
  finished_at?: string
}

export interface Move {
  id: string
  session_id: string
  player_id: string
  payload: unknown
  move_number: number
  applied_at: string
}

export interface PlayerStats {
  player_id: string
  total_games: number
  wins: number
  losses: number
  draws: number
  forfeits: number
}

export interface LeaderboardEntry {
  player_id: string
  username: string
  avatar_url?: string
  wins: number
  losses: number
  draws: number
}

// --- HTTP client -------------------------------------------------------------

const BASE = '/api/v1'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    credentials: 'include',
    ...init,
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.error ?? res.statusText)
  }

  return res.json()
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

// --- Auth --------------------------------------------------------------------

export const auth = {
  me: () => request<Player>('/../../auth/me'),
  logout: () => fetch('/auth/logout', { method: 'POST', credentials: 'include' }),
  loginUrl: '/auth/github',
}

// --- Players -----------------------------------------------------------------

export const players = {
  stats: (id: string) => request<PlayerStats>(`/players/${id}/stats`),
  sessions: (id: string) => request<GameSession[]>(`/players/${id}/sessions`),
}

// --- Rooms -------------------------------------------------------------------

export const rooms = {
  list: () => request<RoomView[]>('/rooms'),
  get: (id: string) => request<RoomView>(`/rooms/${id}`),
  create: (gameId: string, playerId: string) =>
    request<RoomView>('/rooms', {
      method: 'POST',
      body: JSON.stringify({ game_id: gameId, player_id: playerId }),
    }),
  join: (code: string, playerId: string) =>
    request<RoomView>('/rooms/join', {
      method: 'POST',
      body: JSON.stringify({ code, player_id: playerId }),
    }),
  leave: (roomId: string, playerId: string) =>
    request<void>(`/rooms/${roomId}/leave`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  start: (roomId: string, playerId: string) =>
    request<GameSession>(`/rooms/${roomId}/start`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
}

// --- Sessions ----------------------------------------------------------------

export const sessions = {
  get: (id: string) => request<{ session: GameSession; state: { current_player_id: string; data: unknown } }>(`/sessions/${id}`),
  history: (id: string) => request<Move[]>(`/sessions/${id}/history`),
  move: (sessionId: string, playerId: string, payload: unknown) =>
  request<{ session: GameSession; state: { current_player_id: string; data: unknown }; is_over: boolean }>(
    `/sessions/${sessionId}/move`,
    {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, payload }),
    }
  ),
}

// --- Leaderboard -------------------------------------------------------------

export const leaderboard = {
  get: (gameId?: string, limit = 20) => {
    const params = new URLSearchParams({ limit: String(limit) })
    if (gameId) params.set('game_id', gameId)
    return request<LeaderboardEntry[]>(`/leaderboard?${params}`)
  },
}
