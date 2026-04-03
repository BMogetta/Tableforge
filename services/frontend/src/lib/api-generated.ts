/* eslint-disable */
/* tslint:disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

export enum ResultStatus {
  ResultWin = "win",
  ResultDraw = "draw",
}

export enum EndedBy {
  EndedByNormal = "win",
  EndedByDraw = "draw",
  EndedByForfeit = "forfeit",
  EndedByTimeout = "timeout",
  EndedByReadyTimeout = "ready_timeout",
  EndedBySuspended = "suspended",
}

export interface GameResult {
  created_at?: string;
  duration_secs?: number;
  ended_by?: EndedBy;
  game_id?: string;
  id?: string;
  is_draw?: boolean;
  session_id?: string;
  winner_id?: string;
}

export interface GameSessionDTO {
  finished_at?: string;
  game_id: string;
  id: string;
  last_move_at?: string;
  mode: string;
  move_count: number;
  name: string;
  ready_players: string[];
  room_id: string;
  started_at?: string;
  suspend_count: number;
  suspended_at?: string;
  suspended_reason?: string;
  turn_timeout_secs: number;
}

export interface MoveDTO {
  applied_at: string;
  id: string;
  move_number: number;
  payload: any;
  player_id: string;
  session_id: string;
  state_after?: string;
}

export interface MoveResponse {
  is_over: boolean;
  result?: Result;
  session: GameSessionDTO;
  state: any;
}

export interface PauseVoteResult {
  /** AllVoted is true when consensus was reached and the session state changed. */
  all_voted: boolean;
  /** Required is the number of human participants who must vote. */
  required: number;
  /** Votes is the number of players that have voted so far. */
  votes: number;
}

export interface ReadyVoteResult {
  all_ready: boolean;
  ready_players: string[];
  required: number;
}

export interface RematchResponse {
  total_players: number;
  votes: number;
}

export interface Result {
  status?: ResultStatus;
  winner_id?: string;
}

export interface SessionEventDTO {
  event_type: string;
  id: string;
  occurred_at: string;
  payload: any;
  player_id: string;
  session_id: string;
}

export interface SessionResponse {
  result?: GameResult;
  session: GameSessionDTO;
  state: any;
}

export interface MoveRequest {
  payload?: Record<string, any>;
  player_id?: string;
}

export interface RematchRequest {
  player_id?: string;
}

export interface SurrenderRequest {
  player_id?: string;
}
