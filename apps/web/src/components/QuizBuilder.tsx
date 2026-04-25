'use client'

import { useState, useTransition, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import type { QuizDetail, QuizRound, Bank, Question } from '@/types'
import {
  createRoundAction,
  deleteRoundAction,
  setRoundQuestionsAction,
  createGameFromQuizAction,
  listBankQuestionsAction,
} from '@/app/(host)/quizzes/actions'

interface Props {
  quiz: QuizDetail
  banks: Bank[]
}

export default function QuizBuilder({ quiz: initialQuiz, banks }: Props) {
  const router = useRouter()
  const [quiz, setQuiz] = useState<QuizDetail>(initialQuiz)
  // Sync local state whenever the server re-renders with fresh quiz data
  // (e.g. after router.refresh() following a round add/delete).
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { setQuiz(initialQuiz) }, [initialQuiz])
  const [isPending, startTransition] = useTransition()
  const [error, setError] = useState('')
  const [launchPending, startLaunch] = useTransition()

  // Which round has the question picker open.
  const [pickerRoundID, setPickerRoundID] = useState<string | null>(null)
  // Which bank is shown in the picker.
  const [pickerBankID, setPickerBankID] = useState<string>(banks[0]?.id ?? '')

  function handleAddRound() {
    startTransition(async () => {
      const res = await createRoundAction(quiz.id)
      if (res?.error) { setError(res.error); return }
      // Refresh the page to get updated round data.
      router.refresh()
    })
  }

  function handleDeleteRound(roundID: string) {
    if (!confirm('Remove this round?')) return
    startTransition(async () => {
      const res = await deleteRoundAction(quiz.id, roundID)
      if (res?.error) { setError(res.error); return }
      router.refresh()
    })
  }

  function handleLaunchGame() {
    startLaunch(async () => {
      const res = await createGameFromQuizAction(quiz.id)
      if (res?.error) { setError(res.error) }
      // redirect happens inside action on success
    })
  }

  const pickerRound = quiz.rounds.find(r => r.id === pickerRoundID)

  return (
    <main className="mx-auto max-w-4xl px-6 py-10">
      {/* Header */}
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <button
            onClick={() => router.push('/quizzes')}
            className="mb-2 text-xs text-slate-400 hover:text-slate-600"
          >
            ← All Quizzes
          </button>
          <h1 className="text-2xl font-bold text-slate-900">{quiz.name}</h1>
          {quiz.description && (
            <p className="mt-1 text-sm text-slate-500">{quiz.description}</p>
          )}
        </div>
        <button
          onClick={handleLaunchGame}
          disabled={launchPending || quiz.rounds.length === 0}
          className="rounded-lg bg-green-600 px-4 py-2 text-sm font-display font-semibold text-white hover:bg-green-700 disabled:opacity-40 transition-colors"
        >
          {launchPending ? 'Creating game…' : '▶ Launch Game'}
        </button>
      </div>

      {error && (
        <p className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-brand-red">{error}</p>
      )}

      {/* Rounds */}
      <div className="space-y-4">
        {quiz.rounds.length === 0 && (
          <div className="rounded-xl border border-dashed border-gray-300 bg-white py-12 text-center">
            <p className="text-slate-400">No rounds yet. Add a round to get started.</p>
          </div>
        )}

        {quiz.rounds.map((round, idx) => (
          <RoundCard
            key={round.id}
            round={round}
            roundNumber={idx + 1}
            quizID={quiz.id}
            onDelete={() => handleDeleteRound(round.id)}
            onOpenPicker={() => setPickerRoundID(round.id)}
            onRefresh={() => router.refresh()}
          />
        ))}
      </div>

      <button
        onClick={handleAddRound}
        disabled={isPending}
        className="mt-4 w-full rounded-xl border-2 border-dashed border-gray-300 py-3 text-sm text-slate-500 hover:border-brand-blue/40 hover:text-brand-blue disabled:opacity-40 transition-colors"
      >
        {isPending ? 'Adding round…' : '+ Add Round'}
      </button>

      {/* Question Picker Modal */}
      {pickerRoundID && pickerRound && (
        <QuestionPickerModal
          quizID={quiz.id}
          round={pickerRound}
          banks={banks}
          selectedBankID={pickerBankID}
          onBankChange={setPickerBankID}
          onClose={() => setPickerRoundID(null)}
          onSaved={() => { setPickerRoundID(null); router.refresh() }}
        />
      )}
    </main>
  )
}

