/* eslint-disable */
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED FROM JSON SCHEMAS                  ##
 * ## DO NOT MODIFY BY HAND — edit shared/schemas/*.json instead ##
 * ---------------------------------------------------------------
 */

import { z } from 'zod'

// ---- Shared types (defs/) -------------------------------------------------

export const botProfileSchema = z.object({ "name": z.string(), "iterations": z.number().int(), "determinizations": z.number().int(), "exploration_c": z.number(), "aggressiveness": z.number(), "risk_aversion": z.number() })
export type BotProfile = z.infer<typeof botProfileSchema>

export const gameResultSchema = z.object({ "status": z.enum(["win","draw"]), "winner_id": z.string().optional() })
export type GameResult = z.infer<typeof gameResultSchema>

export const gameResultRecordSchema = z.object({ "id": z.string(), "session_id": z.string(), "game_id": z.string(), "winner_id": z.string().optional(), "is_draw": z.boolean(), "ended_by": z.enum(["win","draw","forfeit","timeout","ready_timeout","suspended"]), "duration_secs": z.number().int().optional(), "created_at": z.string().datetime({ offset: true }) })
export type GameResultRecord = z.infer<typeof gameResultRecordSchema>

export const gameSessionSchema = z.object({ "id": z.string(), "room_id": z.string(), "game_id": z.string(), "name": z.string().optional(), "mode": z.string(), "move_count": z.number().int(), "suspend_count": z.number().int(), "suspended_at": z.string().datetime({ offset: true }).optional(), "suspended_reason": z.string().optional(), "ready_players": z.array(z.string()), "turn_timeout_secs": z.number().int().optional(), "last_move_at": z.string().datetime({ offset: true }), "started_at": z.string().datetime({ offset: true }), "finished_at": z.string().datetime({ offset: true }).optional() })
export type GameSession = z.infer<typeof gameSessionSchema>

export const leaderboardEntrySchema = z.object({ "rank": z.number().int(), "player_id": z.string(), "display_rating": z.number(), "games_played": z.number().int() })
export type LeaderboardEntry = z.infer<typeof leaderboardEntrySchema>

export const lobbySettingSchema = z.object({ "key": z.string(), "label": z.string(), "description": z.string().optional(), "type": z.enum(["select","int"]), "default": z.string(), "options": z.array(z.object({ "value": z.string(), "label": z.string() })).optional(), "min": z.number().int().optional(), "max": z.number().int().optional() })
export type LobbySetting = z.infer<typeof lobbySettingSchema>

export const matchHistoryEntrySchema = z.object({ "id": z.string(), "session_id": z.string(), "game_id": z.string(), "outcome": z.enum(["win","loss","draw","forfeit"]), "ended_by": z.enum(["win","draw","forfeit","timeout","ready_timeout","suspended"]), "duration_secs": z.number().int().optional(), "created_at": z.string().datetime({ offset: true }) })
export type MatchHistoryEntry = z.infer<typeof matchHistoryEntrySchema>

export const moveSchema = z.object({ "id": z.string(), "session_id": z.string(), "player_id": z.string(), "payload": z.any(), "state_after": z.string().optional(), "move_number": z.number().int(), "applied_at": z.string().datetime({ offset: true }) })
export type Move = z.infer<typeof moveSchema>

export const notificationSchema = z.object({ "id": z.string(), "player_id": z.string(), "type": z.enum(["friend_request","friend_request_accepted","room_invitation","ban_issued"]), "payload": z.any(), "read_at": z.string().datetime({ offset: true }).optional(), "action_taken": z.string().optional(), "action_expires_at": z.string().datetime({ offset: true }).optional(), "created_at": z.string().datetime({ offset: true }) })
export type Notification = z.infer<typeof notificationSchema>

export const pauseVoteResultSchema = z.object({ "all_voted": z.boolean(), "votes": z.number().int(), "required": z.number().int() })
export type PauseVoteResult = z.infer<typeof pauseVoteResultSchema>

