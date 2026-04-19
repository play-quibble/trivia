'use client'

import { useEffect, useRef, useState, useCallback } from 'react'
import type {
  Game,
  GamePlayer,
  LobbyUpdatePayload,
  GameStartedPayload,
  QuestionRevealedPayload,
  AnswersRevealedPayload,
  LeaderboardPayload,
  ScoreboardUpdatePayload,
} from '@/types'

// ---- types ----------------------------------------------------------------

type Phase =
  | 'connecting'
  | 'lobby'
  | 'question'
  | 'answers'
  | 'leaderboard'
  | 'ended'
  | 'error'

interface AnswerEntry {
  display_name: string
  answer: string
  correct: boolean
  points: number
}

interface LeaderboardEntry {
  rank: number
  display_name: string
  score: number
}

interface QuestionState {
  index: number
  total: number
  round: number
  pos_in_round: number
  round_size: number
  question: {
    id: string
    type: 'text' | 'multiple_choice'
    prompt: string
    points: number
    choices?: string[]
  }
}

// ---- helpers ---------------------------------------------------------------

function send(ws: WebSocket | null, type: string, payload?: unknown) {
  if (!ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ type, payload: payload ?? {} }))
}

// ---- component -------------------------------------------------------------

interface Props {
  game: Game
  initialPlayers: GamePlayer[]
  wsBase: string
  hostToken: string
}