// ---- RoundCard ---------------------------------------------------------------

function RoundCard({
  round, roundNumber, quizID, onDelete, onOpenPicker, onRefresh,
}: {
  round: QuizRound
  roundNumber: number
  quizID: string
  onDelete: () => void
  onOpenPicker: () => void
  onRefresh: () => void
}) {
  const [removing, startRemove] = useTransition()

  function handleRemoveQuestion(questionID: string) {
    const newIDs = round.questions.filter(q => q.id !== questionID).map(q => q.id)
    startRemove(async () => {
      const res = await setRoundQuestionsAction(quizID, round.id, newIDs)
      if (!res?.error) onRefresh()
    })
  }

  function handleMoveQuestion(questionID: string, direction: -1 | 1) {
    const ids = round.questions.map(q => q.id)
    const idx = ids.indexOf(questionID)
    if (idx < 0) return
    const newIdx = idx + direction
    if (newIdx < 0 || newIdx >= ids.length) return
    ;[ids[idx], ids[newIdx]] = [ids[newIdx], ids[idx]]
    startRemove(async () => {
      const res = await setRoundQuestionsAction(quizID, round.id, ids)
      if (!res?.error) onRefresh()
    })
  }

  return (
    <div className="rounded-xl border border-gray-100 bg-white shadow-sm overflow-hidden">
      {/* Round header */}
      <div className="flex items-center justify-between px-5 py-3 border-b border-gray-100 bg-slate-50">
        <div>
          <span className="text-xs font-medium uppercase tracking-wide text-slate-400">
            Round {roundNumber}
          </span>
          {round.title && (
            <span className="ml-2 text-sm font-semibold text-slate-700">{round.title}</span>
          )}
          <span className="ml-2 text-xs text-slate-400">
            ({round.questions.length} question{round.questions.length !== 1 ? 's' : ''})
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onOpenPicker}
            className="rounded-lg bg-brand-blue px-3 py-1.5 text-xs font-semibold text-white hover:bg-brand-blue/90 transition-colors"
          >
            + Add Questions
          </button>
          <button
            onClick={onDelete}
            className="rounded-lg border border-gray-200 px-2 py-1.5 text-xs text-slate-500 hover:border-brand-red/40 hover:text-brand-red transition-colors"
          >
            Remove Round
          </button>
        </div>
      </div>

      {/* Questions list */}
      {round.questions.length === 0 ? (
        <p className="px-5 py-4 text-sm text-slate-400 italic">
          No questions yet — click &quot;Add Questions&quot; to pick from your banks.
        </p>
      ) : (
        <ul className="divide-y divide-gray-50">
          {round.questions.map((q, i) => (
            <li key={q.id} className="flex items-start gap-3 px-5 py-3">
              <span className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-semibold text-slate-500">
                {i + 1}
              </span>
              <div className="flex-1 min-w-0">
                <p className="text-sm text-slate-700 truncate">{q.prompt}</p>
                <p className="text-xs text-slate-400">
                  {q.type === 'multiple_choice' ? 'Multiple choice' : 'Text answer'}
                  {' · '}{q.points} pts
                </p>
              </div>
              <div className="flex items-center gap-1 shrink-0">
                <button
                  onClick={() => handleMoveQuestion(q.id, -1)}
                  disabled={i === 0 || removing}
                  title="Move up"
                  className="rounded p-1 text-slate-400 hover:text-slate-600 disabled:opacity-20"
                >
                  ↑
                </button>
                <button
                  onClick={() => handleMoveQuestion(q.id, 1)}
                  disabled={i === round.questions.length - 1 || removing}
                  title="Move down"
                  className="rounded p-1 text-slate-400 hover:text-slate-600 disabled:opacity-20"
                >
                  ↓
                </button>
                <button
                  onClick={() => handleRemoveQuestion(q.id)}
                  disabled={removing}
                  title="Remove"
                  className="rounded p-1 text-slate-400 hover:text-brand-red disabled:opacity-20"
                >
                  ×
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

// ---- QuestionPickerModal -----------------------------------------------------

interface PickerProps {
  quizID: string
  round: QuizRound
  banks: Bank[]
  selectedBankID: string
  onBankChange: (id: string) => void
  onClose: () => void
  onSaved: () => void
}

function QuestionPickerModal({
  quizID, round, banks, selectedBankID, onBankChange, onClose, onSaved,
}: PickerProps) {
  const [bankQuestions, setBankQuestions] = useState<Question[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, startSave] = useTransition()

  const [stagedIDs, setStagedIDs] = useState<string[]>(round.questions.map(q => q.id))

  // Load questions via server action so auth token stays server-side.
  async function loadBank(bankID: string) {
    onBankChange(bankID)
    setLoading(true)
    const qs = await listBankQuestionsAction(bankID)
    setBankQuestions(qs)
    setLoading(false)
  }

  // Load initial bank on mount.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (selectedBankID) loadBank(selectedBankID)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function toggleQuestion(id: string) {
    setStagedIDs(prev =>
      prev.includes(id) ? prev.filter(x => x !== id) : [...prev, id]
    )
  }

  function handleSave() {
    startSave(async () => {
      const res = await setRoundQuestionsAction(quizID, round.id, stagedIDs)
      if (!res?.error) onSaved()
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-2xl rounded-2xl bg-white shadow-xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <p className="font-semibold text-slate-800">
            Add Questions — Round {round.round_number}
          </p>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-600 text-xl leading-none"
          >
            ×
          </button>
        </div>

        {/* Bank selector */}
        {banks.length > 1 && (
          <div className="px-6 py-3 border-b border-gray-100 bg-slate-50">
            <label className="text-xs font-medium text-slate-500 uppercase tracking-wide">
              Question Bank
            </label>
            <select
              value={selectedBankID}
              onChange={e => loadBank(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-slate-700 focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
            >
              {banks.map(b => (
                <option key={b.id} value={b.id}>{b.name}</option>
              ))}
            </select>
          </div>
        )}

        {/* Question list */}
        <div className="flex-1 overflow-y-auto px-6 py-4">
          {loading ? (
            <p className="text-sm text-slate-400 text-center py-8">Loading…</p>
          ) : bankQuestions.length === 0 ? (
            <p className="text-sm text-slate-400 text-center py-8">
              No questions in this bank yet.
            </p>
          ) : (
            <ul className="space-y-2">
              {bankQuestions.map(q => {
                const selected = stagedIDs.includes(q.id)
                return (
                  <li
                    key={q.id}
                    onClick={() => toggleQuestion(q.id)}
                    className={`flex items-start gap-3 rounded-xl border px-4 py-3 cursor-pointer transition-all ${
                      selected
                        ? 'border-brand-blue/40 bg-brand-blue/5'
                        : 'border-gray-100 bg-white hover:border-gray-200'
                    }`}
                  >
                    <span className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded border-2 text-xs font-bold ${
                      selected
                        ? 'border-brand-blue bg-brand-blue text-white'
                        : 'border-gray-300'
                    }`}>
                      {selected ? stagedIDs.indexOf(q.id) + 1 : ''}
                    </span>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-slate-700">{q.prompt}</p>
                      <p className="text-xs text-slate-400 mt-0.5">
                        {q.type === 'multiple_choice' ? 'Multiple choice' : 'Text answer'}
                        {' · '}{q.points} pts
                      </p>
                    </div>
                  </li>
                )
              })}
            </ul>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between gap-3 px-6 py-4 border-t border-gray-100 bg-slate-50">
          <p className="text-sm text-slate-500">
            {stagedIDs.length} question{stagedIDs.length !== 1 ? 's' : ''} selected
          </p>
          <div className="flex gap-3">
            <button
              onClick={onClose}
              className="rounded-lg border border-gray-200 px-4 py-2 text-sm text-slate-600 hover:bg-slate-100"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={saving}
              className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-semibold text-white hover:bg-brand-blue/90 disabled:opacity-40 transition-colors"
            >
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
