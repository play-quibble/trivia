'use client'

import { useEffect, useRef, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import type {
  QuestionRevealedPayload,
  AnswersRevealedPayload,
  LeaderboardPayload,
  LeaderboardEntry,
} from '@/types'

// ---- types ----------------------------------------------------------------

type Phase =
  | 'connecting'
  | 'lobby'
  | 'question'
  | 'answered'    // submitted — waiting for host to reveal
  | 'answers'     // host revealed
  | 'leaderboard'
  | 'ended'
  | 'error'
  | 'no-session'  // landed here without going through /join

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
  code: string
  wsBase: string
}

export default function PlayerGame({ code, wsBase }: Props) {
  const router = useRouter()
  const wsRef = useRef<WebSocket | null>(null)
  const [phase, setPhase] = useState<Phase>('connecting')
  const [question, setQuestion] = useState<QuestionState | null>(null)
  const [textAnswer, setTextAnswer] = useState('')
  const [submittedAnswer, setSubmittedAnswer] = useState('')
  const [wasCorrect, setWasCorrect] = useState<boolean | null>(null)
  const [correctAnswers, setCorrectAnswers] = useState<string[]>([])
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([])
  const [isFinal, setIsFinal] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  const handleMessage = useCallback((raw: string) => {
    let msg: { type: string; payload?: unknown }
    try { msg = JSON.parse(raw) } catch { return }

    switch (msg.type) {
      case 'game_started':
        setPhase('question')
        break

      case 'question_revealed': {
        const p = msg.payload as QuestionRevealedPayload
        setQuestion(p)
        setPhase('question')
        setTextAnswer('')
        setSubmittedAnswer('')
        setWasCorrect(null)
        setCorrectAnswers([])
        break
      }

      case 'answer_accepted': {
        const p = msg.payload as { correct: boolean }
        setWasCorrect(p.correct)
        setPhase('answered')
        break
      }

      case 'answers_revealed': {
        const p = msg.payload as AnswersRevealedPayload
        setCorrectAnswers(p.correct_answers)
        setPhase('answers')
        break
      }

      case 'round_leaderboard': {
        const p = msg.payload as LeaderboardPayload
        setLeaderboard(p.entries)
        setIsFinal(false)
        setPhase('leaderboard')
        break
      }

      case 'game_ended': {
        const p = msg.payload as LeaderboardPayload
        setLeaderboard(p.entries)
        setIsFinal(true)
        setPhase('ended')
        break
      }
    }
  }, [])

  useEffect(() => {
    const sessionToken = sessionStorage.getItem(`quibble_session_${code}`)
    if (!sessionToken) {
      setPhase('no-session')
      return
    }

    const url = `${wsBase}/ws/${code}?session=${encodeURIComponent(sessionToken)}`
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => setPhase('lobby')
    ws.onmessage = (e) => handleMessage(e.data)
    ws.onerror = () => {
      setErrorMsg('Connection failed — the game may have ended or the code is wrong.')
      setPhase('error')
    }

    return () => ws.close()
  }, [wsBase, code, handleMessage])

  // ---- submit answer -------------------------------------------------------

  function submitText(e: React.FormEvent) {
    e.preventDefault()
    if (!textAnswer.trim()) return
    setSubmittedAnswer(textAnswer.trim())
    send(wsRef.current, 'submit_answer', { answer: textAnswer.trim() })
  }

  function submitChoice(choice: string) {
    setSubmittedAnswer(choice)
    send(wsRef.current, 'submit_answer', { answer: choice })
  }

  // ---- render --------------------------------------------------------------

  const medals = ['🥇', '🥈', '🥉']

  return (
    <div className="flex min-h-screen flex-col items-center bg-slate-100 px-4 pt-10">
      <div className="w-full max-w-md">

        {/* Logo strip */}
        <p className="mb-6 text-center font-display text-2xl font-bold text-brand-blue">
          Qui<span className="italic">bb</span>le
        </p>

        {/* ---- NO SESSION ---- */}
        {phase === 'no-session' && (
          <Card accent="red">
            <p className="mb-4 text-sm text-slate-600">
              You need to join this game first.
            </p>
            <button
              onClick={() => router.push(`/join?code=${code}`)}
              className="w-full rounded-lg bg-brand-blue py-2.5 text-sm font-semibold text-white hover:bg-brand-blue/90"
            >
              Go to Join Page
            </button>
          </Card>
        )}

        {/* ---- CONNECTING ---- */}
        {phase === 'connecting' && (
          <Card>
            <p className="text-center text-slate-400">Connecting…</p>
          </Card>
        )}

        {/* ---- ERROR ---- */}
        {phase === 'error' && (
          <Card accent="red">
            <p className="text-sm text-brand-red">{errorMsg}</p>
            <button
              onClick={() => router.push('/join')}
              className="mt-4 w-full rounded-lg border border-gray-200 py-2 text-sm text-slate-600 hover:bg-slate-50"
            >
              Back to Join
            </button>
          </Card>
        )}

        {/* ---- LOBBY ---- */}
        {phase === 'lobby' && (
          <Card>
            <div className="py-8 text-center">
              <div className="mb-3 text-5xl">⏳</div>
              <p className="text-base font-semibold text-slate-700">You&apos;re in!</p>
              <p className="mt-1 text-sm text-slate-500">Waiting for the host to start the game…</p>
              <p className="mt-4 font-display text-3xl font-bold tracking-widest text-brand-blue">{code}</p>
            </div>
          </Card>
        )}

        {/* ---- QUESTION ---- */}
        {phase === 'question' && question && (
          <div className="space-y-4">
            <div className="flex items-center justify-between text-xs text-slate-500">
              <span>Round {question.round} · Q {question.pos_in_round}</span>
              <span>{question.index + 1}/{question.total}</span>
            </div>

            <Card>
              <p className="mb-1 text-xs font-medium uppercase tracking-wide text-slate-400">
                {question.question.type === 'multiple_choice' ? 'Multiple Choice' : 'Text Answer'}
                {' · '}{question.question.points} pts
              </p>
              <p className="text-lg font-semibold leading-snug text-slate-800">
                {question.question.prompt}
              </p>
            </Card>

            {/* Text answer input */}
            {question.question.type === 'text' && (
              <form onSubmit={submitText} className="space-y-3">
                <input
                  type="text"
                  value={textAnswer}
                  onChange={(e) => setTextAnswer(e.target.value)}
                  placeholder="Type your answer…"
                  autoFocus
                  className="w-full rounded-xl border border-gray-200 bg-white px-4 py-3 text-sm text-slate-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
                />
                <button
                  type="submit"
                  disabled={!textAnswer.trim()}
                  className="w-full rounded-xl bg-brand-blue py-3 text-sm font-semibold text-white hover:bg-brand-blue/90 disabled:opacity-40 transition-colors"
                >
                  Submit Answer
                </button>
              </form>
            )}

            {/* MC choice buttons */}
            {question.question.type === 'multiple_choice' && question.question.choices && (
              <div className="space-y-2">
                {question.question.choices.map((c, i) => (
                  <button
                    key={i}
                    onClick={() => submitChoice(c)}
                    className="w-full rounded-xl border border-gray-200 bg-white px-5 py-3 text-left text-sm font-medium text-slate-700 shadow-sm hover:border-brand-blue/40 hover:bg-brand-blue/5 active:scale-[0.99] transition-all"
                  >
                    {c}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {/* ---- ANSWERED (waiting for reveal) ---- */}
        {phase === 'answered' && (
          <Card>
            <div className="py-6 text-center">
              <div className="mb-3 text-5xl">
                {wasCorrect === null ? '📨' : wasCorrect ? '✅' : '❌'}
              </div>
              <p className="text-base font-semibold text-slate-700">
                {wasCorrect === null
                  ? 'Answer submitted!'
                  : wasCorrect
                    ? 'Correct!'
                    : 'Not quite!'}
              </p>
              <p className="mt-1 text-sm text-slate-500">
                You answered: <span className="font-medium text-slate-700">"{submittedAnswer}"</span>
              </p>
              <p className="mt-3 text-xs text-slate-400">Waiting for host to reveal answers…</p>
            </div>
          </Card>
        )}

        {/* ---- ANSWERS REVEALED ---- */}
        {phase === 'answers' && (
          <Card>
            <p className="mb-3 text-xs font-medium uppercase tracking-wide text-slate-500">
              Correct Answer{correctAnswers.length > 1 ? 's' : ''}
            </p>
            <p className="mb-4 text-lg font-semibold text-emerald-700">
              {correctAnswers.join(' · ')}
            </p>
            <div className={`rounded-lg px-4 py-3 text-sm font-medium ${
              wasCorrect ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-50 text-slate-500'
            }`}>
              {wasCorrect
                ? `You got it right! ✓`
                : submittedAnswer
                  ? `You answered: "${submittedAnswer}"`
                  : 'You did not answer in time.'}
            </div>
            <p className="mt-3 text-center text-xs text-slate-400">
              Waiting for host…
            </p>
          </Card>
        )}

        {/* ---- LEADERBOARD ---- */}
        {(phase === 'leaderboard' || phase === 'ended') && (
          <Card>
            <p className="mb-4 text-base font-semibold text-slate-800">
              {isFinal ? '🏁 Final Results' : '📊 Leaderboard'}
            </p>
            {leaderboard.length === 0 ? (
              <p className="text-sm text-slate-400">No scores yet.</p>
            ) : (
              <ul className="space-y-2">
                {leaderboard.map((e) => (
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
            {isFinal && (
              <button
                onClick={() => router.push('/join')}
                className="mt-6 w-full rounded-lg border border-gray-200 py-2.5 text-sm text-slate-600 hover:bg-slate-50"
              >
                Play Again
              </button>
            )}
            {!isFinal && (
              <p className="mt-4 text-center text-xs text-slate-400">Waiting for host…</p>
            )}
          </Card>
        )}

      </div>
    </div>
  )
}

// ---- Card helper -----------------------------------------------------------

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
