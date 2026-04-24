'use client'

import { useState, useRef } from 'react'
import { useRouter } from 'next/navigation'
import { useHostSocket } from '@/lib/useHostSocket'
import type {
  QuestionReleasedPayload,
  RoundReviewPayload,
  RoundReviewQuestion,
  LeaderboardPayload,
  LeaderboardEntry,
} from '@/types'

// ---- types ----------------------------------------------------------------

type GamePhase =
  | 'lobby'
  | 'question'       // host releasing questions one at a time
  | 'round_review'   // host reviewing all answers
  | 'round_scores'   // scores just released, transitioning
  | 'leaderboard'    // between-round leaderboard
  | 'ended'

interface ReleasedQuestion {
  id: string
  prompt: string
  type: string
  points: number
  posInRound: number
  answerCount: number
}

// ---- component -------------------------------------------------------------

interface Props {
  code: string
  gameID: string
  gameStatus: string
  initialPlayers?: string[]
  wsBase: string
  hostToken: string
}

export default function HostGame({ code, gameID, gameStatus, initialPlayers = [], wsBase, hostToken }: Props) {
  const router = useRouter()

  // Track whether the game has been started from this session so onOpen
  // knows whether to show the lobby or wait for server state messages.
  const gameStartedRef = useRef(gameStatus === 'in_progress')

  const [phase, setPhase] = useState<GamePhase>('lobby')
  const [players, setPlayers] = useState<string[]>(initialPlayers)

  // Round state
  const [currentRound, setCurrentRound] = useState(1)
  const [totalRounds, setTotalRounds] = useState(1)
  const [totalInRound, setTotalInRound] = useState(0)
  const [releasedCount, setReleasedCount] = useState(0)
  const [releasedQuestions, setReleasedQuestions] = useState<ReleasedQuestion[]>([])

  // Review state
  const [reviewData, setReviewData] = useState<RoundReviewPayload | null>(null)

  // Leaderboard
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([])
  const [isFinal, setIsFinal] = useState(false)

  // ---- socket ---------------------------------------------------------------

  const {
    status,
    retry,
    startGame,
    releaseQuestion,
    endRound,
    releaseScores,
    overrideAnswer,
    startNextRound,
    endGame,
  } = useHostSocket(wsBase, code, hostToken, {
    onOpen: () => {
      // If the game hasn't started yet, show the lobby. If the host reconnects
      // mid-game, wait for the server to push the current state instead.
      if (!gameStartedRef.current) setPhase('lobby')
    },

    onLobbyUpdate: (players) => {
      setPlayers(players.map(p => p.display_name))
    },

    onGameStarted: (p) => {
      setCurrentRound(p.round ?? 1)
      setTotalRounds(p.total_rounds ?? 1)
      setReleasedCount(0)
      setReleasedQuestions([])
      setReviewData(null)
      setPhase('question')
    },

    onQuestionReleased: (p: QuestionReleasedPayload) => {
      setCurrentRound(p.round)
      setTotalRounds(p.total_rounds)
      setTotalInRound(p.total_in_round)
      setReleasedCount(p.pos_in_round)
      setReleasedQuestions(prev => {
        if (prev.find(q => q.id === p.question.id)) return prev
        return [...prev, {
          id: p.question.id,
          prompt: p.question.prompt,
          type: p.question.type,
          points: p.question.points,
          posInRound: p.pos_in_round,
          answerCount: 0,
        }]
      })
      setPhase('question')
    },

    onScoreboardUpdate: (p) => {
      if (p.question_id) {
        setReleasedQuestions(prev =>
          prev.map(q => q.id === p.question_id ? { ...q, answerCount: p.answer_count } : q)
        )
      }
    },

    onRoundReview: (p: RoundReviewPayload) => {
      setReviewData(p)
      setCurrentRound(p.round)
      setTotalRounds(p.total_rounds)
      setPhase('round_review')
    },

    onRoundScores: (p) => {
      setCurrentRound(p.round)
      setTotalRounds(p.total_rounds)
      setPhase('round_scores')
    },

    onRoundLeaderboard: (p: LeaderboardPayload) => {
      setLeaderboard(p.entries ?? [])
      setCurrentRound(p.round)
      setTotalRounds(p.total_rounds)
      setIsFinal(false)
      setPhase('leaderboard')
    },

    onGameEnded: (p: LeaderboardPayload) => {
      setLeaderboard(p.entries ?? [])
      setIsFinal(true)
      setPhase('ended')
    },
  })

  // ---- host action wrappers -------------------------------------------------
  // Wrap startGame and startNextRound to update local state alongside the send.

  function handleStartGame() {
    gameStartedRef.current = true
    startGame()
  }

  function handleStartNextRound() {
    setReleasedCount(0)
    setReleasedQuestions([])
    setReviewData(null)
    startNextRound()
  }

  // ---- render ---------------------------------------------------------------

  const medals = ['🥇', '🥈', '🥉']

  return (
    <div className="min-h-screen bg-slate-100 px-4 py-8">
      <div className="mx-auto max-w-2xl space-y-4">

        {/* Header strip */}
        <div className="flex items-center justify-between">
          <p className="font-display text-2xl font-bold text-brand-blue">
            Qui<span className="italic">bb</span>le
          </p>
          <div className="rounded-full bg-brand-blue/10 px-4 py-1.5">
            <span className="font-display text-lg font-bold tracking-widest text-brand-blue">{code}</span>
          </div>
        </div>

        {/* ---- CONNECTING / RECONNECTING ---- */}
        {(status === 'connecting' || status === 'reconnecting') && (
          <Card>
            <p className="text-center text-slate-400">Connecting to game…</p>
          </Card>
        )}

        {/* ---- ERROR ---- */}
        {status === 'failed' && (
          <Card accent="red">
            <p className="mb-3 text-sm text-brand-red">
              Could not connect after several attempts. Check the API server is running.
            </p>
            <div className="flex gap-3">
              <button
                onClick={retry}
                className="flex-1 rounded-lg bg-brand-blue py-2.5 text-sm font-semibold text-white hover:bg-brand-blue/90"
              >
                Retry
              </button>
              <button
                onClick={() => router.push('/games')}
                className="flex-1 rounded-lg border border-gray-200 py-2.5 text-sm text-slate-600 hover:bg-slate-50"
              >
                Back to Games
              </button>
            </div>
          </Card>
        )}

        {/* ---- GAME UI (only shown when socket is open) ---- */}
        {status === 'open' && (
          <>
            {/* ---- LOBBY ---- */}
            {phase === 'lobby' && (
              <Card>
                <p className="mb-1 text-xs font-medium uppercase tracking-wide text-slate-400">
                  Waiting for players
                </p>
                <p className="font-display text-5xl font-bold tracking-widest text-brand-blue text-center py-5">
                  {code}
                </p>
                <p className="text-center text-xs text-slate-400 mb-5">
                  Players join at the Quibble join page and enter this code
                </p>
                {players.length > 0 && (
                  <ul className="mb-5 space-y-1">
                    {players.map(name => (
                      <li key={name} className="flex items-center gap-2 rounded-lg bg-slate-50 px-3 py-2 text-sm text-slate-700">
                        <span className="text-brand-blue text-xs">●</span> {name}
                      </li>
                    ))}
                  </ul>
                )}
                <button
                  onClick={handleStartGame}
                  className="w-full rounded-xl bg-brand-blue py-3 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
                >
                  Start Game
                </button>
              </Card>
            )}

            {/* ---- QUESTION PHASE ---- */}
            {phase === 'question' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-sm">
                  <span className="font-semibold text-slate-700">
                    Round {currentRound}{totalRounds > 1 ? ` of ${totalRounds}` : ''}
                  </span>
                  <span className="text-slate-500">
                    {releasedCount} of {totalInRound || '?'} questions released
                  </span>
                </div>

                {releasedQuestions.length === 0 ? (
                  <Card>
                    <p className="text-center text-sm text-slate-400 py-2">
                      Press &quot;Release Question&quot; to reveal the first question to players.
                    </p>
                  </Card>
                ) : (
                  <div className="space-y-2">
                    {releasedQuestions.map(q => (
                      <div key={q.id} className="flex items-center gap-3 rounded-xl border border-gray-100 bg-white px-4 py-3 shadow-sm">
                        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-brand-blue/10 text-xs font-bold text-brand-blue">
                          {q.posInRound}
                        </span>
                        <p className="flex-1 text-sm text-slate-700 truncate">{q.prompt}</p>
                        <span className="text-xs text-slate-400 shrink-0">
                          {q.answerCount} {q.answerCount === 1 ? 'answer' : 'answers'}
                        </span>
                      </div>
                    ))}
                  </div>
                )}

                <div className="flex gap-3">
                  {(!totalInRound || releasedCount < totalInRound) && (
                    <button
                      onClick={releaseQuestion}
                      className="flex-1 rounded-xl bg-brand-blue py-3 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
                    >
                      Release Question {releasedCount > 0 ? releasedCount + 1 : 1}
                    </button>
                  )}
                  {releasedCount > 0 && (
                    <button
                      onClick={endRound}
                      className="flex-1 rounded-xl border-2 border-brand-blue py-3 text-sm font-semibold text-brand-blue hover:bg-brand-blue/5 transition-colors"
                    >
                      End Round → Review
                    </button>
                  )}
                </div>
              </div>
            )}

            {/* ---- ROUND REVIEW ---- */}
            {phase === 'round_review' && reviewData && (
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <p className="text-base font-semibold text-slate-800">
                    Round {reviewData.round} — Answer Review
                  </p>
                  <p className="text-xs text-slate-400">Tap incorrect answers to override them</p>
                </div>

                {reviewData.questions.map(qReview => (
                  <ReviewQuestionCard
                    key={qReview.question_id}
                    question={qReview}
                    onOverride={overrideAnswer}
                  />
                ))}

                {reviewData.questions.length === 0 && (
                  <Card>
                    <p className="text-center text-sm text-slate-400">
                      No questions were released this round.
                    </p>
                  </Card>
                )}

                <button
                  onClick={releaseScores}
                  className="w-full rounded-xl bg-brand-blue py-3 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
                >
                  Release Scores to Players
                </button>
              </div>
            )}

            {/* ---- TRANSITIONING (scores released, waiting for leaderboard) ---- */}
            {phase === 'round_scores' && (
              <Card>
                <p className="text-center text-sm text-slate-500">
                  Scores released — loading leaderboard…
                </p>
              </Card>
            )}

            {/* ---- LEADERBOARD / ENDED ---- */}
            {(phase === 'leaderboard' || phase === 'ended') && (
              <Card>
                <p className="mb-4 text-base font-semibold text-slate-800">
                  {isFinal
                    ? '🏁 Final Results'
                    : `📊 After Round ${currentRound}${totalRounds > 1 ? ` of ${totalRounds}` : ''}`}
                </p>
                {leaderboard.length === 0 ? (
                  <p className="text-sm text-slate-400">No scores yet.</p>
                ) : (
                  <ul className="mb-5 space-y-2">
                    {leaderboard.map(e => (
                      <li key={e.rank} className="flex items-center gap-3 rounded-lg bg-slate-50 px-4 py-3">
                        <span className="w-8 text-center text-lg">
                          {e.rank <= 3 ? medals[e.rank - 1] : `#${e.rank}`}
                        </span>
                        <span className="flex-1 text-sm font-medium text-slate-700">{e.display_name}</span>
                        <span className="text-sm font-semibold text-slate-800">{e.score.toLocaleString()}</span>
                      </li>
                    ))}
                  </ul>
                )}
                {!isFinal && (
                  <div className="flex gap-3">
                    {currentRound < totalRounds && (
                      <button
                        onClick={handleStartNextRound}
                        className="flex-1 rounded-xl bg-brand-blue py-3 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
                      >
                        Start Round {currentRound + 1}
                      </button>
                    )}
                    <button
                      onClick={endGame}
                      className="flex-1 rounded-xl border border-gray-200 py-3 text-sm text-slate-600 hover:bg-slate-50"
                    >
                      End Game
                    </button>
                  </div>
                )}
                {isFinal && (
                  <button
                    onClick={() => router.push('/games')}
                    className="w-full rounded-lg border border-gray-200 py-2.5 text-sm text-slate-600 hover:bg-slate-50"
                  >
                    Back to Games
                  </button>
                )}
              </Card>
            )}
          </>
        )}

      </div>
    </div>
  )
}