export const playerSchema = z.object({ "id": z.string(), "username": z.string(), "role": z.enum(["player","manager","owner"]), "avatar_url": z.string().optional(), "is_bot": z.boolean(), "created_at": z.string().datetime({ offset: true }), "deleted_at": z.string().datetime({ offset: true }).optional() })
export type Player = z.infer<typeof playerSchema>

export const playerStatsSchema = z.object({ "player_id": z.string(), "total_games": z.number().int(), "wins": z.number().int(), "losses": z.number().int(), "draws": z.number().int(), "forfeits": z.number().int() })
export type PlayerStats = z.infer<typeof playerStatsSchema>

export const queuePositionSchema = z.object({ "position": z.number().int(), "estimated_wait_secs": z.number().int() })
export type QueuePosition = z.infer<typeof queuePositionSchema>

export const readyVoteResultSchema = z.object({ "all_ready": z.boolean(), "ready_players": z.array(z.string()), "required": z.number().int() })
export type ReadyVoteResult = z.infer<typeof readyVoteResultSchema>

export const roomSchema = z.object({ "id": z.string(), "code": z.string(), "game_id": z.string(), "owner_id": z.string(), "status": z.enum(["waiting","in_progress","finished"]), "max_players": z.number().int(), "turn_timeout_secs": z.number().int().optional(), "created_at": z.string().datetime({ offset: true }), "updated_at": z.string().datetime({ offset: true }), "deleted_at": z.string().datetime({ offset: true }).optional() })
export type Room = z.infer<typeof roomSchema>

export const roomPlayerSchema = z.object({ "id": z.string(), "username": z.string(), "role": z.enum(["player","manager","owner"]), "avatar_url": z.string().optional(), "is_bot": z.boolean(), "created_at": z.string().datetime({ offset: true }), "deleted_at": z.string().datetime({ offset: true }).optional(), "seat": z.number().int(), "joined_at": z.string().datetime({ offset: true }) })
export type RoomPlayer = z.infer<typeof roomPlayerSchema>

export const sessionEventSchema = z.object({ "id": z.string(), "session_id": z.string(), "event_type": z.string(), "player_id": z.string().optional(), "payload": z.any().optional(), "occurred_at": z.string().datetime({ offset: true }) })
export type SessionEvent = z.infer<typeof sessionEventSchema>

export const gameInfoSchema = z.object({
  "id": z.string(),
  "name": z.string(),
  "min_players": z.number().int(),
  "max_players": z.number().int(),
  "settings": z.array(lobbySettingSchema)
})
export type GameInfo = z.infer<typeof gameInfoSchema>

// ---- Endpoint schemas ----------------------------------------------------

export const acceptMatchRequestSchema = z.object({ "match_id": z.string().min(1) })
export type AcceptMatchRequest = z.infer<typeof acceptMatchRequestSchema>

export const addBotRequestSchema = z.object({ "player_id": z.string().min(1), "profile": z.enum(["easy","medium","hard","aggressive"]).optional() })
export type AddBotRequest = z.infer<typeof addBotRequestSchema>

export const addBotResponseSchema = playerSchema
export type AddBotResponse = z.infer<typeof addBotResponseSchema>

export const applyMoveRequestSchema = z.object({ "player_id": z.string().min(1), "payload": z.record(z.string(), z.any()) })
export type ApplyMoveRequest = z.infer<typeof applyMoveRequestSchema>

export const applyMoveResponseSchema = z.object({
  "session": gameSessionSchema,
  "state": z.unknown(),
  "is_over": z.boolean(),
  "result": gameResultSchema.optional()
})
export type ApplyMoveResponse = z.infer<typeof applyMoveResponseSchema>

export const createRoomRequestSchema = z.object({ "game_id": z.string().min(1), "player_id": z.string().min(1), "turn_timeout_secs": z.number().int().gte(5).optional() })
export type CreateRoomRequest = z.infer<typeof createRoomRequestSchema>

