'use client'

import { useEffect, useRef, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import type {
  QuestionReleasedPayload,
  RoundEndedPayload,
  RoundScoresPayload,
  RoundScoreQuestion,
  LeaderboardPayload,
  LeaderboardEntry,
  AnswerAcceptedPayload,
  GameStartedPayload,
} from '@/types'

// ---- types ----------------------------------------------------------------

type Phase =
  | 'connecting'
  | 'lobby'
  | 'question'      // questions appearing one at a time; player can answer each
  | 'round_ended'   // round over; waiting for host review
  | 'round_scores'  // host released scores — player sees their results
  | 'leaderboard'   // between-round scoreboard
  | 'ended'         // final scoreboard
  | 'error'
  | 'no-session'

interface ActiveQuestion {
  id: string
  type: 'text' | 'multiple_choice'
  prompt: string
  points: number
  choices?: string[]
  posInRound: number
  submittedAnswer?: string
  correct?: boolean
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
  const [errorMsg, setErrorMsg] = useState('')

  // Round state
  const [currentRound, setCurrentRound] = useState(1)
  const [totalRounds, setTotalRounds] = useState(1)
  const [activeQuestions, setActiveQuestions] = useState<ActiveQuestion[]>([])
  const [textInputs, setTextInputs] = useState<Record<string, string>>({})

  // Round results
  const [roundResults, setRoundResults] = useState<RoundScoreQuestion[]>([])
  const [roundScore, setRoundScore] = useState(0)

  // Leaderboard
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([])
  const [isFinal, setIsFinal] = useState(false)

  // Keep a stable ref to the latest textInputs so handleMessage can read the
  // current value without being listed as a dependency. This prevents the
  // WebSocket effect from re-running (and tearing down the connection) every
  // time the player types a character.
  const textInputsRef = useRef(textInputs)
  useEffect(() => { textInputsRef.current = textInputs }, [textInputs])

  const handleMessage = useCallback((raw: string) => {
    let msg: { type: string; payload?: unknown }
    try { msg = JSON.parse(raw) } catch { return }

    switch (msg.type) {
      case 'game_started': {
        const p = msg.payload as GameStartedPayload
        setCurrentRound(p.round ?? 1)
        setTotalRounds(p.total_rounds ?? 1)
        setActiveQuestions([])
        setTextInputs({})
        setPhase('question')
        break
      }

      case 'question_released': {
        const p = msg.payload as QuestionReleasedPayload
        setCurrentRound(p.round)
        setTotalRounds(p.total_rounds)
        setPhase('question')
        setActiveQuestions(prev => {
          if (prev.find(q => q.id === p.question.id)) return prev
          return [...prev, {
            id: p.question.id,
            type: p.question.type,
            prompt: p.question.prompt,
            points: p.question.points,
            choices: p.question.choices,
            posInRound: p.pos_in_round,
          }]
        })
        break
      }

      case 'answer_accepted': {
        const p = msg.payload as AnswerAcceptedPayload
        if (p.question_id) {
          // Don't apply the server's instant correct/incorrect verdict here — the host
          // reviews answers manually before scores are released. Keeping `correct`
          // undefined shows the neutral "submitted" state until round_scores arrives.
          setActiveQuestions(prev =>
            prev.map(q => q.id === p.question_id
              ? { ...q, submittedAnswer: textInputsRef.current[q.id] ?? q.submittedAnswer }
              : q
            )
          )
        }
        break
      }

      case 'round_ended': {
        const p = msg.payload as RoundEndedPayload
        setCurrentRound(p.round)
        setTotalRounds(p.total_rounds)
        setPhase('round_ended')
        break
      }

      case 'round_scores': {
        const p = msg.payload as RoundScoresPayload
        setCurrentRound(p.round)
        setTotalRounds(p.total_rounds)
        setRoundResults(p.questions ?? [])
        setRoundScore(p.round_score ?? 0)
        setPhase('round_scores')
        break
      }

      case 'round_leaderboard': {
        const p = msg.payload as LeaderboardPayload
        setLeaderboard(p.entries ?? [])
        setCurrentRound(p.round)
        setTotalRounds(p.total_rounds)
        setIsFinal(false)
        setPhase('leaderboard')
        break
      }

      case 'game_ended': {
        const p = msg.payload as LeaderboardPayload
        setLeaderboard(p.entries ?? [])
        setIsFinal(true)
        setPhase('ended')
        break
      }
    }
  }, []) // stable — reads textInputs via ref, so no dependency needed

  // Keep a ref to the latest handleMessage so the WebSocket effect never needs
  // to re-run just because the handler was recreated.
  const handleMessageRef = useRef(handleMessage)
  useEffect(() => { handleMessageRef.current = handleMessage }, [handleMessage])

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
    ws.onmessage = (e) => handleMessageRef.current(e.data)
    ws.onerror = () => {
      setErrorMsg('Connection failed — the game may have ended or the code is wrong.')
      setPhase('error')
    }

    return () => ws.close()
  }, [wsBase, code]) // WebSocket only reconnects if the game code or server URL changes

  // ---- submit helpers -----------------------------------------------------

  function submitText(questionID: string, e: React.FormEvent) {
    e.preventDefault()
    const answer = (textInputs[questionID] ?? '').trim()
    if (!answer) return
    // Mark locally submitted
    setActiveQuestions(prev =>
      prev.map(q => q.id === questionID ? { ...q, submittedAnswer: answer } : q)
    )
    send(wsRef.current, 'submit_answer', { question_id: questionID, answer })
  }

  function submitChoice(questionID: string, choice: string) {
    setActiveQuestions(prev =>
      prev.map(q => q.id === questionID ? { ...q, submittedAnswer: choice } : q)
    )
    send(wsRef.current, 'submit_answer', { question_id: questionID, answer: choice })
  }

  // ---- render -------------------------------------------------------------

  const medals = ['🥇', '🥈', '🥉']

  return (
    <div className="flex min-h-screen flex-col items-center bg-slate-100 px-4 pt-10 pb-20">
      <div className="w-full max-w-md space-y-4">

        {/* Logo */}
        <p className="mb-2 text-center font-display text-2xl font-bold text-brand-blue">
          Qui<span className="italic">bb</span>le
        </p>

        {/* ---- NO SESSION ---- */}
        {phase === 'no-session' && (
          <Card accent="red">
            <p className="mb-4 text-sm text-slate-600">You need to join this game first.</p>
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
          <Card><p className="text-center text-slate-400">Connecting…</p></Card>
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
              <p className="mt-1 text-sm text-slate-500">Waiting for the host to start…</p>
              <p className="mt-4 font-display text-3xl font-bold tracking-widest text-brand-blue">{code}</p>
            </div>
          </Card>
        )}

        {/* ---- ACTIVE QUESTIONS (accumulating during round) ---- */}
        {phase === 'question' && activeQuestions.length > 0 && (
          <div className="space-y-3">
            <p className="text-xs font-medium uppercase tracking-wide text-slate-400">
              Round {currentRound}{totalRounds > 1 ? ` of ${totalRounds}` : ''}
            </p>
            {activeQuestions.map(q => (
              <QuestionCard
                key={q.id}
                question={q}
                textValue={textInputs[q.id] ?? ''}
                onTextChange={(val) => setTextInputs(prev => ({ ...prev, [q.id]: val }))}
                onSubmitText={(e) => submitText(q.id, e)}
                onSubmitChoice={(choice) => submitChoice(q.id, choice)}
              />
            ))}
            <p className="text-center text-xs text-slate-400">
              Waiting for the host to release more questions…
            </p>
          </div>
        )}

        {/* ---- WAITING FOR FIRST QUESTION ---- */}
        {phase === 'question' && activeQuestions.length === 0 && (
          <Card>
            <p className="text-center text-sm text-slate-400 py-4">
              Round {currentRound} is starting — get ready!
            </p>
          </Card>
        )}

        {/* ---- ROUND ENDED (waiting for host review) ---- */}
        {phase === 'round_ended' && (
          <Card>
            <div className="py-6 text-center">
              <div className="mb-3 text-5xl">⏸</div>
              <p className="text-base font-semibold text-slate-700">Round {currentRound} complete!</p>
              <p className="mt-1 text-sm text-slate-500">The host is reviewing answers…</p>
            </div>
          </Card>
        )}

        {/* ---- ROUND SCORES ---- */}
        {phase === 'round_scores' && (
          <div className="space-y-3">
            <Card>
              <div className="text-center mb-4">
                <p className="text-base font-semibold text-slate-800">
                  Round {currentRound} Results
                </p>
                <p className="mt-1 text-2xl font-bold text-brand-blue">
                  +{roundScore.toLocaleString()} pts
                </p>
              </div>
              <div className="space-y-3">
                {roundResults.map(qr => (
                  <div
                    key={qr.question_id}
                    className={`rounded-xl px-4 py-3 ${
                      qr.correct ? 'bg-emerald-50 border border-emerald-100' : 'bg-slate-50 border border-gray-100'
                    }`}
                  >
                    <p className="text-sm font-medium text-slate-700 mb-1">{qr.prompt}</p>
                    <p className="text-xs text-slate-500">
                      Correct: <span className="font-medium text-slate-700">{(qr.correct_answers ?? []).join(' · ')}</span>
                    </p>
                    {qr.your_answer && (
                      <p className={`mt-1 text-xs font-medium ${qr.correct ? 'text-emerald-700' : 'text-slate-400'}`}>
                        Your answer: &ldquo;{qr.your_answer}&rdquo;
                        {qr.correct ? ' ✓' : ''}
                        {qr.correct && qr.points_earned ? ` (+${qr.points_earned} pts)` : ''}
                      </p>
                    )}
                    {!qr.your_answer && (
                      <p className="mt-1 text-xs text-slate-400">Not answered</p>
                    )}
                  </div>
                ))}
              </div>
              <p className="mt-4 text-center text-xs text-slate-400">Waiting for leaderboard…</p>
            </Card>
          </div>
        )}

        {/* ---- LEADERBOARD ---- */}
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
              <ul className="space-y-2">
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
            {isFinal && (
              <button
                onClick={() => router.push('/join')}
                className="mt-6 w-full rounded-lg border border-gray-200 py-2.5 text-sm text-slate-600 hover:bg-slate-50"
              >
                Play Again
              </button>
            )}
            {!isFinal && (
              <p className="mt-4 text-center text-xs text-slate-400">
                Waiting for the next round…
              </p>
            )}
          </Card>
        )}

      </div>
    </div>
  )
}

