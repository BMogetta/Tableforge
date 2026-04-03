/* eslint-disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED FROM JSON SCHEMAS                  ##
 * ## DO NOT MODIFY BY HAND — edit shared/schemas/*.json instead ##
 * ---------------------------------------------------------------
 */

export interface GameResult {
  status: "win" | "draw";
  winner_id?: string;
}

export interface GameResultRecord {
  id: string;
  session_id: string;
  game_id: string;
  winner_id?: string;
  is_draw: boolean;
  ended_by: "win" | "draw" | "forfeit" | "timeout" | "ready_timeout" | "suspended";
  duration_secs?: number;
  created_at: string;
}

export interface GameSession {
  id: string;
  room_id: string;
  game_id: string;
  name?: string;
  mode: string;
  move_count: number;
  suspend_count: number;
  suspended_at?: string;
  suspended_reason?: string;
  ready_players: string[];
  turn_timeout_secs?: number;
  last_move_at: string;
  started_at: string;
  finished_at?: string;
}

export interface Move {
  id: string;
  session_id: string;
  player_id: string;
  payload: unknown;
  state_after?: string;
  move_number: number;
  applied_at: string;
}

export interface PauseVoteResult {
  all_voted: boolean;
  votes: number;
  required: number;
}

export interface ReadyVoteResult {
  all_ready: boolean;
  ready_players: string[];
  required: number;
}

export interface Room {
  id: string;
  code: string;
  game_id: string;
  owner_id: string;
  status: "waiting" | "in_progress" | "finished";
  max_players: number;
  turn_timeout_secs?: number;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface RoomPlayer {
  id: string;
  username: string;
  role: "player" | "manager" | "owner";
  avatar_url?: string;
  is_bot: boolean;
  created_at: string;
  deleted_at?: string;
  seat: number;
  joined_at: string;
}

export interface SessionEvent {
  id: string;
  session_id: string;
  event_type: string;
  player_id?: string;
  payload?: unknown;
  occurred_at: string;
}

export interface AddBotRequest {
  player_id: string;
  profile?: "easy" | "medium" | "hard" | "aggressive";
}

export interface ApplyMoveRequest {
  player_id: string;
  payload: {};
}

export interface ApplyMoveResponse {
  session: GameSession;
  state: unknown;
  is_over: boolean;
  result?: GameResult;
}

export interface CreateRoomRequest {
  game_id: string;
  player_id: string;
  turn_timeout_secs?: number;
}

export interface CreateRoomResponse {
  room: Room;
  players: RoomPlayer[];
  settings: {
    [k: string]: string;
  };
}

export interface GetSessionResponse {
  session: GameSession;
  state: unknown;
  result?: GameResultRecord;
}

export interface JoinRoomRequest {
  code: string;
  player_id: string;
}

export interface JoinRoomResponse {
  room: Room;
  players: RoomPlayer[];
  settings: {
    [k: string]: string;
  };
}

export interface LeaveRoomRequest {
  player_id: string;
}

export interface RemoveBotRequest {
  player_id: string;
}

export interface StartGameRequest {
  player_id: string;
}

export interface SurrenderRequest {
  player_id: string;
}

export interface UpdateRoomSettingRequest {
  player_id: string;
  value: string;
}

export interface VotePauseRequest {
  player_id: string;
}

export interface VoteRematchRequest {
  player_id: string;
}

export interface VoteRematchResponse {
  votes: number;
  total_players: number;
}

export interface VoteResumeRequest {
  player_id: string;
}