// ---- ReviewQuestionCard ---------------------------------------------------

function ReviewQuestionCard({
  question,
  onOverride,
}: {
  question: RoundReviewQuestion
  onOverride: (questionID: string, playerID: string) => void
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm">
      <div className="border-b border-gray-100 bg-slate-50 px-4 py-3">
        <p className="text-sm font-semibold text-slate-800">{question.prompt}</p>
        <p className="mt-0.5 text-xs text-emerald-700 font-medium">
          ✓ {question.correct_answers.join(' · ')}
        </p>
      </div>
      {question.answers.length === 0 ? (
        <p className="px-4 py-3 text-xs text-slate-400 italic">No answers submitted.</p>
      ) : (
        <ul className="divide-y divide-gray-50">
          {question.answers.map(a => (
            <li key={a.player_id} className="flex items-center gap-3 px-4 py-2.5">
              <span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-bold ${
                a.correct ? 'bg-emerald-100 text-emerald-700' : 'bg-red-50 text-brand-red'
              }`}>
                {a.correct ? '✓' : '✗'}
              </span>
              <span className="flex-1 text-sm font-medium text-slate-700">{a.player_name}</span>
              <span className={`text-sm ${a.correct ? 'text-slate-600' : 'text-slate-500'}`}>
                &ldquo;{a.answer}&rdquo;
              </span>
              {a.overridden && (
                <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700">
                  overridden
                </span>
              )}
              {!a.correct && !a.overridden && (
                <button
                  onClick={() => onOverride(question.question_id, a.player_id)}
                  className="rounded-lg border border-emerald-200 px-2.5 py-1 text-xs font-medium text-emerald-700 hover:bg-emerald-50 transition-colors"
                >
                  Mark ✓
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

// ---- Card helper ----------------------------------------------------------

function Card({
  children,
  accent,
}: {
  children: React.ReactNode
  accent?: 'blue' | 'red'
}) {
  return (
    <div className="overflow-hidden rounded-2xl border border-gray-100 bg-white shadow-sm">
      <div className={`h-[3px] ${accent === 'red' ? 'bg-brand-red' : 'bg-brand-blue'}`} />
      <div className="p-6">{children}</div>
    </div>
  )
}
