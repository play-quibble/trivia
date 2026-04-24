'use client'

import { useCallback } from 'react'
import { useGameSocket, SocketStatus } from './useGameSocket'
import type {
  LobbyPlayer,
  GameStartedPayload,
  QuestionReleasedPayload,
  ScoreboardUpdatePayload,
  RoundReviewPayload,
  RoundScoresPayload,
  LeaderboardPayload,
} from '@/types'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type { LobbyPlayer }

export interface HostSocketHandlers {
  onLobbyUpdate?: (players: LobbyPlayer[]) => void
  onGameStarted?: (payload: GameStartedPayload) => void
  onQuestionReleased?: (payload: QuestionReleasedPayload) => void
  onScoreboardUpdate?: (payload: ScoreboardUpdatePayload) => void
  onRoundReview?: (payload: RoundReviewPayload) => void
  onRoundScores?: (payload: RoundScoresPayload) => void
  onRoundLeaderboard?: (payload: LeaderboardPayload) => void
  onGameEnded?: (payload: LeaderboardPayload) => void
  onOpen?: () => void
}

export interface UseHostSocketResult {
  status: SocketStatus

  /** Manually retry after a failed connection (e.g. from an error-state button). */
  retry: () => void

  // Host control actions — each no-ops silently if the socket is not open.
  startGame: () => void
  releaseQuestion: () => void
  endRound: () => void
  releaseScores: () => void
  overrideAnswer: (questionID: string, playerID: string) => void
  startNextRound: () => void
  endGame: () => void
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

/**
 * Typed WebSocket hook for the host game panel.
 *
 * Wraps useGameSocket with host-specific message dispatch and exposes named
 * action functions so HostGame.tsx never touches raw WebSocket calls.
 *
 * @param wsBase    WebSocket base URL, e.g. "ws://localhost:8080"
 * @param code      6-character game code
 * @param hostToken Auth0 access token (passed as ?token= query param)
 * @param handlers  Message handler callbacks — all optional; provide only what the component needs
 */
export function useHostSocket(
  wsBase: string,
  code: string,
  hostToken: string,
  handlers: HostSocketHandlers,
): UseHostSocketResult {
  const url = `${wsBase}/ws/${code}?token=${encodeURIComponent(hostToken)}`

  const onMessage = useCallback((type: string, payload: unknown) => {
    switch (type) {
      case 'lobby_update': {
        const p = payload as { players?: LobbyPlayer[] }
        handlers.onLobbyUpdate?.(p.players ?? [])
        break
      }
      case 'game_started':
        handlers.onGameStarted?.(payload as GameStartedPayload)
        break
      case 'question_released':
        handlers.onQuestionReleased?.(payload as QuestionReleasedPayload)
        break
      case 'scoreboard_update':
        handlers.onScoreboardUpdate?.(payload as ScoreboardUpdatePayload)
        break
      case 'round_review':
        handlers.onRoundReview?.(payload as RoundReviewPayload)
        break
      case 'round_scores':
        handlers.onRoundScores?.(payload as RoundScoresPayload)
        break
      case 'round_leaderboard':
        handlers.onRoundLeaderboard?.(payload as LeaderboardPayload)
        break
      case 'game_ended':
        handlers.onGameEnded?.(payload as LeaderboardPayload)
        break
    }
  // handlers intentionally excluded — useGameSocket keeps onMessage current
  // via optionsRef, so we don't need a new callback when handler identities change.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const { status, send, retry } = useGameSocket(url, {
    onMessage,
    onOpen: handlers.onOpen,
  })

  // ---- Actions -------------------------------------------------------------
  // Each wraps a send() call with a named function so the component has a
  // clean imperative API and never constructs message type strings directly.

  const startGame = useCallback(() => send('start_game'), [send])
  const releaseQuestion = useCallback(() => send('release_question'), [send])
  const endRound = useCallback(() => send('end_round'), [send])
  const releaseScores = useCallback(() => send('release_scores'), [send])
  const overrideAnswer = useCallback(
    (questionID: string, playerID: string) =>
      send('override_answer', { question_id: questionID, player_id: playerID }),
    [send],
  )
  const startNextRound = useCallback(() => send('start_next_round'), [send])
  const endGame = useCallback(() => send('end_game'), [send])

  return {
    status,
    retry,
    startGame,
    releaseQuestion,
    endRound,
    releaseScores,
    overrideAnswer,
    startNextRound,
    endGame,
  }
}
