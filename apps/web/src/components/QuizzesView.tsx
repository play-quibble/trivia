'use client'

import { useState, useTransition } from 'react'
import { useRouter } from 'next/navigation'
import type { Quiz } from '@/types'
import { createQuizAction, deleteQuizAction, createGameFromQuizAction } from '@/app/(host)/quizzes/actions'

interface Props {
  quizzes: Quiz[]
}

export default function QuizzesView({ quizzes: initial }: Props) {
  const router = useRouter()
  const [quizzes, setQuizzes] = useState<Quiz[]>(initial)
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [isPending, startTransition] = useTransition()
  const [error, setError] = useState('')

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) return
    const fd = new FormData()
    fd.set('name', name.trim())
    startTransition(async () => {
      const result = await createQuizAction(fd)
      if (result?.error) {
        setError(result.error)
      }
      // redirect happens server-side on success
    })
  }

  return (
    <main className="mx-auto max-w-5xl px-6 py-10">
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Quizzes</h1>
          <p className="mt-1 text-sm text-slate-500">
            Build multi-round quizzes by choosing questions from your banks.
          </p>
        </div>
        <button
          onClick={() => { setShowCreate(true); setError('') }}
          className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
        >
          + New Quiz
        </button>
      </div>

      {showCreate && (
        <form
          onSubmit={handleCreate}
          className="mb-6 rounded-xl border border-gray-200 bg-white p-5 shadow-sm"
        >
          <p className="mb-3 text-sm font-semibold text-slate-700">New Quiz</p>
          {error && <p className="mb-2 text-sm text-brand-red">{error}</p>}
          <div className="flex gap-3">
            <input
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="Quiz name…"
              autoFocus
              className="flex-1 rounded-lg border border-gray-200 px-3 py-2 text-sm text-slate-700 focus:outline-none focus:ring-2 focus:ring-brand-blue/20"
            />
            <button
              type="submit"
              disabled={isPending || !name.trim()}
              className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-semibold text-white hover:bg-brand-blue/90 disabled:opacity-40 transition-colors"
            >
              {isPending ? 'Creating…' : 'Create'}
            </button>
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="rounded-lg border border-gray-200 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {quizzes.length === 0 ? (
        <div className="rounded-xl border border-dashed border-gray-300 bg-white py-16 text-center">
          <p className="text-slate-400">No quizzes yet — create your first one above.</p>
        </div>
      ) : (
        <ul className="space-y-3">
          {quizzes.map(q => (
            <QuizCard
              key={q.id}
              quiz={q}
              onDelete={() => setQuizzes(prev => prev.filter(x => x.id !== q.id))}
            />
          ))}
        </ul>
      )}
    </main>
  )
}

function QuizCard({ quiz, onDelete }: { quiz: Quiz; onDelete: () => void }) {
  const router = useRouter()
  const [launching, startLaunch] = useTransition()
  const [deleting, startDelete] = useTransition()
  const [launchError, setLaunchError] = useState('')

  function handleLaunch() {
    setLaunchError('')
    startLaunch(async () => {
      const result = await createGameFromQuizAction(quiz.id)
      if (result?.error) setLaunchError(result.error)
      // redirect happens inside the action on success
    })
  }

  function handleDelete() {
    if (!confirm(`Delete quiz "${quiz.name}"? This cannot be undone.`)) return
    startDelete(async () => {
      const result = await deleteQuizAction(quiz.id)
      if (result.success) {
        onDelete()
      } else {
        alert(result.error ?? 'Failed to delete quiz.')
      }
    })
  }

  return (
    <li className="rounded-xl border border-gray-100 bg-white px-5 py-4 shadow-sm">
      <div className="flex items-center gap-4">
        <div className="flex-1 min-w-0">
          <p className="font-semibold text-slate-800 truncate">{quiz.name}</p>
          {quiz.description && (
            <p className="mt-0.5 text-xs text-slate-500 truncate">{quiz.description}</p>
          )}
        </div>
        <button
          onClick={handleLaunch}
          disabled={launching}
          className="rounded-lg bg-green-600 px-4 py-2 text-sm font-display font-semibold text-white hover:bg-green-700 disabled:opacity-40 transition-colors"
        >
          {launching ? 'Launching…' : '▶ Launch Game'}
        </button>
        <button
          onClick={() => router.push(`/quizzes/${quiz.id}`)}
          className="rounded-lg bg-brand-blue px-4 py-2 text-sm font-semibold text-white hover:bg-brand-blue/90 transition-colors"
        >
          Edit
        </button>
        <button
          onClick={handleDelete}
          disabled={deleting}
          className="rounded-lg border border-gray-200 px-3 py-2 text-sm text-slate-500 hover:border-brand-red/40 hover:text-brand-red disabled:opacity-40 transition-colors"
        >
          {deleting ? '…' : 'Delete'}
        </button>
      </div>
      {launchError && (
        <p className="mt-2 text-xs text-brand-red">{launchError}</p>
      )}
    </li>
  )
}