export default function HostGame({ game, initialPlayers, wsBase, hostToken }: Props) {
  const wsRef = useRef<WebSocket | null>(null)
  const [phase, setPhase] = useState<Phase>('connecting')
  const [players, setPlayers] = useState<GamePlayer[]>(initialPlayers)
  const [question, setQuestion] = useState<QuestionState | null>(null)
  const [answerCount, setAnswerCount] = useState(0)
  const [answerEntries, setAnswerEntries] = useState<AnswerEntry[]>([])
  const [correctAnswers, setCorrectAnswers] = useState<string[]>([])
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([])
  const [isFinalLeaderboard, setIsFinalLeaderboard] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  const handleMessage = useCallback((raw: string) => {
    let msg: { type: string; payload?: unknown }
    try { msg = JSON.parse(raw) } catch { return }

    switch (msg.type) {
      case 'lobby_update': {
        const p = msg.payload as LobbyUpdatePayload
        setPlayers((prev) => [
          ...prev,
          { id: Math.random().toString(), display_name: p.player_name, score: 0 },
        ])
        break
      }
      case 'game_started': {
        const p = msg.payload as GameStartedPayload
        console.log('game started', p)
        setPhase('question')
        setAnswerCount(0)
        break
      }
      case 'question_revealed': {
        const p = msg.payload as QuestionRevealedPayload
        setQuestion(p)
        setPhase('question')
        setAnswerCount(0)
        setAnswerEntries([])
        setCorrectAnswers([])
        break
      }
      case 'scoreboard_update': {
        const p = msg.payload as ScoreboardUpdatePayload
        setAnswerCount(p.answer_count)
        break
      }
      case 'answers_revealed': {
        const p = msg.payload as AnswersRevealedPayload
        setCorrectAnswers(p.correct_answers)
        setAnswerEntries(p.entries)
        setPhase('answers')
        break
      }
      case 'round_leaderboard': {
        const p = msg.payload as LeaderboardPayload
        setLeaderboard(p.entries)
        setIsFinalLeaderboard(false)
        setPhase('leaderboard')
        break
      }
      case 'game_ended': {
        const p = msg.payload as LeaderboardPayload
        setLeaderboard(p.entries)
        setIsFinalLeaderboard(true)
        setPhase('ended')
        break
      }
    }
  }, [])

  useEffect(() => {
    const url = `${wsBase}/ws/${game.code}?host_token=${encodeURIComponent(hostToken)}`
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      // If game is already in lobby phase, show lobby; otherwise the server will
      // replay the current state when we first connect.
      setPhase(game.status === 'lobby' ? 'lobby' : 'question')
    }

    ws.onmessage = (e) => handleMessage(e.data)

    ws.onerror = () => {
      setErrorMsg('WebSocket connection failed. Is the API server running?')
      setPhase('error')
    }

    ws.onclose = () => {
      if (phase !== 'ended') {
        // Don't overwrite ended state on normal close
      }
    }

    return () => ws.close()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wsBase, game.code, hostToken])

  // ---- actions -------------------------------------------------------------

  const startGame  = () => send(wsRef.current, 'start_game')
  const revealAnswers  = () => send(wsRef.current, 'reveal_answers')
  const advanceQuestion = () => send(wsRef.current, 'advance_question')
  const endGame    = () => { if (confirm('End game now?')) send(wsRef.current, 'end_game') }

  // ---- render helpers ------------------------------------------------------

  const playerCount = players.length

  // ---- render --------------------------------------------------------------

  return (
    <main className="mx-auto max-w-4xl px-6 py-10">

      {/* Header bar */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <p className="text-xs font-medium uppercase tracking-widest text-slate-400">Game Code</p>
          <p className="font-display text-4xl font-bold tracking-[0.15em] text-brand-blue">
            {game.code}
          </p>
        </div>
        <div className="text-right text-sm text-slate-500">
          {playerCount} player{playerCount !== 1 ? 's' : ''}
          {phase !== 'lobby' && question && (
            <span className="ml-2 text-slate-400">
              Q {question.index + 1}/{question.total}
            </span>
          )}
        </div>
      </div>

      {/* ---- CONNECTING ---- */}
      {phase === 'connecting' && (
        <div className="rounded-xl bg-white p-12 text-center text-slate-400 shadow-sm">
          Connecting…
        </div>
      )}

      {/* ---- ERROR ---- */}
      {phase === 'error' && (
        <div className="rounded-xl border border-red-100 bg-red-50 p-8 text-center text-brand-red">
          {errorMsg}
        </div>
      )}

      {/* ---- LOBBY ---- */}
      {phase === 'lobby' && (
        <div className="space-y-6">
          {/* Join instructions */}
          <div className="overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm">
            <div className="h-[3px] bg-brand-blue" />
            <div className="p-8 text-center">
              <p className="mb-1 text-sm text-slate-500">Players join at</p>
              <p className="mb-4 text-lg font-semibold text-slate-700">
                {typeof window !== 'undefined' ? window.location.origin : ''}/join
              </p>
              <p className="mb-2 text-sm text-slate-500">and enter the code</p>
              <p className="font-display text-6xl font-bold tracking-[0.2em] text-brand-blue">
                {game.code}
              </p>
            </div>
          </div>

          {/* Player roster */}
          {players.length > 0 && (
            <div className="overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm">
              <div className="border-b border-gray-100 px-5 py-3">
                <p className="text-sm font-medium text-slate-600">
                  Players joined ({players.length})
                </p>
              </div>
              <ul className="divide-y divide-gray-50">
                {players.map((p) => (
                  <li key={p.id} className="flex items-center gap-3 px-5 py-3">
                    <div className="h-7 w-7 rounded-full bg-brand-blue/10 text-center text-xs font-semibold leading-7 text-brand-blue">
                      {p.display_name[0]?.toUpperCase()}
                    </div>
                    <span className="text-sm text-slate-700">{p.display_name}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Start button */}
          <div className="flex justify-center">
            <button
              onClick={startGame}
              disabled={players.length === 0}
              className="rounded-xl bg-brand-blue px-10 py-3 text-base font-semibold text-white shadow-sm hover:bg-brand-blue/90 disabled:opacity-40 transition-colors"
            >
              {players.length === 0 ? 'Waiting for players…' : 'Start Game'}
            </button>
          </div>
        </div>
      )}

      {/* ---- QUESTION ---- */}
      {phase === 'question' && question && (
        <div className="space-y-5">
          {/* Progress */}
          <RoundProgress
            index={question.index}
            total={question.total}
            round={question.round}
            posInRound={question.pos_in_round}
            roundSize={question.round_size}
          />

          {/* Question card */}
          <div className="overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm">
            <div className="h-[3px] bg-brand-blue" />
            <div className="p-8">
              <div className="mb-2 flex items-center gap-2">
                <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  question.question.type === 'multiple_choice'
                    ? 'bg-blue-100 text-blue-700'
                    : 'bg-emerald-100 text-emerald-700'
                }`}>
                  {question.question.type === 'multiple_choice' ? 'Multiple Choice' : 'Text Answer'}
                </span>
                <span className="text-xs text-slate-400">{question.question.points} pts</span>
              </div>
              <p className="text-2xl font-semibold leading-snug text-slate-800">
                {question.question.prompt}
              </p>

              {/* MC choices — shown to host for reference */}
              {question.question.type === 'multiple_choice' && question.question.choices && (
                <div className="mt-6 grid grid-cols-2 gap-3">
                  {question.question.choices.map((c, i) => (
                    <div
                      key={i}
                      className="rounded-lg border border-gray-200 bg-slate-50 px-4 py-3 text-sm text-slate-700"
                    >
                      {c}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Answer count */}
          <div className="flex items-center justify-between rounded-xl border border-gray-100 bg-white px-6 py-4 shadow-sm">
            <span className="text-sm text-slate-600">
              <span className="mr-1 text-2xl font-bold text-slate-800">{answerCount}</span>
              {' '}/ {players.length} answered
            </span>
            <div className="flex gap-3">
              <button
                onClick={endGame}
                className="rounded-lg border border-gray-200 px-4 py-2 text-sm text-slate-500 hover:border-brand-red hover:text-brand-red transition-colors"
              >
                End Game
              </button>
              <button
                onClick={revealAnswers}
                className="rounded-lg bg-brand-blue px-5 py-2 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
              >
                Reveal Answers
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ---- ANSWERS ---- */}
      {phase === 'answers' && question && (
        <div className="space-y-5">
          <RoundProgress
            index={question.index}
            total={question.total}
            round={question.round}
            posInRound={question.pos_in_round}
            roundSize={question.round_size}
          />

          {/* Correct answer(s) */}
          <div className="overflow-hidden rounded-xl border border-emerald-100 bg-emerald-50 px-6 py-4 shadow-sm">
            <p className="mb-1 text-xs font-medium uppercase tracking-widest text-emerald-600">
              Correct Answer{correctAnswers.length > 1 ? 's' : ''}
            </p>
            <p className="text-lg font-semibold text-emerald-800">
              {correctAnswers.join(' · ')}
            </p>
          </div>

          {/* Answer breakdown */}
          <div className="overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm">
            <div className="border-b border-gray-100 px-5 py-3">
              <p className="text-sm font-medium text-slate-600">
                Answers ({answerEntries.length}/{players.length})
              </p>
            </div>
            {answerEntries.length === 0 ? (
              <p className="px-5 py-4 text-sm text-slate-400">No answers submitted.</p>
            ) : (
              <ul className="divide-y divide-gray-50">
                {answerEntries
                  .sort((a, b) => (b.correct ? 1 : 0) - (a.correct ? 1 : 0))
                  .map((e, i) => (
                    <li key={i} className="flex items-center justify-between px-5 py-3">
                      <div className="flex items-center gap-3">
                        <span className={`text-base ${e.correct ? 'text-emerald-500' : 'text-slate-300'}`}>
                          {e.correct ? '✓' : '✗'}
                        </span>
                        <span className="text-sm font-medium text-slate-700">{e.display_name}</span>
                        <span className="text-sm text-slate-400">"{e.answer}"</span>
                      </div>
                      {e.correct && (
                        <span className="text-xs font-semibold text-emerald-600">+{e.points}</span>
                      )}
                    </li>
                  ))}
              </ul>
            )}
          </div>

          <div className="flex justify-end">
            <button
              onClick={advanceQuestion}
              className="rounded-lg bg-brand-blue px-6 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
            >
              {question.index + 1 >= question.total
                ? 'Show Final Results'
                : (question.index + 1) % question.round_size === 0
                  ? 'Show Leaderboard'
                  : 'Next Question →'}
            </button>
          </div>
        </div>
      )}

      {/* ---- LEADERBOARD (mid-round) ---- */}
      {phase === 'leaderboard' && (
        <div className="space-y-5">
          <LeaderboardView entries={leaderboard} title="Round Leaderboard" isFinal={false} />
          <div className="flex justify-center">
            <button
              onClick={advanceQuestion}
              className="rounded-lg bg-brand-blue px-8 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
            >
              Continue →
            </button>
          </div>
        </div>
      )}

      {/* ---- GAME ENDED ---- */}
      {phase === 'ended' && (
        <LeaderboardView entries={leaderboard} title="Final Results" isFinal={true} />
      )}

    </main>
  )
}

// ---- Sub-components --------------------------------------------------------

function RoundProgress({
  index, total, round, posInRound, roundSize,
}: {
  index: number; total: number; round: number
  posInRound: number; roundSize: number
}) {
  return (
    <div className="flex items-center justify-between text-sm text-slate-500">
      <span>Round {round} · Question {posInRound}/{Math.min(roundSize, total - (round - 1) * roundSize)}</span>
      <span className="text-xs text-slate-400">{index + 1} of {total} total</span>
    </div>
  )
}

function LeaderboardView({
  entries, title, isFinal,
}: {
  entries: LeaderboardEntry[]
  title: string
  isFinal: boolean
}) {
  const medals = ['🥇', '🥈', '🥉']

  return (
    <div className="overflow-hidden rounded-xl border border-gray-100 bg-white shadow-sm">
      <div className={`h-[3px] ${isFinal ? 'bg-brand-red' : 'bg-brand-blue'}`} />
      <div className="border-b border-gray-100 px-6 py-4">
        <p className="text-base font-semibold text-slate-800">{title}</p>
      </div>
      {entries.length === 0 ? (
        <p className="px-6 py-6 text-sm text-slate-400">No scores yet.</p>
      ) : (
        <ul className="divide-y divide-gray-50">
          {entries.map((e) => (
            <li key={e.rank} className="flex items-center gap-4 px-6 py-3">
              <span className="w-8 text-center text-lg">
                {e.rank <= 3 ? medals[e.rank - 1] : `#${e.rank}`}
              </span>
              <span className="flex-1 text-sm font-medium text-slate-700">{e.display_name}</span>
              <span className="text-sm font-semibold text-slate-800">{e.score.toLocaleString()}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
