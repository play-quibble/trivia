// Shared TypeScript types for the trivia web app.
// These mirror the JSON shapes returned by the Go API —
// keep them in sync with the response types in apps/api/internal/game/types.go.

export interface Bank {
  id: string
  owner_id: string
  name: string
  description?: string
  created_at: string
  updated_at: string
}

export interface Session {
  userId: string
  email: string
  name: string
}

export interface MCChoice {
  text: string
  correct: boolean
}

export interface Question {
  id: string
  bank_id: string
  type: 'text' | 'multiple_choice'
  prompt: string
  points: number
  position: number
  accepted_answers?: string[]
  choices?: MCChoice[]
  created_at: string
  updated_at: string
}

export const MAX_PROMPT_LEN  = 500
export const MAX_CHOICE_LEN  = 200
export const MAX_ANSWER_LEN  = 150
export const MAX_CHOICES     = 6
export const MIN_CHOICES     = 2
export const MAX_ANSWERS     = 10

// --- Quiz types ---

export interface Quiz {
  id: string
  owner_id: string
  name: string
  description?: string
  created_at: string
  updated_at: string
}

export interface QuizRound {
  id: string
  quiz_id: string
  round_number: number
  title?: string
  questions: Question[]
  created_at: string
}

export interface QuizDetail extends Quiz {
  rounds: QuizRound[]
}

// --- Game types ---

export type GameStatus = 'lobby' | 'in_progress' | 'completed' | 'cancelled'

export interface Game {
  id: string
  code: string
  status: GameStatus
  bank_id?: string
  quiz_id?: string
  current_question_idx: number
  current_round_idx: number
  round_size: number
  created_at: string
}

export interface GamePlayer {
  id: string
  display_name: string
  score: number
}

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

export interface QuestionPayload {
  id: string
  type: 'text' | 'multiple_choice'
  prompt: string
  points: number
  choices?: string[]
}

// question_released (new, quiz-based)
export interface QuestionReleasedPayload {
  pos_in_round: number   // 1-indexed position of this question within the round
  total_in_round: number // total questions in this round
  round: number          // current round number (1-indexed)
  total_rounds: number
  question: QuestionPayload
}

// round_ended — sent to all when host moves to review phase
export interface RoundEndedPayload {
  round: number
  total_rounds: number
}

// round_review — sent to host only
export interface RoundReviewAnswerEntry {
  player_id: string
  player_name: string
  answer: string
  correct: boolean
  overridden: boolean
}

export interface RoundReviewQuestion {
  question_id: string
  prompt: string
  correct_answers: string[]
  answers: RoundReviewAnswerEntry[]
}

export interface RoundReviewPayload {
  round: number
  total_rounds: number
  questions: RoundReviewQuestion[]
}

// round_scores — sent per-player (and a host variant)
export interface RoundScoreQuestion {
  question_id: string
  prompt: string
  correct_answers: string[]
  your_answer?: string    // absent on host variant
  correct?: boolean       // absent on host variant
  points_earned?: number  // absent on host variant
}

export interface RoundScoresPayload {
  round: number
  total_rounds: number
  questions: RoundScoreQuestion[]
  round_score?: number  // player total for this round
  is_host?: boolean
}

// round_leaderboard / game_ended
export interface LeaderboardPayload {
  entries: LeaderboardEntry[]
  round: number
  total_rounds: number
}

// answer_accepted — sent per-player
export interface AnswerAcceptedPayload {
  question_id: string
  correct: boolean
}

// scoreboard_update — sent to host
export interface ScoreboardUpdatePayload {
  question_id: string
  answer_count: number
}

// lobby_update
export interface LobbyUpdatePayload { player_name: string }

// game_started
export interface GameStartedPayload {
  round?: number
  total_rounds?: number
  // legacy fields (bank-based)
  total?: number
  round_size?: number
}
