// Shared TypeScript types for the trivia web app.
// These mirror the JSON shapes returned by the Go API —
// keep them in sync with the response types in apps/api/internal/game/types.go.

export interface Bank {
  id: string
  owner_id: string
  name: string
  description?: string   // optional — may be absent from the JSON entirely
  created_at: string     // ISO 8601 date string, e.g. "2026-04-19T12:00:00Z"
  updated_at: string
}

export interface Session {
  userId: string
  email: string
  name: string
}

// MCChoice is one option in a multiple-choice question.
// Exactly one choice per question will have correct: true.
export interface MCChoice {
  text: string
  correct: boolean
}

// Question mirrors the questionResponse shape from the Go API.
// Exactly one of accepted_answers (text type) or choices (multiple_choice type)
// will be present for any given question.
export interface Question {
  id: string
  bank_id: string
  type: 'text' | 'multiple_choice'
  prompt: string
  points: number
  position: number
  accepted_answers?: string[]   // text questions: list of accepted responses
  choices?: MCChoice[]          // multiple_choice questions: options with correct flag
  created_at: string
  updated_at: string
}

// Character limits — keep in sync with the constants in apps/api/internal/game/service.go
export const MAX_PROMPT_LEN  = 500
export const MAX_CHOICE_LEN  = 200
export const MAX_ANSWER_LEN  = 150
export const MAX_CHOICES     = 6
export const MIN_CHOICES     = 2
export const MAX_ANSWERS     = 10

// --- Game types ---

export type GameStatus = 'lobby' | 'in_progress' | 'completed' | 'cancelled'

// Game mirrors the gameResponse shape from the Go API.
export interface Game {
  id: string
  code: string
  status: GameStatus
  bank_id: string
  current_question_idx: number
  round_size: number
  created_at: string
}

// GamePlayer is one entry in the player roster.
export interface GamePlayer {
  id: string
  display_name: string
  score: number
}

// JoinGameResponse is returned by POST /join.
export interface JoinGameResponse {
  game_code: string
  session_token: string
  display_name: string
}

// --- WebSocket message shapes ---

export interface LeaderboardEntry {
  rank: number
  display_name: string
  score: number
}

export interface AnswerEntry {
  display_name: string
  answer: string
  correct: boolean
  points: number
}

// QuestionPayload is embedded in question_revealed messages.
export interface QuestionPayload {
  id: string
  type: 'text' | 'multiple_choice'
  prompt: string
  points: number
  choices?: string[] // MC only — without correct flag
}

// WS message payloads (inbound from server)
export interface LobbyUpdatePayload   { player_name: string }
export interface GameStartedPayload   { total: number; round_size: number }
export interface QuestionRevealedPayload {
  index: number
  total: number
  round: number
  pos_in_round: number
  round_size: number
  question: QuestionPayload
}
export interface AnswersRevealedPayload {
  correct_answers: string[]
  entries: AnswerEntry[]
}
export interface LeaderboardPayload   { entries: LeaderboardEntry[] }
export interface AnswerAcceptedPayload { correct: boolean }
export interface ScoreboardUpdatePayload { answer_count: number }