export const createRoomResponseSchema = z.object({
  "room": roomSchema,
  "players": z.array(roomPlayerSchema),
  "settings": z.record(z.string(), z.string())
})
export type CreateRoomResponse = z.infer<typeof createRoomResponseSchema>

export const declineMatchRequestSchema = z.object({ "match_id": z.string().min(1) })
export type DeclineMatchRequest = z.infer<typeof declineMatchRequestSchema>

export const getLeaderboardResponseSchema = z.object({
  "game_id": z.string(),
  "entries": z.array(leaderboardEntrySchema),
  "total": z.number().int()
})
export type GetLeaderboardResponse = z.infer<typeof getLeaderboardResponseSchema>

export const getPlayerRatingResponseSchema = z.object({ "player_id": z.string(), "game_id": z.string(), "display_rating": z.number(), "games_played": z.number().int(), "win_streak": z.number().int(), "loss_streak": z.number().int() })
export type GetPlayerRatingResponse = z.infer<typeof getPlayerRatingResponseSchema>

export const getPlayerStatsResponseSchema = playerStatsSchema
export type GetPlayerStatsResponse = z.infer<typeof getPlayerStatsResponseSchema>

export const getRoomResponseSchema = z.object({
  "room": roomSchema,
  "players": z.array(roomPlayerSchema),
  "settings": z.record(z.string(), z.string())
})
export type GetRoomResponse = z.infer<typeof getRoomResponseSchema>

export const getSessionResponseSchema = z.object({
  "session": gameSessionSchema,
  "state": z.unknown(),
  "result": gameResultRecordSchema.optional()
})
export type GetSessionResponse = z.infer<typeof getSessionResponseSchema>

export const joinRoomRequestSchema = z.object({ "code": z.string().min(1), "player_id": z.string().min(1) })
export type JoinRoomRequest = z.infer<typeof joinRoomRequestSchema>

export const joinRoomResponseSchema = z.object({
  "room": roomSchema,
  "players": z.array(roomPlayerSchema),
  "settings": z.record(z.string(), z.string())
})
export type JoinRoomResponse = z.infer<typeof joinRoomResponseSchema>

export const leaveRoomRequestSchema = z.object({ "player_id": z.string().min(1) })
export type LeaveRoomRequest = z.infer<typeof leaveRoomRequestSchema>

export const listPlayerMatchesResponseSchema = z.object({
  "matches": z.array(matchHistoryEntrySchema),
  "total": z.number().int()
})
export type ListPlayerMatchesResponse = z.infer<typeof listPlayerMatchesResponseSchema>

export const listRoomsResponseSchema = z.array(roomSchema)
export type ListRoomsResponse = z.infer<typeof listRoomsResponseSchema>

export const removeBotRequestSchema = z.object({ "player_id": z.string().min(1) })
export type RemoveBotRequest = z.infer<typeof removeBotRequestSchema>

export const startGameRequestSchema = z.object({ "player_id": z.string().min(1) })
export type StartGameRequest = z.infer<typeof startGameRequestSchema>

export const surrenderRequestSchema = z.object({ "player_id": z.string().min(1) })
export type SurrenderRequest = z.infer<typeof surrenderRequestSchema>

export const updateRoomSettingRequestSchema = z.object({ "player_id": z.string().min(1), "value": z.string() })
export type UpdateRoomSettingRequest = z.infer<typeof updateRoomSettingRequestSchema>

export const votePauseRequestSchema = z.object({ "player_id": z.string().min(1) })
export type VotePauseRequest = z.infer<typeof votePauseRequestSchema>

export const voteRematchRequestSchema = z.object({ "player_id": z.string().min(1) })
export type VoteRematchRequest = z.infer<typeof voteRematchRequestSchema>

export const voteRematchResponseSchema = z.object({ "votes": z.number().int(), "total_players": z.number().int() })
export type VoteRematchResponse = z.infer<typeof voteRematchResponseSchema>

export const voteResumeRequestSchema = z.object({ "player_id": z.string().min(1) })
export type VoteResumeRequest = z.infer<typeof voteResumeRequestSchema>