// ---- QuestionCard -----------------------------------------------------------

function QuestionCard({
  question,
  textValue,
  onTextChange,
  onSubmitText,
  onSubmitChoice,
}: {
  question: ActiveQuestion
  textValue: string
  onTextChange: (val: string) => void
  onSubmitText: (e: React.FormEvent) => void
  onSubmitChoice: (choice: string) => void
}) {
  const submitted = !!question.submittedAnswer

  return (
    <div className="overflow-hidden rounded-2xl border border-gray-100 bg-white shadow-sm">
      <div className={`h-[3px] ${submitted
        ? (question.correct === true ? 'bg-emerald-500' : question.correct === false ? 'bg-brand-red' : 'bg-slate-300')
        : 'bg-brand-blue'
      }`} />
      <div className="p-4">
        <div className="mb-2 flex items-center justify-between">
          <span className="text-xs font-medium uppercase tracking-wide text-slate-400">
            Q{question.posInRound} · {question.type === 'multiple_choice' ? 'Multiple Choice' : 'Text Answer'} · {question.points} pts
          </span>
          {submitted && (
            <span className={`text-xs font-medium ${
              question.correct === true ? 'text-emerald-600' :
              question.correct === false ? 'text-brand-red' :
              'text-slate-400'
            }`}>
              {question.correct === true ? '✓ Correct' :
               question.correct === false ? '✗ Incorrect' :
               '✓ Submitted'}
            </span>
          )}
        </div>

        <p className="mb-3 text-sm font-semibold leading-snug text-slate-800">
          {question.prompt}
        </p>

        {/* Already submitted — show their answer */}
        {submitted && (
          <div className={`rounded-lg px-3 py-2 text-sm ${
            question.correct === true ? 'bg-emerald-50 text-emerald-700' :
            question.correct === false ? 'bg-red-50 text-brand-red' :
            'bg-slate-100 text-slate-500'
          }`}>
            Your answer: &ldquo;{question.submittedAnswer}&rdquo;
          </div>
        )}

        {/* Text input (not yet submitted) */}
        {!submitted && question.type === 'text' && (
          <form onSubmit={onSubmitText} className="flex gap-2">
            <input
              type="text"
              value={textValue}
              onChange={e => onTextChange(e.target.value)}
              placeholder="Type your answer…"
              className="flex-1 rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-sm text-slate-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
            />
            <button
              type="submit"
              disabled={!textValue.trim()}
              className="rounded-xl bg-brand-blue px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-blue/90 disabled:opacity-40 transition-colors"
            >
              Submit
            </button>
          </form>
        )}

        {/* MC choices (not yet submitted) */}
        {!submitted && question.type === 'multiple_choice' && question.choices && (
          <div className="space-y-1.5">
            {question.choices.map((c, i) => (
              <button
                key={i}
                onClick={() => onSubmitChoice(c)}
                className="w-full rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-left text-sm font-medium text-slate-700 shadow-sm hover:border-brand-blue/40 hover:bg-brand-blue/5 active:scale-[0.99] transition-all"
              >
                {c}
              </button>
            ))}
          </div>
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
